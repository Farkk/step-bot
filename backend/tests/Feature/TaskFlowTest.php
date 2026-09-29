<?php

namespace Tests\Feature;

use Illuminate\Foundation\Testing\LazilyRefreshDatabase;
use Illuminate\Support\Facades\DB;
use Tests\TestCase;

class TaskFlowTest extends TestCase
{
    use LazilyRefreshDatabase;

    public function test_application_decision_and_order_status_require_valid_transitions(): void
    {
        config()->set('app.env', 'local');
        [$token, $csrf] = $this->createOwnerSession();
        $userId = DB::table('users')->insertGetId(['full_name' => 'Иван Петров', 'phone' => '+79991234567', 'phone_verified' => false, 'gender' => 'male', 'age' => 29, 'created_at' => now(), 'updated_at' => now()]);
        DB::table('max_identities')->insert(['max_id' => -1, 'user_id' => $userId, 'display_name' => 'Иван', 'updated_at' => now()]);
        $companyId = DB::table('company_members')->where('email', 'owner@example.test')->value('company_id');
        $memberId = DB::table('company_members')->where('email', 'owner@example.test')->value('id');
        $taskId = DB::table('tasks')->insertGetId(['company_id' => $companyId, 'created_by' => $memberId, 'title' => 'Уборка', 'category' => 'Уборка', 'description' => 'Полная уборка офиса', 'budget' => 5000, 'deadline' => now()->addDay(), 'location' => 'Омск', 'status' => 'open', 'field_values' => '{}', 'field_schema' => '[]', 'created_at' => now()]);
        $applicationId = DB::table('applications')->insertGetId(['task_id' => $taskId, 'user_id' => $userId, 'status' => 'pending', 'created_at' => now()]);

        $admin = fn () => $this->withUnencryptedCookie('step_admin', $token)->withCredentials()->withHeader('X-CSRF-Token', $csrf);
        $worker = fn () => $this->withHeader('X-Max-Init-Data', 'local-preview');
        $admin()->getJson("/api/v1/admin/tasks/{$taskId}/applications")->assertOk()->assertJsonCount(1)->assertJsonPath('0.name', 'Иван Петров');
        $admin()->postJson("/api/v1/admin/tasks/{$taskId}/applications/{$applicationId}/decision", ['decision' => 'accept'])->assertOk()->assertJsonPath('status', 'accept');
        $admin()->postJson("/api/v1/admin/tasks/{$taskId}/applications/{$applicationId}/decision", ['decision' => 'accept'])->assertConflict();
        $worker()->getJson('/api/v1/worker/orders')->assertOk()->assertJsonPath('0.status', 'assigned');
        $worker()->postJson("/api/v1/worker/orders/{$taskId}/status", ['action' => 'complete'])->assertConflict();
        $worker()->postJson("/api/v1/worker/orders/{$taskId}/status", ['action' => 'start'])->assertOk()->assertJsonPath('status', 'in_progress');
        $admin()->postJson("/api/v1/admin/tasks/{$taskId}/status", ['action' => 'pause'])->assertOk()->assertJsonPath('status', 'paused');
        $admin()->postJson("/api/v1/admin/tasks/{$taskId}/status", ['action' => 'resume'])->assertOk()->assertJsonPath('status', 'in_progress');
        $worker()->postJson("/api/v1/worker/orders/{$taskId}/status", ['action' => 'complete'])->assertOk()->assertJsonPath('status', 'awaiting_confirmation');
        $admin()->postJson("/api/v1/admin/tasks/{$taskId}/status", ['action' => 'confirm', 'score' => 5, 'comment' => 'Отлично'])->assertOk()->assertJsonPath('status', 'completed');
        $this->assertDatabaseHas('task_ratings', ['task_id' => $taskId, 'score' => 5]);
        $admin()->getJson('/api/v1/admin/analytics')->assertOk()->assertJsonPath('completed', 1);
        $worker()->getJson('/api/v1/worker/notifications')->assertOk()->assertJsonCount(7);
    }

    public function test_draft_is_invisible_until_published_and_worker_can_apply_once(): void
    {
        config()->set('app.env', 'local');
        [$token, $csrf] = $this->createOwnerSession();
        $userId = DB::table('users')->insertGetId(['full_name' => 'Иван Петров', 'phone' => '+79991234567', 'phone_verified' => false, 'gender' => 'male', 'age' => 29, 'created_at' => now(), 'updated_at' => now()]);
        DB::table('max_identities')->insert(['max_id' => -1, 'user_id' => $userId, 'display_name' => 'Иван', 'updated_at' => now()]);

        $created = $this->withUnencryptedCookie('step_admin', $token)->withCredentials()->withHeader('X-CSRF-Token', $csrf)
            ->postJson('/api/v1/admin/tasks', [
                'title' => 'Уборка помещения',
                'category' => 'Уборка',
                'description' => 'Убрать помещение после ремонта',
                'budget' => 5000,
                'deadline' => now()->addDay()->toISOString(),
                'location' => 'Омск',
                'fields' => new \stdClass,
                'publish' => false,
            ])->assertCreated();
        $taskId = $created->json('id');
        $this->assertDatabaseHas('tasks', ['id' => $taskId, 'status' => 'draft']);

        $this->withHeader('X-Max-Init-Data', 'local-preview')->getJson('/api/v1/worker/tasks')->assertOk()->assertJsonCount(0);
        $this->withUnencryptedCookie('step_admin', $token)->withCredentials()->withHeader('X-CSRF-Token', $csrf)
            ->postJson("/api/v1/admin/tasks/{$taskId}/publish", [])->assertOk()->assertJsonPath('status', 'open');
        $this->withHeader('X-Max-Init-Data', 'local-preview')->getJson('/api/v1/worker/tasks')->assertOk()->assertJsonCount(1);

        $this->withHeader('X-Max-Init-Data', 'local-preview')->postJson("/api/v1/worker/tasks/{$taskId}/applications", [])->assertCreated();
        $this->withHeader('X-Max-Init-Data', 'local-preview')->postJson("/api/v1/worker/tasks/{$taskId}/applications", [])->assertConflict();
        $this->assertDatabaseHas('applications', ['task_id' => $taskId, 'user_id' => $userId, 'status' => 'pending']);
    }

    /** @return array{string, string} */
    private function createOwnerSession(): array
    {
        $companyId = DB::table('companies')->insertGetId(['name' => 'Компания']);
        $memberId = DB::table('company_members')->insertGetId(['company_id' => $companyId, 'email' => 'owner@example.test', 'password_hash' => 'unused', 'full_name' => 'Владелец', 'role' => 'owner', 'created_at' => now()]);
        $token = str_repeat('a', 64);
        $csrf = str_repeat('b', 64);
        DB::table('admin_sessions')->insert(['token_hash' => hash('sha256', $token), 'member_id' => $memberId, 'csrf_token' => $csrf, 'expires_at' => now()->addDay()]);

        return [$token, $csrf];
    }
}
