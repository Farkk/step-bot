<?php

namespace Tests\Feature;

use Illuminate\Foundation\Testing\LazilyRefreshDatabase;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\Storage;
use Tests\TestCase;

class MaxBotTest extends TestCase
{
    use LazilyRefreshDatabase;

    public function test_photo_callback_is_acknowledged_before_image_delivery(): void
    {
        Storage::fake('local');
        config()->set('step.max_webhook_secret', 'secret');
        config()->set('step.max_bot_token', 'token');
        config()->set('step.max_api_base_url', 'https://max.example.test');
        Http::fake([
            'max.example.test/answers*' => Http::response(['success' => true], 200),
            'max.example.test/uploads*' => Http::response(['url' => 'https://iu.oneme.ru/upload'], 200),
            'iu.oneme.ru/*' => Http::response(['photos' => ['p1' => ['token' => 'image-token']]], 200),
            'max.example.test/messages*' => Http::response([], 200),
        ]);
        $companyId = DB::table('companies')->insertGetId(['name' => 'Компания']);
        $memberId = DB::table('company_members')->insertGetId(['company_id' => $companyId, 'email' => 'owner@example.test', 'password_hash' => 'unused', 'full_name' => 'Владелец', 'role' => 'owner', 'created_at' => now()]);
        $userId = DB::table('users')->insertGetId(['full_name' => 'Иван', 'phone' => '+79991234567', 'phone_verified' => true, 'gender' => 'male', 'age' => 30, 'created_at' => now(), 'updated_at' => now()]);
        DB::table('max_identities')->insert(['max_id' => 12345, 'user_id' => $userId, 'display_name' => 'Иван', 'updated_at' => now()]);
        $taskId = DB::table('tasks')->insertGetId(['company_id' => $companyId, 'created_by' => $memberId, 'title' => 'Заявка', 'category' => 'Работа', 'description' => 'Описание задачи', 'budget' => 100, 'deadline' => now()->addDay(), 'location' => 'Омск', 'status' => 'open', 'field_values' => '{}', 'field_schema' => '[]', 'created_at' => now()]);
        Storage::disk('local')->put('attachments/photo', 'fake-image');
        DB::table('task_attachments')->insert(['task_id' => $taskId, 'filename' => 'photo.jpg', 'content_type' => 'image/jpeg', 'object_key' => 'attachments/photo', 'created_at' => now()]);

        $this->withHeader('X-Max-Bot-Api-Secret', 'secret')->postJson('/integrations/max/webhook', [
            'update_type' => 'message_callback', 'callback' => ['callback_id' => 'photo-callback', 'payload' => 'photo:'.$taskId, 'user' => ['user_id' => 12345]],
        ])->assertOk();
        Http::assertSentCount(4);
        $this->assertDatabaseHas('max_bot_outbox', ['kind' => 'photo', 'callback_id' => null, 'attempts' => 1]);
        $this->assertNotNull(DB::table('max_bot_outbox')->where('kind', 'photo')->value('sent_at'));
        $this->artisan('max:tick')->assertSuccessful();
        $this->assertDatabaseHas('max_bot_outbox', ['kind' => 'photo', 'attempts' => 1]);
        Http::assertSentCount(4);
    }

    public function test_failed_max_delivery_stays_in_outbox_for_retry(): void
    {
        config()->set('step.max_webhook_secret', 'secret');
        config()->set('step.max_bot_token', 'token');
        config()->set('step.max_api_base_url', 'https://max.example.test');
        Http::fake(['max.example.test/*' => Http::response([], 500)]);

        $this->withHeader('X-Max-Bot-Api-Secret', 'secret')->postJson('/integrations/max/webhook', [
            'update_type' => 'bot_started', 'user' => ['user_id' => 12345],
        ])->assertOk();
        $this->artisan('max:tick')->assertSuccessful();
        $this->assertDatabaseHas('max_bot_outbox', ['max_user_id' => 12345, 'attempts' => 1, 'sent_at' => null]);
        $this->artisan('max:tick')->assertSuccessful();
        Http::assertSentCount(1);
    }

    public function test_duplicate_webhook_is_processed_once_and_callback_queues_application(): void
    {
        config()->set('step.max_webhook_secret', 'secret');
        config()->set('step.max_bot_token', 'token');
        config()->set('step.max_api_base_url', 'https://max.example.test');
        Http::fake(['max.example.test/*' => Http::response([], 200)]);
        $companyId = DB::table('companies')->insertGetId(['name' => 'Компания']);
        $memberId = DB::table('company_members')->insertGetId(['company_id' => $companyId, 'email' => 'owner@example.test', 'password_hash' => 'unused', 'full_name' => 'Владелец', 'role' => 'owner', 'created_at' => now()]);
        $userId = DB::table('users')->insertGetId(['full_name' => 'Иван', 'phone' => '+79991234567', 'phone_verified' => true, 'gender' => 'male', 'age' => 30, 'created_at' => now(), 'updated_at' => now()]);
        DB::table('max_identities')->insert(['max_id' => 12345, 'user_id' => $userId, 'display_name' => 'Иван', 'updated_at' => now()]);
        $taskId = DB::table('tasks')->insertGetId(['company_id' => $companyId, 'created_by' => $memberId, 'title' => 'Заявка', 'category' => 'Работа', 'description' => 'Описание задачи', 'budget' => 100, 'deadline' => now()->addDay(), 'location' => 'Омск', 'status' => 'open', 'field_values' => '{}', 'field_schema' => '[]', 'created_at' => now()]);
        $event = ['update_type' => 'message_callback', 'callback' => ['callback_id' => 'c1', 'payload' => 'apply:'.$taskId, 'user' => ['user_id' => 12345]]];
        $this->withHeader('X-Max-Bot-Api-Secret', 'secret')->postJson('/integrations/max/webhook', $event)->assertOk();
        $this->assertDatabaseCount('applications', 1);
        Http::assertSentCount(2);
        $this->withHeader('X-Max-Bot-Api-Secret', 'secret')->postJson('/integrations/max/webhook', $event)->assertOk();
        $this->artisan('max:tick')->assertSuccessful();
        $this->assertDatabaseCount('max_webhook_inbox', 1);
        $this->assertDatabaseCount('applications', 1);
        $this->assertDatabaseHas('max_bot_outbox', ['callback_id' => 'c1', 'text' => 'Отклик на заявку отправлен']);
        Http::assertSentCount(2);
    }
}
