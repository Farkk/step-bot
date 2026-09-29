<?php

namespace Tests\Feature;

use Illuminate\Foundation\Testing\LazilyRefreshDatabase;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Http;
use Tests\TestCase;

class ImmediateMaxDeliveryTest extends TestCase
{
    use LazilyRefreshDatabase;

    public function test_publishing_a_draft_sends_max_notification_without_waiting_for_cron(): void
    {
        config()->set('step.max_bot_token', 'new-bot-token');
        config()->set('step.max_bot_web_app', 'id615421905600_bot');
        config()->set('step.max_api_base_url', 'https://max.example.test');
        Http::fake(['max.example.test/messages*' => Http::response(['message' => []], 200)]);

        $companyId = DB::table('companies')->insertGetId(['name' => 'Компания']);
        $memberId = DB::table('company_members')->insertGetId([
            'company_id' => $companyId, 'email' => 'owner@example.test', 'password_hash' => 'unused',
            'full_name' => 'Владелец', 'role' => 'owner', 'created_at' => now(),
        ]);
        $token = str_repeat('a', 64);
        $csrf = str_repeat('b', 64);
        DB::table('admin_sessions')->insert([
            'token_hash' => hash('sha256', $token), 'member_id' => $memberId,
            'csrf_token' => $csrf, 'expires_at' => now()->addDay(),
        ]);
        $userId = DB::table('users')->insertGetId([
            'full_name' => 'Иван Петров', 'phone' => '+79991234567', 'phone_verified' => false,
            'gender' => 'male', 'age' => 29, 'created_at' => now(), 'updated_at' => now(),
        ]);
        DB::table('max_identities')->insert([
            'max_id' => 12345, 'user_id' => $userId, 'display_name' => 'Иван', 'updated_at' => now(),
        ]);

        $created = $this->withUnencryptedCookie('step_admin', $token)
            ->withCredentials()
            ->withHeader('X-CSRF-Token', $csrf)
            ->postJson('/api/v1/admin/tasks', [
                'title' => 'Уборка помещения', 'category' => 'Уборка',
                'description' => 'Убрать помещение после ремонта', 'budget' => 5000,
                'deadline' => now()->addDay()->toISOString(), 'fields' => new \stdClass,
                'publish' => false,
            ])->assertCreated();
        Http::assertNothingSent();

        $this->withUnencryptedCookie('step_admin', $token)
            ->withCredentials()
            ->withHeader('X-CSRF-Token', $csrf)
            ->postJson('/api/v1/admin/tasks/'.$created->json('id').'/publish', [])
            ->assertOk()->assertJsonPath('status', 'open');

        Http::assertSentCount(1);
        $this->assertDatabaseHas('notification_outbox', [
            'task_id' => $created->json('id'), 'user_id' => $userId, 'attempts' => 1,
        ]);
        $this->assertNotNull(DB::table('notification_outbox')->where('task_id', $created->json('id'))->value('sent_at'));
    }

    public function test_new_bot_webhook_replies_without_waiting_for_cron(): void
    {
        config()->set('step.max_webhook_secret', 'new-secret');
        config()->set('step.max_bot_token', 'new-bot-token');
        config()->set('step.max_api_base_url', 'https://max.example.test');
        Http::fake(['max.example.test/messages*' => Http::response(['message' => []], 200)]);

        $this->withHeader('X-Max-Bot-Api-Secret', 'new-secret')->postJson('/integrations/max/webhook', [
            'update_type' => 'bot_started', 'user' => ['user_id' => 12345],
        ])->assertOk();

        Http::assertSentCount(1);
        $this->assertDatabaseHas('max_bot_outbox', ['max_user_id' => 12345, 'attempts' => 1]);
        $this->assertNotNull(DB::table('max_bot_outbox')->where('max_user_id', 12345)->value('sent_at'));
    }
}
