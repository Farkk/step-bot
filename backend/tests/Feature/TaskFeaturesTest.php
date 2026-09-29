<?php

namespace Tests\Feature;

use Illuminate\Foundation\Testing\LazilyRefreshDatabase;
use Illuminate\Support\Facades\DB;
use Tests\TestCase;

class TaskFeaturesTest extends TestCase
{
    use LazilyRefreshDatabase;

    public function test_owner_can_define_field_and_rating_is_tenant_scoped(): void
    {
        $companyId = DB::table('companies')->insertGetId(['name' => 'Компания']);
        $otherCompanyId = DB::table('companies')->insertGetId(['name' => 'Чужая']);
        $memberId = DB::table('company_members')->insertGetId(['company_id' => $companyId, 'email' => 'owner@example.test', 'password_hash' => 'unused', 'full_name' => 'Владелец', 'role' => 'owner', 'created_at' => now()]);
        $otherMemberId = DB::table('company_members')->insertGetId(['company_id' => $otherCompanyId, 'email' => 'other@example.test', 'password_hash' => 'unused', 'full_name' => 'Другой', 'role' => 'owner', 'created_at' => now()]);
        $token = str_repeat('c', 64);
        $csrf = str_repeat('d', 64);
        DB::table('admin_sessions')->insert(['token_hash' => hash('sha256', $token), 'member_id' => $memberId, 'csrf_token' => $csrf, 'expires_at' => now()->addDay()]);
        $this->withUnencryptedCookie('step_admin', $token)->withCredentials()->withHeader('X-CSRF-Token', $csrf)
            ->postJson('/api/v1/admin/fields', ['category' => 'Уборка', 'key' => 'rooms', 'label' => 'Комнаты', 'type' => 'number', 'required' => true, 'options' => []])->assertOk();
        $this->getJson('/api/v1/admin/fields?category='.rawurlencode('Уборка'))->assertOk()->assertJsonPath('0.key', 'rooms');
        $taskId = DB::table('tasks')->insertGetId(['company_id' => $otherCompanyId, 'created_by' => $otherMemberId, 'title' => 'Чужая задача', 'category' => 'Уборка', 'description' => 'Описание чужой заявки', 'budget' => 100, 'deadline' => now()->addDay(), 'location' => '', 'status' => 'completed', 'field_values' => '{}', 'field_schema' => '[]', 'completed_at' => now(), 'created_at' => now()]);
        $this->getJson("/api/v1/admin/tasks/{$taskId}/rating")->assertNotFound();
        $this->putJson("/api/v1/admin/tasks/{$taskId}/rating", ['score' => 5, 'comment' => 'Хорошо'])->assertNotFound();
    }
}
