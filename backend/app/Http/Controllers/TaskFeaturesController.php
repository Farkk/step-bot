<?php

namespace App\Http\Controllers;

use App\AdminAccess;
use App\MaxLaunchVerifier;
use App\TaskFields;
use Carbon\CarbonImmutable;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;

class TaskFeaturesController extends Controller
{
    public function fields(Request $request, AdminAccess $access, TaskFields $fields): JsonResponse
    {
        $member = $access->requireMember($request);

        return response()->json($fields->definitions($member['companyId'], (string) $request->query('category', '')));
    }

    public function saveField(Request $request, AdminAccess $access): JsonResponse
    {
        $member = $access->requireMember($request, true);
        if ($member['role'] !== 'owner') {
            return response()->json(['error' => 'Только владелец настраивает поля'], 403);
        }
        $category = trim((string) $request->input('category', ''));
        $key = (string) $request->input('key', '');
        $label = trim((string) $request->input('label', ''));
        $type = $request->input('type');
        $required = $request->input('required');
        $options = $request->input('options', []);
        if (strlen($request->getContent()) > 8192 || mb_strlen($category) < 2 || mb_strlen($category) > 80 || mb_strlen($key) < 1 || mb_strlen($key) > 60 || mb_strlen($label) < 1 || mb_strlen($label) > 120 || ! in_array($type, ['text', 'number', 'select', 'date', 'boolean'], true) || ! is_bool($required) || ! is_array($options) || count($options) > 50 || ($type === 'select' && $options === []) || count(array_filter($options, static fn ($value) => ! is_string($value) || mb_strlen($value) > 200)) > 0) {
            return response()->json(['error' => 'Проверьте описание поля'], 400);
        }
        DB::table('task_field_definitions')->upsert([[
            'company_id' => $member['companyId'], 'category' => $category, 'key' => $key,
            'label' => $label, 'type' => $type, 'required' => $required, 'options' => json_encode($options, JSON_UNESCAPED_UNICODE),
        ]], ['company_id', 'category', 'key'], ['label', 'type', 'required', 'options']);

        return response()->json(compact('category', 'key', 'label', 'type', 'required', 'options'));
    }

    public function events(Request $request, AdminAccess $access, string $id): JsonResponse
    {
        $member = $access->requireMember($request);
        $taskId = $this->ownedTask($id, $member['companyId']);
        if ($taskId instanceof JsonResponse) {
            return $taskId;
        }
        $events = DB::table('task_events as e')
            ->leftJoin('company_members as cm', 'cm.id', '=', 'e.actor_member_id')
            ->leftJoin('users as u', 'u.id', '=', 'e.actor_user_id')
            ->leftJoin('max_identities as mi', 'mi.user_id', '=', 'u.id')
            ->leftJoin('users as su', 'su.id', '=', 'e.subject_user_id')
            ->leftJoin('max_identities as sm', 'sm.user_id', '=', 'su.id')
            ->where('e.task_id', $taskId)->orderBy('e.created_at')->orderBy('e.id')
            ->get(['e.kind', 'cm.full_name as member_name', 'u.full_name as user_name', 'mi.display_name as max_name', 'su.full_name as subject_name', 'sm.display_name as subject_max_name', 'e.created_at']);

        return response()->json($events->map(static fn (object $e): array => [
            'kind' => $e->kind,
            'actor' => $e->member_name ?: ($e->max_name ?: ($e->user_name ?: 'Система')),
            'subject' => $e->subject_max_name ?: ($e->subject_name ?: ''),
            'createdAt' => CarbonImmutable::parse($e->created_at, 'UTC')->toISOString(),
        ]));
    }

    public function getRating(Request $request, AdminAccess $access, string $id): JsonResponse
    {
        $member = $access->requireMember($request);
        $taskId = $this->ownedTask($id, $member['companyId']);
        if ($taskId instanceof JsonResponse) {
            return $taskId;
        }

        return response()->json($this->ratingState($taskId));
    }

    public function putRating(Request $request, AdminAccess $access, string $id): JsonResponse
    {
        $member = $access->requireMember($request, true);
        $taskId = $this->ownedTask($id, $member['companyId']);
        if ($taskId instanceof JsonResponse) {
            return $taskId;
        }
        $score = $request->input('score');
        $comment = trim((string) $request->input('comment', ''));
        if (strlen($request->getContent()) > 4096 || ! is_int($score) || $score < 1 || $score > 5 || mb_strlen($comment) < 1 || mb_strlen($comment) > 2000) {
            return response()->json(['error' => 'Укажите 1–5 звёзд и комментарий'], 400);
        }

        return DB::transaction(function () use ($taskId, $member, $score, $comment): JsonResponse {
            $task = DB::table('tasks')->where('id', $taskId)->where('company_id', $member['companyId'])->lockForUpdate()->first();
            if ($task->status !== 'completed' || $task->completed_at === null || CarbonImmutable::parse($task->completed_at, 'UTC')->addDays(14)->isPast()) {
                return response()->json(['error' => 'Срок оценки истёк'], 409);
            }
            $exists = DB::table('task_ratings')->where('task_id', $taskId)->exists();
            DB::table('task_ratings')->upsert([[
                'task_id' => $taskId, 'score' => $score, 'comment' => $comment,
                'author_member_id' => $member['id'], 'created_at' => now(), 'updated_at' => now(),
            ]], ['task_id'], ['score', 'comment', 'author_member_id', 'updated_at']);
            DB::table('task_events')->insert(['task_id' => $taskId, 'kind' => $exists ? 'rating_updated' : 'rating_created', 'actor_member_id' => $member['id'], 'created_at' => now()]);

            return response()->json($this->ratingState($taskId));
        });
    }

    public function reputation(Request $request, MaxLaunchVerifier $verifier): JsonResponse
    {
        $maxUser = $verifier->authenticate($request);
        if ($maxUser === null) {
            return response()->json(['error' => 'Недействительные данные запуска MAX'], 401);
        }
        $userId = DB::table('max_identities')->where('max_id', $maxUser['id'])->value('user_id');
        if ($userId === null) {
            return response()->json(['error' => 'Сначала заполните профиль'], 403);
        }
        $accepted = DB::table('applications')->where('user_id', $userId)->where('status', 'accepted')->count();
        $reviewed = DB::table('applications')->where('user_id', $userId)->whereIn('status', ['accepted', 'rejected'])->count();
        $rating = DB::table('task_ratings as r')->join('tasks as t', 't.id', '=', 'r.task_id')->where('t.assigned_user_id', $userId)->selectRaw('count(*) as total, avg(r.score) as average')->first();

        return response()->json([
            'acceptedPercent' => $reviewed > 0 ? $accepted * 100 / $reviewed : 0,
            'averageRating' => (float) ($rating->average ?? 0), 'ratings' => (int) ($rating->total ?? 0),
            'completed' => DB::table('tasks')->where('assigned_user_id', $userId)->where('status', 'completed')->count(),
        ]);
    }

    public function executors(Request $request, AdminAccess $access): JsonResponse
    {
        $access->requireMember($request);
        $rows = DB::table('users as u')->join('max_identities as m', 'm.user_id', '=', 'u.id')
            ->where('m.max_id', '>', 0)->orderByDesc('u.created_at')->get(['u.*']);

        return response()->json($rows->map(static function (object $user): array {
            $rating = DB::table('task_ratings as r')->join('tasks as t', 't.id', '=', 'r.task_id')->where('t.assigned_user_id', $user->id)->selectRaw('count(*) as total, avg(r.score) as average')->first();

            return [
                'id' => $user->id, 'fullName' => $user->full_name, 'phone' => $user->phone,
                'phoneVerified' => (bool) $user->phone_verified, 'gender' => $user->gender, 'age' => $user->age,
                'rating' => (float) ($rating->average ?? 0), 'ratings' => (int) ($rating->total ?? 0),
                'completed' => DB::table('tasks')->where('assigned_user_id', $user->id)->where('status', 'completed')->count(),
                'registeredAt' => CarbonImmutable::parse($user->created_at, 'UTC')->toISOString(),
            ];
        })->sort(static fn (array $a, array $b): int => [$b['ratings'] > 0, $b['rating'], $b['ratings'], $b['completed'], $b['registeredAt'], $b['id']] <=> [$a['ratings'] > 0, $a['rating'], $a['ratings'], $a['completed'], $a['registeredAt'], $a['id']])->values());
    }

    public function allApplications(Request $request, AdminAccess $access): JsonResponse
    {
        $member = $access->requireMember($request);
        $rows = DB::table('applications as a')->join('tasks as t', 't.id', '=', 'a.task_id')->join('users as u', 'u.id', '=', 'a.user_id')
            ->where('t.company_id', $member['companyId'])->orderByDesc('a.created_at')
            ->get(['a.id', 'a.user_id', 'a.status', 'a.created_at', 't.id as task_id', 't.title as task_title', 't.status as task_status', 'u.full_name']);

        return response()->json($rows->map(static function (object $row): array {
            $rating = DB::table('task_ratings as r')->join('tasks as t', 't.id', '=', 'r.task_id')->where('t.assigned_user_id', $row->user_id)->selectRaw('count(*) as total, avg(r.score) as average')->first();

            return [
                'id' => $row->id, 'taskId' => $row->task_id, 'taskTitle' => $row->task_title,
                'taskStatus' => $row->task_status, 'name' => $row->full_name, 'status' => $row->status,
                'createdAt' => CarbonImmutable::parse($row->created_at, 'UTC')->toISOString(),
                'rating' => (float) ($rating->average ?? 0), 'ratings' => (int) ($rating->total ?? 0),
                'completed' => DB::table('tasks')->where('assigned_user_id', $row->user_id)->where('status', 'completed')->count(),
            ];
        }));
    }

    private function ratingState(int $taskId): array
    {
        $task = DB::table('tasks')->where('id', $taskId)->first(['completed_at']);
        $rating = DB::table('task_ratings')->where('task_id', $taskId)->first();
        $completed = $task->completed_at === null ? null : CarbonImmutable::parse($task->completed_at, 'UTC');
        $result = [
            'score' => (int) ($rating->score ?? 0), 'comment' => (string) ($rating->comment ?? ''),
            'completedAt' => $completed?->toISOString() ?? '0001-01-01T00:00:00Z',
            'editableUntil' => $completed?->addDays(14)->toISOString() ?? '0001-01-01T00:00:00Z',
        ];
        if ($rating !== null) {
            $result['updatedAt'] = CarbonImmutable::parse($rating->updated_at, 'UTC')->toISOString();
        }

        return $result;
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
