<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('max_bot_outbox', function (Blueprint $table) {
            $table->id();
            $table->string('kind', 20);
            $table->bigInteger('max_user_id')->nullable();
            $table->string('callback_id')->nullable();
            $table->text('text');
            $table->foreignId('task_id')->nullable()->constrained('tasks');
            $table->unsignedInteger('attempts')->default(0);
            $table->dateTime('next_attempt_at')->useCurrent();
            $table->dateTime('sent_at')->nullable();
            $table->text('last_error')->nullable();
            $table->dateTime('created_at')->useCurrent();
            $table->index(['sent_at', 'next_attempt_at', 'id']);
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('max_bot_outbox');
    }
};
