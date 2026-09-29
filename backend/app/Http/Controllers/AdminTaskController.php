<?php

namespace App\Http\Controllers;

use App\AdminAccess;
use App\TaskFields;
use Carbon\CarbonImmutable;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;
use Throwable;

class AdminTaskController extends Controller
{
    public function index(Request $request, AdminAccess $access): JsonResponse
    {
        $member = $access->requireMember($request);
        $tasks = DB::table('tasks as t')
            ->join('companies as c', 'c.id', '=', 't.company_id')
            ->where('t.company_id', $member['companyId'])
            ->select('t.*', 'c.name as company')
            ->selectSub(DB::table('applications as a')->selectRaw('count(*)')->whereColumn('a.task_id', 't.id'), 'applications')
            ->orderByDesc('t.created_at')
            ->limit(200)
            ->get();

        return response()->json($tasks->map(static fn (object $task): array => [
            'id' => $task->id,
            'company' => $task->company,
            'title' => $task->title,
            'category' => $task->category,
            'description' => $task->description,
            'budget' => $task->budget,
            'deadline' => CarbonImmutable::parse($task->deadline, 'UTC')->toISOString(),
            'location' => $task->location,
            'status' => $task->status,
            'applications' => (int) $task->applications,
            'fields' => json_decode($task->field_values, true) ?: (object) [],
            'fieldSchema' => json_decode($task->field_schema, true) ?: [],
            'latitude' => $task->latitude,
            'longitude' => $task->longitude,
        ]));
    }

    public function store(Request $request, AdminAccess $access, TaskFields $taskFields): JsonResponse
    {
        $member = $access->requireMember($request, true);
        $title = trim((string) $request->input('title', ''));
        $category = trim((string) $request->input('category', ''));
        $description = trim((string) $request->input('description', ''));
        $location = trim((string) $request->input('location', ''));
        $budget = $request->input('budget');
        $offset = $request->input('startOffsetMinutes', 180);
        $latitude = $request->input('latitude');
        $longitude = $request->input('longitude');
        try {
            $deadline = CarbonImmutable::parse((string) $request->input('deadline', ''));
        } catch (Throwable) {
            return response()->json(['error' => 'Проверьте поля заявки'], 400);
        }
        if (strlen($request->getContent()) > 65536 || mb_strlen($title) < 3 || mb_strlen($title) > 160 || mb_strlen($category) < 2 || mb_strlen($category) > 80 || mb_strlen($description) < 10 || mb_strlen($description) > 10000 || ! is_int($budget) || $budget < 1 || ! $deadline->isFuture() || mb_strlen($location) > 300 || ! is_int($offset) || $offset < -840 || $offset > 840 || ($latitude === null) !== ($longitude === null) || ($latitude !== null && (! is_numeric($latitude) || ! is_numeric($longitude) || $latitude < -90 || $latitude > 90 || $longitude < -180 || $longitude > 180))) {
            return response()->json(['error' => 'Проверьте поля заявки'], 400);
        }
        $fields = $request->input('fields', []);
        if (! is_array($fields)) {
            return response()->json(['error' => 'Проверьте поля заявки'], 400);
        }
        $definitions = $taskFields->definitions($member['companyId'], $category);
        if (($error = $taskFields->validate($definitions, $fields)) !== null) {
            return response()->json(['error' => $error], 400);
        }
        $status = $request->input('publish') === false ? 'draft' : 'open';

        $id = DB::transaction(function () use ($member, $title, $category, $description, $budget, $deadline, $location, $fields, $definitions, $latitude, $longitude, $status, $offset): int {
            $id = DB::table('tasks')->insertGetId([
                'company_id' => $member['companyId'],
                'created_by' => $member['id'],
                'title' => $title,
                'category' => $category,
                'description' => $description,
                'budget' => $budget,
                'deadline' => $deadline->utc()->format('Y-m-d H:i:s'),
                'location' => $location,
                'field_values' => json_encode($fields === [] ? (object) [] : $fields, JSON_UNESCAPED_UNICODE),
                'field_schema' => json_encode($definitions, JSON_UNESCAPED_UNICODE),
                'latitude' => $latitude,
                'longitude' => $longitude,
                'status' => $status,
                'start_offset_minutes' => $offset,
                'created_at' => now(),
            ]);
            DB::table('task_events')->insert(['task_id' => $id, 'kind' => $status === 'draft' ? 'draft_created' : 'published', 'actor_member_id' => $member['id'], 'created_at' => now()]);
            if ($status === 'open') {
                $this->notifyWorkers($id, $title);
            }

            return $id;
        });

        return response()->json(['id' => $id], 201);
    }

    public function publish(Request $request, AdminAccess $access, string $id): JsonResponse
    {
        $member = $access->requireMember($request, true);
        if (! ctype_digit($id) || (int) $id < 1) {
            return response()->json(['error' => 'Некорректный номер'], 400);
        }

        return DB::transaction(function () use ($id, $member): JsonResponse {
            $task = DB::table('tasks')->where('id', (int) $id)->where('company_id', $member['companyId'])->lockForUpdate()->first();
            if ($task === null) {
                return response()->json(['error' => 'Заявка не найдена'], 404);
            }
            if ($task->status !== 'draft' || ! CarbonImmutable::parse($task->deadline, 'UTC')->isFuture()) {
                return response()->json(['error' => 'Заявка уже опубликована или срок истёк'], 409);
            }
            DB::table('tasks')->where('id', (int) $id)->update(['status' => 'open']);
            DB::table('task_events')->insert(['task_id' => (int) $id, 'kind' => 'published', 'actor_member_id' => $member['id'], 'created_at' => now()]);
            $this->notifyWorkers((int) $id, $task->title);

            return response()->json(['status' => 'open']);
        });
    }

    private function notifyWorkers(int $taskId, string $title): void
    {
        $rows = DB::table('max_identities')->where('max_id', '>', 0)->get(['user_id'])
            ->map(static fn (object $identity): array => [
                'task_id' => $taskId,
                'user_id' => $identity->user_id,
                'text' => 'Новая заявка «'.$title.'». Откройте ШАГ в MAX, чтобы увидеть детали.',
                'next_attempt_at' => now(),
                'created_at' => now(),
            ])->all();
        if ($rows !== []) {
            DB::table('notification_outbox')->insert($rows);
        }
    }
}
