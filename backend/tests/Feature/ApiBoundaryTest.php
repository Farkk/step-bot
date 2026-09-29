<?php

namespace Tests\Feature;

use Illuminate\Foundation\Testing\LazilyRefreshDatabase;
use Tests\TestCase;

class ApiBoundaryTest extends TestCase
{
    use LazilyRefreshDatabase;

    public function test_health_routes_report_live_process_and_ready_database(): void
    {
        $this->get('/health/live')->assertOk();
        $this->get('/health/ready')->assertOk();
    }

    public function test_react_clients_are_served_at_the_existing_paths(): void
    {
        $this->get('/app/')->assertOk()->assertSee('assets/index-', false);
        $this->get('/admin/')->assertOk()->assertSee('assets/index-', false);
    }

    public function test_private_routes_reject_unauthenticated_requests_with_existing_status_codes(): void
    {
        $this->get('/api/v1/auth/session')->assertUnauthorized();
        $this->get('/api/v1/worker/tasks')->assertUnauthorized();
    }

    public function test_webhook_rejects_missing_secret(): void
    {
        $this->postJson('/integrations/max/webhook', ['update_type' => 'bot_started'])->assertUnauthorized();
    }

    public function test_webhook_stores_exactly_one_copy_of_a_valid_event(): void
    {
        config()->set('step.max_webhook_secret', 'known-secret');
        $headers = ['X-Max-Bot-Api-Secret' => 'known-secret'];
        $event = ['update_type' => 'bot_started', 'user' => ['user_id' => 123]];

        $this->withHeaders($headers)->postJson('/integrations/max/webhook', $event)->assertOk();
        $this->withHeaders($headers)->postJson('/integrations/max/webhook', $event)->assertOk();

        $this->assertDatabaseCount('max_webhook_inbox', 1);
    }
}
