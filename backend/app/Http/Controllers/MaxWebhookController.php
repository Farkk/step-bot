<?php

namespace App\Http\Controllers;

use App\MaxBotProcessor;
use Illuminate\Http\Request;
use Illuminate\Http\Response;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Log;
use JsonException;
use Throwable;

class MaxWebhookController extends Controller
{
    public function __invoke(Request $request, MaxBotProcessor $processor): Response
    {
        $secret = (string) config('step.max_webhook_secret');
        $supplied = (string) $request->header('X-Max-Bot-Api-Secret', '');

        if ($secret === '' || ! hash_equals($secret, $supplied)) {
            return response('unauthorized', 401);
        }

        $payload = $request->getContent();

        if (strlen($payload) > 1024 * 1024) {
            return response('payload too large', 413);
        }

        try {
            $event = json_decode($payload, true, 512, JSON_THROW_ON_ERROR);
        } catch (JsonException) {
            return response('invalid JSON', 400);
        }

        try {
            DB::table('max_webhook_inbox')->insertOrIgnore([
                'event_hash' => hash('sha256', $payload),
                'payload' => $payload,
                'received_at' => now(),
            ]);
        } catch (Throwable) {
            return response('storage unavailable', 503);
        }

        $callbackId = is_array($event) && ($event['update_type'] ?? '') === 'message_callback'
            ? ($event['callback']['callback_id'] ?? null) : null;
        if (is_string($callbackId) && $callbackId !== '') {
            try {
                $processor->handleCallbackNow(hash('sha256', $payload), $callbackId);
            } catch (Throwable $error) {
                Log::warning('MAX callback deferred to cron', ['error' => $error->getMessage()]);
            }
        }

        return response('', 200);
    }
}
