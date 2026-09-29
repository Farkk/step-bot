<?php

namespace App\Http\Controllers;

use App\AdminAccess;
use App\MaxLaunchVerifier;
use App\TaskWorkflow;
use Carbon\CarbonImmutable;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;

class TaskWorkflowController extends Controller
{
    public function applications(Request $request, AdminAccess $access, string $id): JsonResponse
    {
        $member = $access->requireMember($request);
        $taskId = $this->ownedTask($id, $member['companyId']);
        if ($taskId instanceof JsonResponse) {
            return $taskId;
        }
        $rows = DB::table('applications as a')->join('users as u', 'u.id', '=', 'a.user_id')
            ->where('a.task_id', $taskId)->orderBy('a.created_at')
            ->get(['a.id', 'a.user_id', 'u.full_name', 'a.status', 'a.created_at']);

        return response()->json($rows->map(static function (object $row): array {
            $ratings = DB::table('task_ratings as r')->join('tasks as t', 't.id', '=', 'r.task_id')
                ->where('t.assigned_user_id', $row->user_id)->selectRaw('count(*) as total, avg(r.score) as average')->first();

            return [
                'id' => $row->id, 'userId' => $row->user_id, 'name' => $row->full_name,
                'status' => $row->status, 'createdAt' => CarbonImmutable::parse($row->created_at, 'UTC')->toISOString(),
                'rating' => (float) ($ratings->average ?? 0), 'ratings' => (int) ($ratings->total ?? 0),
                'completed' => DB::table('tasks')->where('assigned_user_id', $row->user_id)->where('status', 'completed')->count(),
            ];
        }));
    }

    public function decide(Request $request, AdminAccess $access, string $id, string $application): JsonResponse
    {
        $member = $access->requireMember($request, true);
        if (strlen($request->getContent()) > 1024 || ! in_array($request->input('decision'), ['accept', 'reject'], true) || ! ctype_digit($id) || ! ctype_digit($application) || (int) $id < 1 || (int) $application < 1) {
            return response()->json(['error' => 'Нужно выбрать решение'], 400);
        }

        return DB::transaction(function () use ($member, $id, $application, $request): JsonResponse {
            $task = DB::table('tasks')->where('id', (int) $id)->where('company_id', $member['companyId'])->lockForUpdate()->first();
            if ($task === null) {
                return response()->json(['error' => 'Заявка не найдена'], 404);
            }
            if ($task->status !== 'open') {
                return response()->json(['error' => 'Заявка уже назначена или закрыта'], 409);
            }
            $chosen = DB::table('applications')->where('id', (int) $application)->where('task_id', $task->id)->where('status', 'pending')->lockForUpdate()->first();
            if ($chosen === null) {
                return response()->json(['error' => 'Отклик уже обработан'], 409);
            }
            $accept = $request->input('decision') === 'accept';
            if ($accept) {
                $others = DB::table('applications')->where('task_id', $task->id)->where('id', '<>', $chosen->id)->where('status', 'pending')->pluck('user_id');
                DB::table('tasks')->where('id', $task->id)->update(['status' => 'assigned', 'assigned_user_id' => $chosen->user_id]);
                DB::table('applications')->where('task_id', $task->id)->where('status', 'pending')->update(['status' => 'rejected']);
                DB::table('applications')->where('id', $chosen->id)->update(['status' => 'accepted']);
                foreach ($others as $otherUserId) {
                    DB::table('task_events')->insert(['task_id' => $task->id, 'kind' => 'application_rejected_auto', 'actor_member_id' => $member['id'], 'subject_user_id' => $otherUserId, 'created_at' => now()]);
                    TaskWorkflow::notify($task->id, $otherUserId, 'rejected', $task->title);
                }
            } else {
                DB::table('applications')->where('id', $chosen->id)->update(['status' => 'rejected']);
            }
            DB::table('task_events')->insert(['task_id' => $task->id, 'kind' => $accept ? 'application_accepted' : 'application_rejected', 'actor_member_id' => $member['id'], 'subject_user_id' => $chosen->user_id, 'created_at' => now()]);
            TaskWorkflow::notify($task->id, $chosen->user_id, $accept ? 'assigned' : 'rejected', $task->title);

            return response()->json(['status' => $request->input('decision')]);
        });
    }

    public function orders(Request $request, MaxLaunchVerifier $verifier): JsonResponse
    {
        $userId = $this->workerId($request, $verifier);
        if ($userId instanceof JsonResponse) {
            return $userId;
        }
        $rows = DB::table('tasks as t')->join('companies as c', 'c.id', '=', 't.company_id')
            ->where('t.assigned_user_id', $userId)->whereIn('t.status', ['assigned', 'in_progress', 'paused', 'awaiting_confirmation', 'completed', 'cancelled'])
            ->orderByDesc('t.created_at')->limit(200)->get(['t.*', 'c.name as company']);

        return response()->json($rows->map(static function (object $task): array {
            $result = [
                'id' => $task->id, 'company' => $task->company, 'title' => $task->title, 'category' => $task->category,
                'description' => $task->description, 'budget' => $task->budget,
                'deadline' => CarbonImmutable::parse($task->deadline, 'UTC')->toISOString(), 'location' => $task->location,
                'status' => $task->status, 'applications' => DB::table('applications')->where('task_id', $task->id)->count(),
                'fields' => json_decode($task->field_values, true) ?: (object) [], 'fieldSchema' => json_decode($task->field_schema, true) ?: [],
            ];
            if ($task->latitude !== null && $task->longitude !== null) {
                $result['latitude'] = (float) $task->latitude;
                $result['longitude'] = (float) $task->longitude;
            }

            return $result;
        }));
    }

    public function orderStatus(Request $request, MaxLaunchVerifier $verifier, string $id): JsonResponse
    {
        $userId = $this->workerId($request, $verifier);
        if ($userId instanceof JsonResponse) {
            return $userId;
        }
        $action = $request->input('action');
        if (! ctype_digit($id) || (int) $id < 1 || strlen($request->getContent()) > 1024 || ! in_array($action, ['start', 'complete'], true)) {
            return response()->json(['error' => 'Некорректное действие'], 400);
        }

        return DB::transaction(function () use ($id, $userId, $action): JsonResponse {
            $task = DB::table('tasks')->where('id', (int) $id)->where('assigned_user_id', $userId)->lockForUpdate()->first();
            if ($task === null) {
                return response()->json(['error' => 'Заказ не найден'], 404);
            }
            $next = TaskWorkflow::nextStatus($task->status, $action);
            if ($next === null) {
                return response()->json(['error' => 'Действие недоступно'], 409);
            }
            DB::table('tasks')->where('id', $task->id)->update(['status' => $next]);
            DB::table('task_events')->insert(['task_id' => $task->id, 'kind' => $next, 'actor_user_id' => $userId, 'created_at' => now()]);
            TaskWorkflow::notify($task->id, $userId, $next, $task->title);

            return response()->json(['status' => $next]);
        });
    }

    public function adminStatus(Request $request, AdminAccess $access, string $id): JsonResponse
    {
        $member = $access->requireMember($request, true);
        $action = $request->input('action');
        $reason = trim((string) $request->input('reason', ''));
        $comment = trim((string) $request->input('comment', ''));
        $score = $request->input('score');
        if (! ctype_digit($id) || (int) $id < 1 || strlen($request->getContent()) > 8192 || ! in_array($action, ['pause', 'resume', 'confirm', 'cancel'], true)) {
            return response()->json(['error' => 'Некорректное действие'], 400);
        }
        if ($action === 'confirm' && (! is_int($score) || $score < 1 || $score > 5 || mb_strlen($comment) < 1 || mb_strlen($comment) > 2000)) {
            return response()->json(['error' => 'Укажите 1–5 звёзд и комментарий перед завершением'], 400);
        }
        if ($action === 'cancel' && (mb_strlen($reason) < 3 || mb_strlen($reason) > 500)) {
            return response()->json(['error' => 'Укажите причину отмены'], 400);
        }

        return DB::transaction(function () use ($id, $member, $action, $reason, $score, $comment): JsonResponse {
            $task = DB::table('tasks')->where('id', (int) $id)->where('company_id', $member['companyId'])->lockForUpdate()->first();
            if ($task === null) {
                return response()->json(['error' => 'Заявка не найдена'], 404);
            }
            $next = TaskWorkflow::nextStatus($task->status, $action);
            if ($next === null) {
                return response()->json(['error' => 'Переход статуса недоступен'], 409);
            }
            DB::table('tasks')->where('id', $task->id)->update(['status' => $next, 'completed_at' => $next === 'completed' ? now() : $task->completed_at]);
            DB::table('task_events')->insert(['task_id' => $task->id, 'kind' => $next.($reason !== '' ? ': '.$reason : ''), 'actor_member_id' => $member['id'], 'created_at' => now()]);
            if ($action === 'confirm') {
                DB::table('task_ratings')->insert(['task_id' => $task->id, 'score' => $score, 'comment' => $comment, 'author_member_id' => $member['id'], 'created_at' => now(), 'updated_at' => now()]);
                DB::table('task_events')->insert(['task_id' => $task->id, 'kind' => 'rating_created', 'actor_member_id' => $member['id'], 'created_at' => now()]);
            }
            if ($task->assigned_user_id !== null) {
                TaskWorkflow::notify($task->id, $task->assigned_user_id, $next, $task->title);
            } elseif ($next === 'cancelled') {
                $userIds = DB::table('applications')->where('task_id', $task->id)->where('status', 'pending')->pluck('user_id');
                foreach ($userIds as $userId) {
                    TaskWorkflow::notify($task->id, $userId, $next, $task->title);
                }
            }

            return response()->json(['status' => $next]);
        });
    }

    public function notifications(Request $request, MaxLaunchVerifier $verifier): JsonResponse
    {
        $userId = $this->workerId($request, $verifier);
        if ($userId instanceof JsonResponse) {
            return $userId;
        }
        $kinds = ['application_accepted', 'application_rejected', 'application_rejected_auto', 'in_progress', 'paused', 'awaiting_confirmation', 'completed', 'cancelled', 'rating_created', 'rating_updated'];
        $rows = DB::table('task_events as e')->join('tasks as t', 't.id', '=', 'e.task_id')
            ->where(function ($query) use ($userId, $kinds): void {
                $query->where('e.kind', 'published')
                    ->orWhere(fn ($q) => $q->where('e.subject_user_id', $userId)->whereIn('e.kind', array_slice($kinds, 0, 3)))
                    ->orWhere(fn ($q) => $q->where('t.assigned_user_id', $userId)->whereIn('e.kind', array_slice($kinds, 3)));
            })->orderByDesc('e.id')->limit(50)->get(['e.id', 't.id as task_id', 't.title', 'e.kind', 'e.created_at']);

        return response()->json($rows->map(static fn (object $row): array => [
            'id' => $row->id, 'taskId' => $row->task_id, 'title' => $row->title,
            'kind' => $row->kind, 'createdAt' => CarbonImmutable::parse($row->created_at, 'UTC')->toISOString(),
        ]));
    }

    private function workerId(Request $request, MaxLaunchVerifier $verifier): int|JsonResponse
    {
        $user = $verifier->authenticate($request);
        if ($user === null) {
            return response()->json(['error' => 'Недействительные данные запуска MAX'], 401);
        }
        $id = DB::table('max_identities')->where('max_id', $user['id'])->value('user_id');

        return $id === null ? response()->json(['error' => 'Сначала заполните профиль'], 403) : (int) $id;
    }

    private function ownedTask(string $id, int $companyId): int|JsonResponse
    {
        if (! ctype_digit($id) || (int) $id < 1) {
            return response()->json(['error' => 'Некорректный номер'], 400);
        }

        return DB::table('tasks')->where('id', (int) $id)->where('company_id', $companyId)->exists()
            ? (int) $id : response()->json(['error' => 'Заявка не найдена'], 404);
    }
}
