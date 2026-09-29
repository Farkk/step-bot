<?php

namespace App\Console\Commands;

use App\MaxBotProcessor;
use Illuminate\Console\Attributes\Description;
use Illuminate\Console\Attributes\Signature;
use Illuminate\Console\Command;

#[Signature('max:tick {--limit=30} {--seconds=40}')]
#[Description('Process MAX webhook events and outbound messages')]
class MaxTick extends Command
{
    public function handle(MaxBotProcessor $processor): int
    {
        $limit = (int) $this->option('limit');
        if ($limit < 1 || $limit > 100) {
            $this->error('Limit must be between 1 and 100');

            return self::FAILURE;
        }
        $seconds = (int) $this->option('seconds');
        if ($seconds < 1 || $seconds > 40) {
            $this->error('Seconds must be between 1 and 40');

            return self::FAILURE;
        }
        $lock = fopen(storage_path('app/max-tick.lock'), 'c');
        if ($lock === false || ! flock($lock, LOCK_EX | LOCK_NB)) {
            $this->warn('MAX tick is already running');

            return self::SUCCESS;
        }
        try {
            $this->info(json_encode($processor->tick($limit, $seconds)));
        } finally {
            flock($lock, LOCK_UN);
            fclose($lock);
        }

        return self::SUCCESS;
    }
}
