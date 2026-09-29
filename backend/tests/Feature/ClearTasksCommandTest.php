<?php

namespace Tests\Feature;

use Illuminate\Foundation\Testing\LazilyRefreshDatabase;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Storage;
use Tests\TestCase;

class ClearTasksCommandTest extends TestCase
{
    use LazilyRefreshDatabase;

    public function test_preview_preserves_data_and_force_removes_tasks_with_related_data_and_files(): void
    {
        Storage::fake('local');
        $companyId = DB::table('companies')->insertGetId(['name' => 'Компания']);
        $memberId = DB::table('company_members')->insertGetId([
            'company_id' => $companyId, 'email' => 'owner@example.test', 'password_hash' => 'unused',
            'full_name' => 'Владелец', 'role' => 'owner', 'created_at' => now(),
        ]);
        $userId = DB::table('users')->insertGetId([
            'full_name' => 'Иван Петров', 'phone' => '+79991234567', 'phone_verified' => true,
            'gender' => 'male', 'age' => 29, 'created_at' => now(), 'updated_at' => now(),
        ]);
        DB::table('max_identities')->insert(['max_id' => 12345, 'user_id' => $userId, 'display_name' => 'Иван', 'updated_at' => now()]);
        $taskId = DB::table('tasks')->insertGetId([
            'company_id' => $companyId, 'created_by' => $memberId, 'title' => 'Тестовая заявка',
            'category' => 'Уборка', 'description' => 'Описание задачи', 'budget' => 5000,
            'deadline' => now()->addDay(), 'location' => 'Омск', 'status' => 'open',
            'field_values' => '{}', 'field_schema' => '[]', 'created_at' => now(),
        ]);
        DB::table('applications')->insert(['task_id' => $taskId, 'user_id' => $userId, 'created_at' => now()]);
        DB::table('task_events')->insert(['task_id' => $taskId, 'kind' => 'published', 'actor_member_id' => $memberId, 'created_at' => now()]);
        DB::table('notification_outbox')->insert(['task_id' => $taskId, 'user_id' => $userId, 'text' => 'Новая заявка', 'created_at' => now()]);
        DB::table('max_bot_outbox')->insert(['kind' => 'photo', 'max_user_id' => 12345, 'task_id' => $taskId, 'text' => 'Фото', 'created_at' => now()]);
        DB::table('task_ratings')->insert(['task_id' => $taskId, 'score' => 5, 'comment' => 'Отлично', 'author_member_id' => $memberId, 'created_at' => now(), 'updated_at' => now()]);
        Storage::disk('local')->put('attachments/test-photo', 'image');
        DB::table('task_attachments')->insert(['task_id' => $taskId, 'filename' => 'test.jpg', 'content_type' => 'image/jpeg', 'object_key' => 'attachments/test-photo', 'created_at' => now()]);

        $this->artisan('tasks:clear')->assertSuccessful();
        $this->assertDatabaseCount('tasks', 1);
        Storage::disk('local')->assertExists('attachments/test-photo');

        $this->artisan('tasks:clear --force')->assertSuccessful();
        foreach (['tasks', 'applications', 'task_events', 'notification_outbox', 'max_bot_outbox', 'task_ratings', 'task_attachments'] as $table) {
            $this->assertDatabaseCount($table, 0);
        }
        Storage::disk('local')->assertMissing('attachments/test-photo');
        $this->assertDatabaseCount('users', 1);
        $this->assertDatabaseCount('max_identities', 1);
        $this->assertDatabaseCount('companies', 1);
        $this->assertDatabaseCount('company_members', 1);
    }
}
