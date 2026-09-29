<?php

namespace Tests\Feature;

use Illuminate\Foundation\Testing\LazilyRefreshDatabase;
use Tests\TestCase;

class ProfileControllerTest extends TestCase
{
    use LazilyRefreshDatabase;

    protected function setUp(): void
    {
        parent::setUp();
        config()->set('app.env', 'local');
    }

    public function test_local_preview_can_create_and_read_profile(): void
    {
        $headers = ['X-Max-Init-Data' => 'local-preview'];
        $this->withHeaders($headers)->getJson('/api/v1/me/profile')->assertOk()->assertJsonPath('registered', false);

        $this->withHeaders($headers)->putJson('/api/v1/me/profile', [
            'fullName' => 'Иван Петров',
            'phone' => '+7 (999) 123-45-67',
            'gender' => 'male',
            'age' => 29,
        ])->assertOk()->assertJsonPath('profile.phone', '+79991234567');

        $this->assertDatabaseHas('users', ['full_name' => 'Иван Петров', 'phone' => '+79991234567']);
        $this->withHeaders($headers)->getJson('/api/v1/me/profile')->assertOk()->assertJsonPath('registered', true);
    }

    public function test_profile_rejects_invalid_age_without_saving(): void
    {
        $this->withHeader('X-Max-Init-Data', 'local-preview')->putJson('/api/v1/me/profile', [
            'fullName' => 'Иван Петров',
            'phone' => '+79991234567',
            'gender' => 'male',
            'age' => 121,
        ])->assertBadRequest();

        $this->assertDatabaseCount('users', 0);
    }

    public function test_profile_rejects_request_without_max_launch_data(): void
    {
        $this->getJson('/api/v1/me/profile')->assertUnauthorized();
    }
}
