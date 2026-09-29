<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    /**
     * Run the migrations.
     */
    public function up(): void
    {
        Schema::create('max_identities', function (Blueprint $table) {
            $table->bigInteger('max_id')->primary();
            $table->foreignId('user_id')->unique()->constrained('users')->cascadeOnDelete();
            $table->string('display_name');
            $table->dateTime('updated_at');
        });
        Schema::create('companies', function (Blueprint $table) {
            $table->id();
            $table->string('name');
        });
        Schema::create('company_members', function (Blueprint $table) {
            $table->id();
            $table->foreignId('company_id')->constrained('companies');
            $table->string('email', 191)->unique();
            $table->string('password_hash');
            $table->string('full_name');
            $table->string('role', 12);
            $table->dateTime('created_at')->useCurrent();
        });
        Schema::create('admin_sessions', function (Blueprint $table) {
            $table->char('token_hash', 64)->primary();
            $table->foreignId('member_id')->constrained('company_members')->cascadeOnDelete();
            $table->char('csrf_token', 64);
            $table->dateTime('expires_at')->index();
        });
        Schema::create('tasks', function (Blueprint $table) {
            $table->id();
            $table->foreignId('company_id')->constrained('companies');
            $table->foreignId('created_by')->constrained('company_members');
            $table->string('title');
            $table->string('category');
            $table->text('description');
            $table->unsignedBigInteger('budget');
            $table->dateTime('deadline');
            $table->string('location')->default('');
            $table->string('status', 32)->default('open');
            $table->foreignId('assigned_user_id')->nullable()->constrained('users');
            $table->dateTime('completed_at')->nullable();
            $table->json('field_values');
            $table->json('field_schema');
            $table->double('latitude')->nullable();
            $table->double('longitude')->nullable();
            $table->smallInteger('start_offset_minutes')->default(180);
            $table->dateTime('created_at')->useCurrent();
            $table->index(['company_id', 'created_at']);
            $table->index(['status', 'created_at']);
            $table->index(['assigned_user_id', 'status']);
        });
        Schema::create('applications', function (Blueprint $table) {
            $table->id();
            $table->foreignId('task_id')->constrained('tasks');
            $table->foreignId('user_id')->constrained('users');
            $table->string('status', 16)->default('pending');
            $table->dateTime('created_at')->useCurrent();
            $table->unique(['task_id', 'user_id']);
        });
        Schema::create('task_events', function (Blueprint $table) {
            $table->id();
            $table->foreignId('task_id')->constrained('tasks');
            $table->string('kind', 64);
            $table->foreignId('actor_member_id')->nullable()->constrained('company_members');
            $table->foreignId('actor_user_id')->nullable()->constrained('users');
            $table->foreignId('subject_user_id')->nullable()->constrained('users');
            $table->dateTime('created_at')->useCurrent();
        });
        Schema::create('notification_outbox', function (Blueprint $table) {
            $table->id();
            $table->foreignId('task_id')->constrained('tasks');
            $table->foreignId('user_id')->constrained('users');
            $table->text('text');
            $table->unsignedInteger('attempts')->default(0);
            $table->dateTime('next_attempt_at')->useCurrent();
            $table->dateTime('sent_at')->nullable();
            $table->text('last_error')->nullable();
            $table->dateTime('created_at')->useCurrent();
            $table->index(['sent_at', 'next_attempt_at', 'id']);
        });
        Schema::create('max_webhook_inbox', function (Blueprint $table) {
            $table->id();
            $table->char('event_hash', 64)->unique();
            $table->json('payload');
            $table->dateTime('received_at')->useCurrent();
            $table->dateTime('processed_at')->nullable();
            $table->text('last_error')->nullable();
            $table->index(['processed_at', 'id']);
        });
        Schema::create('task_field_definitions', function (Blueprint $table) {
            $table->id();
            $table->foreignId('company_id')->constrained('companies')->cascadeOnDelete();
            $table->string('category', 80);
            $table->string('key', 60);
            $table->string('label');
            $table->string('type', 16);
            $table->boolean('required')->default(false);
            $table->json('options');
            $table->unique(['company_id', 'category', 'key'], 'task_fields_company_category_key');
        });
        Schema::create('task_ratings', function (Blueprint $table) {
            $table->foreignId('task_id')->primary()->constrained('tasks')->cascadeOnDelete();
            $table->unsignedTinyInteger('score');
            $table->text('comment');
            $table->foreignId('author_member_id')->constrained('company_members');
            $table->timestamps();
        });
        Schema::create('task_attachments', function (Blueprint $table) {
            $table->id();
            $table->foreignId('task_id')->constrained('tasks')->cascadeOnDelete();
            $table->string('filename');
            $table->string('content_type', 120);
            $table->string('object_key', 191)->unique();
            $table->dateTime('created_at')->useCurrent();
        });
    }

    /**
     * Reverse the migrations.
     */
    public function down(): void
    {
        Schema::dropIfExists('task_attachments');
        Schema::dropIfExists('task_ratings');
        Schema::dropIfExists('task_field_definitions');
        Schema::dropIfExists('max_webhook_inbox');
        Schema::dropIfExists('notification_outbox');
        Schema::dropIfExists('task_events');
        Schema::dropIfExists('applications');
        Schema::dropIfExists('tasks');
        Schema::dropIfExists('admin_sessions');
        Schema::dropIfExists('company_members');
        Schema::dropIfExists('companies');
        Schema::dropIfExists('max_identities');
    }
};
