<?php

namespace Tests\Feature;

use Illuminate\Foundation\Testing\LazilyRefreshDatabase;
use Illuminate\Support\Facades\DB;
use Tests\TestCase;

class AdminMemberControllerTest extends TestCase
{
    use LazilyRefreshDatabase;

    public function test_owner_sees_only_their_company_members(): void
    {
        [$token] = $this->createOwnerSession();
        $otherCompany = DB::table('companies')->insertGetId(['name' => 'Другая компания']);
        DB::table('company_members')->insert(['company_id' => $otherCompany, 'email' => 'other@example.test', 'password_hash' => 'unused', 'full_name' => 'Другой', 'role' => 'owner', 'created_at' => now()]);

        $this->withUnencryptedCookie('step_admin', $token)->withCredentials()->getJson('/api/v1/admin/members')
            ->assertOk()->assertJsonCount(1)->assertJsonPath('0.email', 'owner@example.test');
    }

    public function test_owner_can_add_manager_with_csrf_token(): void
    {
        [$token, $csrf, $companyId] = $this->createOwnerSession();

        $this->withUnencryptedCookie('step_admin', $token)->withCredentials()->withHeader('X-CSRF-Token', $csrf)
            ->postJson('/api/v1/admin/members', ['name' => 'Новый менеджер', 'email' => 'manager@example.test', 'role' => 'manager', 'password' => 'long-password-456'])
            ->assertCreated()->assertJsonPath('role', 'manager');

        $this->assertDatabaseHas('company_members', ['company_id' => $companyId, 'email' => 'manager@example.test']);
    }

    public function test_missing_csrf_token_cannot_add_member(): void
    {
        [$token] = $this->createOwnerSession();

        $this->withUnencryptedCookie('step_admin', $token)->withCredentials()
            ->postJson('/api/v1/admin/members', ['name' => 'Новый менеджер', 'email' => 'manager@example.test', 'role' => 'manager', 'password' => 'long-password-456'])
            ->assertForbidden();

        $this->assertDatabaseCount('company_members', 1);
    }

    public function test_public_https_origin_is_allowed_behind_reverse_proxy(): void
    {
        config(['app.url' => 'https://step-bot.madebypavel.space']);
        [$token, $csrf] = $this->createOwnerSession();

        $this->withUnencryptedCookie('step_admin', $token)->withCredentials()
            ->withHeaders(['Origin' => 'https://step-bot.madebypavel.space', 'X-CSRF-Token' => $csrf])
            ->postJson('/api/v1/admin/members', ['name' => 'Новый менеджер', 'email' => 'manager@example.test', 'role' => 'manager', 'password' => 'long-password-456'])
            ->assertCreated();
    }

    public function test_other_origin_is_rejected_even_with_valid_csrf_token(): void
    {
        config(['app.url' => 'https://step-bot.madebypavel.space']);
        [$token, $csrf] = $this->createOwnerSession();

        $this->withUnencryptedCookie('step_admin', $token)->withCredentials()
            ->withHeaders(['Origin' => 'https://other.example.test', 'X-CSRF-Token' => $csrf])
            ->postJson('/api/v1/admin/members', ['name' => 'Новый менеджер', 'email' => 'manager@example.test', 'role' => 'manager', 'password' => 'long-password-456'])
            ->assertForbidden();

        $this->assertDatabaseCount('company_members', 1);
    }

    /** @return array{string, string, int} */
    private function createOwnerSession(): array
    {
        $companyId = DB::table('companies')->insertGetId(['name' => 'Компания']);
        $memberId = DB::table('company_members')->insertGetId(['company_id' => $companyId, 'email' => 'owner@example.test', 'password_hash' => 'unused', 'full_name' => 'Владелец', 'role' => 'owner', 'created_at' => now()]);
        $token = str_repeat('a', 64);
        $csrf = str_repeat('b', 64);
        DB::table('admin_sessions')->insert(['token_hash' => hash('sha256', $token), 'member_id' => $memberId, 'csrf_token' => $csrf, 'expires_at' => now()->addDay()]);

        return [$token, $csrf, $companyId];
    }
}
