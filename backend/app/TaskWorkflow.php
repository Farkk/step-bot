<?php

namespace App;

use Illuminate\Support\Facades\DB;

class TaskWorkflow
{
    public static function nextStatus(string $current, string $action): ?string
    {
        return match ($current.':'.$action) {
            'assigned:start', 'paused:resume' => 'in_progress',
            'in_progress:pause' => 'paused',
            'in_progress:complete' => 'awaiting_confirmation',
            'awaiting_confirmation:confirm' => 'completed',
            'open:cancel', 'assigned:cancel', 'in_progress:cancel', 'paused:cancel', 'awaiting_confirmation:cancel' => 'cancelled',
            default => null,
        };
    }

    public static function notify(int $taskId, int $userId, string $status, string $title): void
    {
        $hasMax = DB::table('max_identities')->where('user_id', $userId)->where('max_id', '>', 0)->exists();
        if (! $hasMax) {
            return;
        }
        $label = match ($status) {
            'assigned' => 'Назначен исполнитель',
            'in_progress' => 'На исполнении',
            'paused' => 'Приостановлена',
            'awaiting_confirmation' => 'Ожидает подтверждения',
            'completed' => 'Завершена',
            'cancelled' => 'Отменена',
            'rejected' => 'Отклик отклонён',
            default => $status,
        };
        DB::table('notification_outbox')->insert([
            'task_id' => $taskId,
            'user_id' => $userId,
            'text' => "Заявка «{$title}»: {$label}. Откройте ШАГ в MAX, чтобы увидеть детали.",
            'next_attempt_at' => now(),
            'created_at' => now(),
        ]);
    }
}
