<?php

namespace App\Console\Commands;

use Illuminate\Console\Attributes\Description;
use Illuminate\Console\Attributes\Signature;
use Illuminate\Console\Command;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Storage;

#[Signature('tasks:clear {--force : Delete all tasks and their related data}')]
#[Description('Preview or delete all tasks while preserving users, companies and members')]
class ClearTasks extends Command
{
    public function handle(): int
    {
        $tables = ['tasks', 'applications', 'task_events', 'notification_outbox', 'task_ratings', 'task_attachments'];
        foreach ($tables as $table) {
            $this->line("{$table}: ".DB::table($table)->count());
        }
        $this->line('max_bot_outbox (task photos): '.DB::table('max_bot_outbox')->whereNotNull('task_id')->count());

        if (! $this->option('force')) {
            $this->info('Preview only. Run tasks:clear --force to delete these records and attachment files.');

            return self::SUCCESS;
        }

        $lock = fopen(storage_path('app/max-tick.lock'), 'c');
        if ($lock === false || ! flock($lock, LOCK_EX | LOCK_NB)) {
            $this->error('MAX processor is running. Retry after it finishes.');

            return self::FAILURE;
        }

        try {
            $keys = DB::transaction(function (): array {
                $keys = DB::table('task_attachments')->pluck('object_key')->all();
                DB::table('max_bot_outbox')->whereNotNull('task_id')->delete();
                foreach (['notification_outbox', 'task_events', 'applications', 'task_ratings', 'task_attachments', 'tasks'] as $table) {
                    DB::table($table)->delete();
                }

                return $keys;
            });
        } finally {
            flock($lock, LOCK_UN);
            fclose($lock);
        }

        $failed = 0;
        foreach ($keys as $key) {
            if (! Storage::disk('local')->delete($key)) {
                $this->warn("Could not remove attachment file: {$key}");
                $failed++;
            }
        }
        if ($failed > 0) {
            $this->error("Task records were deleted, but {$failed} attachment files could not be removed.");

            return self::FAILURE;
        }
        $this->info('All tasks and related data deleted. Users, companies and members were preserved.');

        return self::SUCCESS;
    }
}
