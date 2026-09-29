<?php

namespace App\Http\Controllers;

use App\AdminAccess;
use App\MaxLaunchVerifier;
use Carbon\CarbonImmutable;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Storage;
use Symfony\Component\HttpFoundation\BinaryFileResponse;

class AttachmentController extends Controller
{
    public function upload(Request $request, AdminAccess $access, string $id): JsonResponse
    {
        $member = $access->requireMember($request, true);
        $taskId = $this->ownedTask($id, $member['companyId']);
        if ($taskId instanceof JsonResponse) {
            return $taskId;
        }
        $file = $request->file('file');
        if ($file === null || ! $file->isValid()) {
            return response()->json(['error' => 'Выберите файл'], 400);
        }
        if ($file->getSize() < 1 || $file->getSize() > 5 * 1024 * 1024) {
            return response()->json(['error' => 'Файл должен быть не больше 5 МБ'], 413);
        }
        $name = basename(str_replace('\\', '/', $file->getClientOriginalName()));
        if ($name === '.' || $name === '' || mb_strlen($name) > 200) {
            return response()->json(['error' => 'Некорректное имя файла'], 400);
        }
        $mime = (new \finfo(FILEINFO_MIME_TYPE))->file($file->getRealPath());
        if (! is_string($mime) || (! str_starts_with($mime, 'image/') && ! in_array($mime, ['application/pdf', 'text/plain', 'application/zip'], true))) {
            return response()->json(['error' => 'Допустимы изображения, PDF, текст и ZIP'], 400);
        }
        $key = 'attachments/'.bin2hex(random_bytes(16));
        if (! Storage::disk('local')->put($key, $file->getContent())) {
            return response()->json(['error' => 'Хранилище файлов недоступно'], 503);
        }
        try {
            $attachmentId = DB::transaction(function () use ($taskId, $member, $name, $mime, $key): int {
                $attachmentId = DB::table('task_attachments')->insertGetId([
                    'task_id' => $taskId, 'filename' => $name, 'content_type' => $mime,
                    'object_key' => $key, 'created_at' => now(),
                ]);
                DB::table('task_events')->insert(['task_id' => $taskId, 'kind' => 'attachment_added', 'actor_member_id' => $member['id'], 'created_at' => now()]);

                return $attachmentId;
            });
        } catch (\Throwable $error) {
            Storage::disk('local')->delete($key);
            throw $error;
        }

        return response()->json(['id' => $attachmentId], 201);
    }

    public function list(Request $request, AdminAccess $access, MaxLaunchVerifier $verifier, string $id): JsonResponse
    {
        $taskId = $this->visibleTask($request, $access, $verifier, $id);
        if ($taskId instanceof JsonResponse) {
            return $taskId;
        }

        return response()->json(DB::table('task_attachments')->where('task_id', $taskId)->orderBy('id')->get()
            ->map(static fn (object $item): array => [
                'id' => $item->id, 'filename' => $item->filename, 'contentType' => $item->content_type,
                'createdAt' => CarbonImmutable::parse($item->created_at, 'UTC')->toISOString(),
            ]));
    }

    public function download(Request $request, AdminAccess $access, MaxLaunchVerifier $verifier, string $id): BinaryFileResponse|JsonResponse
    {
        if (! ctype_digit($id) || (int) $id < 1) {
            return response()->json(['error' => 'Некорректный номер'], 400);
        }
        $attachment = DB::table('task_attachments as a')->join('tasks as t', 't.id', '=', 'a.task_id')
            ->where('a.id', (int) $id)->first(['a.*', 't.company_id', 't.assigned_user_id', 't.status']);
        if ($attachment === null) {
            return response()->json(['error' => 'Файл не найден'], 404);
        }
        $member = $access->current($request)['member'] ?? null;
        $allowed = $member !== null && $member['companyId'] === $attachment->company_id;
        if (! $allowed) {
            $maxUser = $verifier->authenticate($request);
            if ($maxUser === null) {
                return response()->json(['error' => 'Недействительные данные запуска MAX'], 401);
            }
            $userId = DB::table('max_identities')->where('max_id', $maxUser['id'])->value('user_id');
            if ($userId === null) {
                return response()->json(['error' => 'Сначала заполните профиль'], 403);
            }
            $allowed = $attachment->status === 'open' || $attachment->assigned_user_id === $userId;
        }
        if (! $allowed) {
            return response()->json(['error' => 'Файл не найден'], 404);
        }
        if (! Storage::disk('local')->exists($attachment->object_key)) {
            return response()->json(['error' => 'Хранилище файлов недоступно'], 503);
        }

        return response()->download(Storage::disk('local')->path($attachment->object_key), $attachment->filename, [
            'Content-Type' => $attachment->content_type, 'Cache-Control' => 'private, no-store', 'X-Content-Type-Options' => 'nosniff',
        ]);
    }

    private function visibleTask(Request $request, AdminAccess $access, MaxLaunchVerifier $verifier, string $id): int|JsonResponse
    {
        if (str_starts_with($request->path(), 'api/v1/admin/')) {
            $member = $access->requireMember($request);

            return $this->ownedTask($id, $member['companyId']);
        }
        $maxUser = $verifier->authenticate($request);
        if ($maxUser === null) {
            return response()->json(['error' => 'Недействительные данные запуска MAX'], 401);
        }
        $userId = DB::table('max_identities')->where('max_id', $maxUser['id'])->value('user_id');
        if ($userId === null) {
            return response()->json(['error' => 'Сначала заполните профиль'], 403);
        }
        if (! ctype_digit($id) || (int) $id < 1) {
            return response()->json(['error' => 'Некорректный номер'], 400);
        }
        $found = DB::table('tasks')->where('id', (int) $id)
            ->where(fn ($q) => $q->where('status', 'open')->orWhere('assigned_user_id', $userId))->exists();

        return $found ? (int) $id : response()->json(['error' => 'Заявка не найдена'], 404);
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
