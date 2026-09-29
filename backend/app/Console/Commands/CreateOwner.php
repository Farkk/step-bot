<?php

namespace App\Console\Commands;

use Illuminate\Console\Attributes\Description;
use Illuminate\Console\Attributes\Signature;
use Illuminate\Console\Command;
use Illuminate\Support\Facades\DB;

#[Signature('owner:create {company} {email} {name}')]
#[Description('Создать первого владельца компании')]
class CreateOwner extends Command
{
    public function handle(): int
    {
        $company = trim((string) $this->argument('company'));
        $email = mb_strtolower(trim((string) $this->argument('email')));
        $name = trim((string) $this->argument('name'));
        $password = (string) getenv('ADMIN_INITIAL_PASSWORD');

        if ($company === '' || $name === '' || ! filter_var($email, FILTER_VALIDATE_EMAIL) || strlen($password) < 12) {
            $this->error('Нужны компания, имя, корректная почта и ADMIN_INITIAL_PASSWORD от 12 символов.');

            return self::FAILURE;
        }

        DB::transaction(static function () use ($company, $email, $name, $password): void {
            $companyId = DB::table('companies')->insertGetId(['name' => $company]);
            DB::table('company_members')->insert([
                'company_id' => $companyId,
                'email' => $email,
                'password_hash' => password_hash($password, PASSWORD_BCRYPT),
                'full_name' => $name,
                'role' => 'owner',
                'created_at' => now(),
            ]);
        });

        $this->info('Владелец создан.');

        return self::SUCCESS;
    }
}
