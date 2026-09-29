<?php

namespace Tests\Feature;

use App\MaxSender;
use Illuminate\Support\Facades\Http;
use Tests\TestCase;

class MaxSenderTest extends TestCase
{
    public function test_task_message_open_app_button_identifies_bot(): void
    {
        config()->set('step.max_bot_token', 'token');
        config()->set('step.max_api_base_url', 'https://max.example.test');
        config()->set('step.max_bot_web_app', 'step_bot');
        Http::fake(['max.example.test/*' => Http::response([], 200)]);

        app(MaxSender::class)->send(12345, 'Новая заявка', 42, false, true);

        Http::assertSent(function ($request): bool {
            $buttons = $request['attachments'][0]['payload']['buttons'];

            return $buttons[0][0]['type'] === 'callback'
                && $buttons[1][0] === ['type' => 'open_app', 'text' => 'Открыть заказ', 'web_app' => 'step_bot', 'payload' => 'task_42'];
        });
    }

    public function test_task_message_without_web_app_keeps_callback_and_omits_open_app_button(): void
    {
        config()->set('step.max_bot_token', 'token');
        config()->set('step.max_api_base_url', 'https://max.example.test');
        config()->set('step.max_bot_web_app', '');
        Http::fake(['max.example.test/*' => Http::response([], 200)]);

        app(MaxSender::class)->send(12345, 'Новая заявка', 42, false, true);

        Http::assertSent(fn ($request): bool => count($request['attachments'][0]['payload']['buttons']) === 1
            && $request['attachments'][0]['payload']['buttons'][0][0]['type'] === 'callback');
    }
}
