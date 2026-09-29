<?php

namespace Tests\Feature;

use Illuminate\Foundation\Testing\LazilyRefreshDatabase;
use Illuminate\Support\Facades\DB;
use Tests\TestCase;

class AdminAuthControllerTest extends TestCase
{
    use LazilyRefreshDatabase;

    public function test_valid_owner_credentials_create_session_and_return_member(): void
    {
        $companyId = DB::table('companies')->insertGetId(['name' => 'Компания']);
        DB::table('company_members')->insert([
            'company_id' => $companyId,
            'email' => 'owner@example.test',
            'password_hash' => password_hash('long-password-123', PASSWORD_BCRYPT),
            'full_name' => 'Владелец',
            'role' => 'owner',
            'created_at' => now(),
        ]);

        $this->postJson('/api/v1/auth/login', [
            'email' => 'owner@example.test',
            'password' => 'long-password-123',
        ])->assertOk()->assertCookie('step_admin')->assertJsonPath('member.role', 'owner');

        $this->assertDatabaseCount('admin_sessions', 1);
    }

    public function test_bad_password_returns_unauthorized_without_session(): void
    {
        $companyId = DB::table('companies')->insertGetId(['name' => 'Компания']);
        DB::table('company_members')->insert([
            'company_id' => $companyId,
            'email' => 'owner@example.test',
            'password_hash' => password_hash('long-password-123', PASSWORD_BCRYPT),
            'full_name' => 'Владелец',
            'role' => 'owner',
            'created_at' => now(),
        ]);

        $this->postJson('/api/v1/auth/login', [
            'email' => 'owner@example.test',
            'password' => 'wrong-password',
        ])->assertUnauthorized();

        $this->assertDatabaseCount('admin_sessions', 0);
    }
}
