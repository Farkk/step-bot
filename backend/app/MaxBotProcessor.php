<?php

namespace App;

use Carbon\CarbonImmutable;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Storage;
use RuntimeException;
use Throwable;

class MaxBotProcessor
{
    public function __construct(private readonly MaxSender $sender) {}

    public function handleCallbackNow(string $eventHash, string $callbackId): void
    {
        $this->processInbox($eventHash);
        if ((string) config('step.max_bot_token') === '') {
            return;
        }
        $row = DB::table('max_bot_outbox')->where('callback_id', $callbackId)->whereNull('sent_at')->first(['kind']);
        if ($row?->kind === 'photo') {
            DB::transaction(function () use ($callbackId): void {
                $pending = DB::table('max_bot_outbox')->where('callback_id', $callbackId)->whereNull('sent_at')->lockForUpdate()->first();
                if ($pending !== null) {
                    $this->sender->answer($callbackId, 'Отправляю фото');
                    DB::table('max_bot_outbox')->where('id', $pending->id)->update(['callback_id' => null]);
                }
            });
        } else {
            $this->deliverReply($callbackId);
        }
    }

    public function tick(int $limit = 30, int $seconds = 40): array
    {
        $counts = ['inbox' => 0, 'replies' => 0, 'notifications' => 0];
        $deadline = microtime(true) + $seconds;
        foreach (['inbox' => 'processInbox', 'replies' => 'deliverReply', 'notifications' => 'deliverNotification'] as $key => $method) {
            for ($i = 0; $i < $limit; $i++) {
                if (microtime(true) >= $deadline || ! $this->$method()) {
                    break;
                }
                $counts[$key]++;
            }
        }

        return $counts;
    }

    private function processInbox(?string $eventHash = null): bool
    {
        return DB::transaction(function () use ($eventHash): bool {
            $query = DB::table('max_webhook_inbox')->whereNull('processed_at');
            if ($eventHash !== null) {
                $query->where('event_hash', $eventHash);
            }
            $row = $query->orderBy('id')->lockForUpdate()->first();
            if ($row === null) {
                return false;
            }
            $event = json_decode($row->payload, true);
            if (! is_array($event)) {
                DB::table('max_webhook_inbox')->where('id', $row->id)->update(['processed_at' => now(), 'last_error' => 'Invalid JSON']);

                return true;
            }
            $type = $event['update_type'] ?? '';
            $text = '';
            $callback = null;
            $recipient = 0;
            $photo = null;
            if ($type === 'bot_started') {
                $recipient = (int) ($event['user']['user_id'] ?? 0);
                $text = 'Откройте Mini App ШАГ, заполните профиль и отправьте /tasks, чтобы получить доступные заявки.';
            } elseif ($type === 'message_created' && trim((string) ($event['message']['body']['text'] ?? '')) === '/tasks') {
                $recipient = (int) ($event['message']['sender']['user_id'] ?? 0);
                $text = $this->listTasks($recipient);
            } elseif ($type === 'message_callback') {
                $callback = (string) ($event['callback']['callback_id'] ?? '');
                $recipient = (int) ($event['callback']['user']['user_id'] ?? $event['user']['user_id'] ?? 0);
                $payload = (string) ($event['callback']['payload'] ?? '');
                if (str_starts_with($payload, 'apply:')) {
                    $id = substr($payload, 6);
                    $text = ctype_digit($id) && (int) $id > 0 ? $this->apply((int) $id, $recipient) : 'Некорректная заявка';
                } elseif (str_starts_with($payload, 'photo:')) {
                    $id = substr($payload, 6);
                    if (! ctype_digit($id) || (int) $id < 1 || $recipient < 1) {
                        $text = 'Некорректная заявка';
                    } else {
                        $userId = DB::table('max_identities')->where('max_id', $recipient)->value('user_id');
                        $available = $userId !== null && DB::table('task_attachments as a')->join('tasks as t', 't.id', '=', 'a.task_id')
                            ->where('a.task_id', (int) $id)->where('a.content_type', 'like', 'image/%')
                            ->where(fn ($q) => $q->where('t.status', 'open')->orWhere('t.assigned_user_id', $userId))->exists();
                        $text = $available ? 'Фото отправлено' : 'Фото недоступно или заявка закрыта';
                        $photo = $available ? (int) $id : null;
                    }
                }
            }
            if ($text !== '' && ($callback !== null && $callback !== '' || $recipient > 0)) {
                DB::table('max_bot_outbox')->insert([
                    'kind' => $photo !== null ? 'photo' : ($callback !== null && $callback !== '' ? 'callback' : 'message'),
                    'max_user_id' => $recipient, 'callback_id' => $callback, 'task_id' => $photo,
                    'text' => $text, 'next_attempt_at' => now(), 'created_at' => now(),
                ]);
            }
            DB::table('max_webhook_inbox')->where('id', $row->id)->update(['processed_at' => now(), 'last_error' => null]);

            return true;
        });
    }

    private function listTasks(int $maxId): string
    {
        $userId = DB::table('max_identities')->where('max_id', $maxId)->value('user_id');
        if ($userId === null) {
            return 'Сначала заполните профиль в Mini App ШАГ';
        }
        $tasks = DB::table('tasks')->where('status', 'open')->orderByDesc('created_at')->limit(5)->get(['id', 'title']);
        foreach ($tasks as $task) {
            DB::table('notification_outbox')->insert(['task_id' => $task->id, 'user_id' => $userId, 'text' => 'Новая заявка «'.$task->title.'»', 'next_attempt_at' => now(), 'created_at' => now()]);
        }

        return $tasks->isEmpty() ? 'Открытых заявок пока нет' : 'Отправляю '.$tasks->count().' последних заявок';
    }

    private function apply(int $taskId, int $maxId): string
    {
        $userId = $maxId > 0 ? DB::table('max_identities')->where('max_id', $maxId)->value('user_id') : null;
        if ($userId === null) {
            return 'Сначала заполните профиль в Mini App';
        }
        $task = DB::table('tasks')->where('id', $taskId)->lockForUpdate()->first(['status']);
        if ($task === null) {
            return 'Заявка не найдена';
        }
        if ($task->status !== 'open') {
            return 'Заявка больше не открыта';
        }
        if (DB::table('applications')->where('task_id', $taskId)->where('user_id', $userId)->exists()) {
            return 'Вы уже откликнулись';
        }
        DB::table('applications')->insert(['task_id' => $taskId, 'user_id' => $userId, 'status' => 'pending', 'created_at' => now()]);
        DB::table('task_events')->insert(['task_id' => $taskId, 'kind' => 'application_created', 'actor_user_id' => $userId, 'created_at' => now()]);
        DB::table('notification_outbox')->insert(['task_id' => $taskId, 'user_id' => $userId, 'text' => 'Отклик на заявку отправлен', 'next_attempt_at' => now(), 'created_at' => now()]);

        return 'Отклик на заявку отправлен';
    }

    private function deliverReply(?string $callbackId = null): bool
    {
        if ((string) config('step.max_bot_token') === '') {
            return false;
        }

        return DB::transaction(function () use ($callbackId): bool {
            $query = DB::table('max_bot_outbox')->whereNull('sent_at')->where('next_attempt_at', '<=', now());
            if ($callbackId !== null) {
                $query->where('callback_id', $callbackId);
            }
            $row = $query->orderBy('next_attempt_at')->orderBy('id')->lockForUpdate()->first();
            if ($row === null) {
                return false;
            }
            try {
                if ($row->kind === 'photo') {
                    $userId = DB::table('max_identities')->where('max_id', $row->max_user_id)->value('user_id');
                    $attachment = $userId === null ? null : DB::table('task_attachments as a')->join('tasks as t', 't.id', '=', 'a.task_id')
                        ->where('a.task_id', $row->task_id)->where('a.content_type', 'like', 'image/%')
                        ->where(fn ($q) => $q->where('t.status', 'open')->orWhere('t.assigned_user_id', $userId))
                        ->orderBy('a.id')->first(['a.filename', 'a.object_key']);
                    $reply = $row->text;
                    if ($attachment === null) {
                        $reply = 'Фото недоступно или заявка закрыта';
                    } elseif (! Storage::disk('local')->exists($attachment->object_key)) {
                        throw new RuntimeException('Photo absent');
                    } else {
                        $this->sender->sendImage($row->max_user_id, $attachment->filename, Storage::disk('local')->get($attachment->object_key));
                    }
                    if ($row->callback_id !== null) {
                        $this->sender->answer($row->callback_id, $reply);
                    }
                } elseif ($row->kind === 'callback') {
                    $this->sender->answer($row->callback_id, $row->text);
                } else {
                    $this->sender->send($row->max_user_id, $row->text);
                }
                DB::table('max_bot_outbox')->where('id', $row->id)->update(['sent_at' => now(), 'attempts' => $row->attempts + 1, 'last_error' => null]);
            } catch (Throwable $error) {
                $this->retry('max_bot_outbox', $row->id, $row->attempts, $error);
            }

            return true;
        });
    }

    private function deliverNotification(): bool
    {
        if ((string) config('step.max_bot_token') === '') {
            return false;
        }

        return DB::transaction(function (): bool {
            $row = DB::table('notification_outbox as o')->join('max_identities as m', 'm.user_id', '=', 'o.user_id')
                ->whereNull('o.sent_at')->where('o.next_attempt_at', '<=', now())->where('m.max_id', '>', 0)
                ->orderBy('o.next_attempt_at')->orderBy('o.id')->lockForUpdate()->first(['o.*', 'm.max_id']);
            if ($row === null) {
                return false;
            }
            try {
                $isNew = str_starts_with($row->text, 'Новая заявка №') || str_starts_with($row->text, 'Новая заявка «');
                $task = DB::table('tasks')->where('id', $row->task_id)->first(['status']);
                $text = $isNew ? $this->taskMessage($row->task_id) : $row->text;
                $hasPhoto = DB::table('task_attachments')->where('task_id', $row->task_id)->where('content_type', 'like', 'image/%')->exists();
                $this->sender->send($row->max_id, $text, $row->task_id, $hasPhoto, $isNew && $task?->status === 'open');
                DB::table('notification_outbox')->where('id', $row->id)->update(['sent_at' => now(), 'attempts' => $row->attempts + 1, 'last_error' => null]);
            } catch (Throwable $error) {
                $this->retry('notification_outbox', $row->id, $row->attempts, $error);
            }

            return true;
        });
    }

    private function taskMessage(int $taskId): string
    {
        $task = DB::table('tasks')->where('id', $taskId)->first();
        if ($task === null) {
            throw new RuntimeException('Task absent');
        }
        $description = mb_strlen($task->description) > 2500 ? mb_substr($task->description, 0, 2500).'…' : $task->description;
        $deadline = CarbonImmutable::parse($task->deadline, 'UTC')->addMinutes($task->start_offset_minutes)->format('d.m.Y H:i');
        $message = "Новая заявка: {$task->title}\n{$description}\nБюджет: {$task->budget} ₽\nНачало работ: {$deadline}\nМесто: {$task->location}";
        $values = json_decode($task->field_values, true) ?: [];
        foreach (json_decode($task->field_schema, true) ?: [] as $field) {
            if (array_key_exists($field['key'], $values)) {
                $value = $values[$field['key']];
                $line = "\n{$field['label']}: ".(is_bool($value) ? ($value ? 'true' : 'false') : (string) $value);
                if (mb_strlen($message.$line) < 3700) {
                    $message .= $line;
                }
            }
        }
        $count = DB::table('task_attachments')->where('task_id', $taskId)->count();
        if ($count > 0) {
            $message .= "\nВложений: {$count} (откройте Mini App)";
        }
        if ($task->status !== 'open') {
            $message .= "\nЗаявка уже закрыта для откликов.";
        }

        return $message;
    }

    private function retry(string $table, int $id, int $attempts, Throwable $error): void
    {
        DB::table($table)->where('id', $id)->update([
            'attempts' => $attempts + 1, 'next_attempt_at' => now()->addSeconds(2 ** min($attempts, 8)),
            'last_error' => mb_substr($error->getMessage(), 0, 2000),
        ]);
    }
}
