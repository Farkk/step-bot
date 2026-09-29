<?php

namespace App\Http\Controllers;

use App\MaxLaunchVerifier;
use Carbon\CarbonImmutable;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;

class WorkerTaskController extends Controller
{
    public function index(Request $request, MaxLaunchVerifier $verifier): JsonResponse
    {
        $maxUser = $verifier->authenticate($request);
        if ($maxUser === null) {
            return response()->json(['error' => 'Недействительные данные запуска MAX'], 401);
        }

        $userId = DB::table('max_identities')->where('max_id', $maxUser['id'])->value('user_id');
        if ($userId === null) {
            return response()->json(['error' => 'Сначала заполните профиль'], 403);
        }

        $tasks = DB::table('tasks as t')
            ->join('companies as c', 'c.id', '=', 't.company_id')
            ->where('t.status', 'open')
            ->select('t.*', 'c.name as company')
            ->selectSub(DB::table('applications as a')->selectRaw('count(*)')->whereColumn('a.task_id', 't.id'), 'applications')
            ->selectSub(DB::table('applications as a')->select('a.status')->whereColumn('a.task_id', 't.id')->where('a.user_id', $userId)->limit(1), 'my_application')
            ->orderByDesc('t.created_at')
            ->limit(200)
            ->get();

        return response()->json($tasks->map(static function (object $task): array {
            $result = [
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
            ];
            if ($task->my_application !== null) {
                $result['myApplication'] = $task->my_application;
            }
            if ($task->latitude !== null && $task->longitude !== null) {
                $result['latitude'] = (float) $task->latitude;
                $result['longitude'] = (float) $task->longitude;
            }

            return $result;
        }));
    }

    public function apply(Request $request, MaxLaunchVerifier $verifier, string $id): JsonResponse
    {
        $maxUser = $verifier->authenticate($request);
        if ($maxUser === null) {
            return response()->json(['error' => 'Недействительные данные запуска MAX'], 401);
        }
        $userId = DB::table('max_identities')->where('max_id', $maxUser['id'])->value('user_id');
        if ($userId === null) {
            return response()->json(['error' => 'Сначала заполните профиль'], 403);
        }
        if (! ctype_digit($id) || (int) $id < 1) {
            return response()->json(['error' => 'Некорректный номер заявки'], 400);
        }

        return DB::transaction(function () use ($id, $userId): JsonResponse {
            $task = DB::table('tasks')->where('id', (int) $id)->lockForUpdate()->first(['id', 'status']);
            if ($task === null) {
                return response()->json(['error' => 'Заявка не найдена'], 404);
            }
            if ($task->status !== 'open') {
                return response()->json(['error' => 'Заявка больше не открыта'], 409);
            }
            if (DB::table('applications')->where('task_id', $task->id)->where('user_id', $userId)->exists()) {
                return response()->json(['error' => 'Вы уже откликнулись'], 409);
            }
            DB::table('applications')->insert(['task_id' => $task->id, 'user_id' => $userId, 'status' => 'pending', 'created_at' => now()]);
            DB::table('task_events')->insert(['task_id' => $task->id, 'kind' => 'application_created', 'actor_user_id' => $userId, 'created_at' => now()]);

            return response()->json(['status' => 'pending'], 201);
        });
    }
}
