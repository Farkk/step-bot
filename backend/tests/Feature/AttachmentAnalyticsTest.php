<?php

namespace Tests\Feature;

use Illuminate\Foundation\Testing\LazilyRefreshDatabase;
use Illuminate\Http\UploadedFile;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Storage;
use Tests\TestCase;

class AttachmentAnalyticsTest extends TestCase
{
    use LazilyRefreshDatabase;

    public function test_private_attachment_requires_company_or_visible_worker_and_analytics_validates_period(): void
    {
        Storage::fake('local');
        config()->set('app.env', 'local');
        $companyId = DB::table('companies')->insertGetId(['name' => 'Компания']);
        $memberId = DB::table('company_members')->insertGetId(['company_id' => $companyId, 'email' => 'owner@example.test', 'password_hash' => 'unused', 'full_name' => 'Владелец', 'role' => 'owner', 'created_at' => now()]);
        $token = str_repeat('e', 64);
        $csrf = str_repeat('f', 64);
        DB::table('admin_sessions')->insert(['token_hash' => hash('sha256', $token), 'member_id' => $memberId, 'csrf_token' => $csrf, 'expires_at' => now()->addDay()]);
        $taskId = DB::table('tasks')->insertGetId(['company_id' => $companyId, 'created_by' => $memberId, 'title' => 'Заявка', 'category' => 'Работа', 'description' => 'Описание задачи', 'budget' => 100, 'deadline' => now()->addDay(), 'location' => 'Омск', 'status' => 'open', 'field_values' => '{}', 'field_schema' => '[]', 'created_at' => now()]);
        $this->getJson('/api/v1/admin/analytics?from=2026-99-99')->assertUnauthorized();
        $admin = fn () => $this->withUnencryptedCookie('step_admin', $token)->withCredentials()->withHeader('X-CSRF-Token', $csrf);
        $admin()->getJson('/api/v1/admin/analytics?from=2026-99-99')->assertBadRequest();
        $admin()->getJson('/api/v1/admin/analytics')->assertOk()->assertJsonPath('completed', 0);
        $created = $admin()->post("/api/v1/admin/tasks/{$taskId}/attachments", ['file' => UploadedFile::fake()->createWithContent('memo.txt', 'Содержимое')]);
        $created->assertCreated();
        $attachmentId = $created->json('id');
        $this->withUnencryptedCookie('step_admin', 'invalid')->get("/api/v1/attachments/{$attachmentId}")->assertUnauthorized();
        $admin()->get("/api/v1/attachments/{$attachmentId}")->assertOk()->assertHeader('X-Content-Type-Options', 'nosniff');
        $this->withHeader('X-Max-Init-Data', 'local-preview')->getJson("/api/v1/worker/tasks/{$taskId}/attachments")->assertForbidden();
    }
}
