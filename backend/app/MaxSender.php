<?php

namespace App;

use Illuminate\Http\Client\PendingRequest;
use Illuminate\Support\Facades\Http;
use RuntimeException;

class MaxSender
{
    public function send(int $maxUserId, string $text, ?int $taskId = null, bool $hasPhoto = false, bool $allowApply = false): void
    {
        $payload = ['text' => $text, 'notify' => true];
        if ($taskId !== null) {
            $buttons = [];
            if ($allowApply) {
                $buttons[] = [['type' => 'callback', 'text' => 'Откликнуться', 'payload' => 'apply:'.$taskId]];
                if ($hasPhoto) {
                    $buttons[] = [['type' => 'callback', 'text' => 'Фото', 'payload' => 'photo:'.$taskId]];
                }
            }
            $webApp = trim((string) config('step.max_bot_web_app'));
            if ($webApp !== '') {
                $buttons[] = [['type' => 'open_app', 'text' => 'Открыть заказ', 'web_app' => $webApp, 'payload' => 'task_'.$taskId]];
            }
            if ($buttons !== []) {
                $payload['attachments'] = [['type' => 'inline_keyboard', 'payload' => ['buttons' => $buttons]]];
            }
        }
        $response = $this->request()->post($this->baseUrl().'/messages?user_id='.$maxUserId, $payload);
        if (! $response->successful()) {
            $detail = $response->json('message');
            if (! is_string($detail)) {
                $detail = $response->json('error');
            }
            throw new RuntimeException('MAX messages HTTP '.$response->status().(is_string($detail) && $detail !== '' ? ': '.mb_substr($detail, 0, 300) : ''));
        }
    }

    public function answer(string $callbackId, string $message): void
    {
        $response = $this->request()->post($this->baseUrl().'/answers?callback_id='.rawurlencode($callbackId), [
            'message' => ['text' => $message, 'attachments' => []],
        ]);
        if (! $response->successful() || $response->json('success') === false) {
            throw new RuntimeException('MAX callback HTTP '.$response->status());
        }
    }

    public function sendImage(int $maxUserId, string $filename, string $data): void
    {
        $upload = $this->request()->post($this->baseUrl().'/uploads?type=image');
        if (! $upload->successful()) {
            throw new RuntimeException('MAX uploads HTTP '.$upload->status());
        }
        $url = $upload->json('url');
        if (! is_string($url) || parse_url($url, PHP_URL_SCHEME) !== 'https' || parse_url($url, PHP_URL_HOST) !== 'iu.oneme.ru') {
            throw new RuntimeException('Unexpected MAX upload URL');
        }
        $uploaded = $this->request()->timeout(20)->attach('data', $data, $filename)->post($url);
        if (! $uploaded->successful()) {
            throw new RuntimeException('MAX image upload HTTP '.$uploaded->status());
        }
        $photos = $uploaded->json('photos');
        $photo = is_array($photos) ? reset($photos) : null;
        $photoToken = is_array($photo) ? ($photo['token'] ?? null) : null;
        if (! is_string($photoToken) || $photoToken === '') {
            throw new RuntimeException('MAX image token absent');
        }
        for ($attempt = 0; $attempt < 3; $attempt++) {
            $response = $this->request()->post($this->baseUrl().'/messages?user_id='.$maxUserId, [
                'text' => $filename, 'attachments' => [['type' => 'image', 'payload' => ['token' => $photoToken]]],
            ]);
            if ($response->successful()) {
                return;
            }
            if (! str_contains($response->body(), 'attachment.not.ready') || $attempt === 2) {
                throw new RuntimeException('MAX image message HTTP '.$response->status());
            }
            sleep($attempt + 1);
        }
    }

    private function request(): PendingRequest
    {
        $request = Http::withHeaders(['Authorization' => $this->token()])->timeout(10)->acceptJson();
        $ca = resource_path('certs/max-ca-bundle.pem');

        return is_file($ca) ? $request->withOptions(['verify' => $ca]) : $request;
    }

    private function token(): string
    {
        $token = (string) config('step.max_bot_token');
        if ($token === '') {
            throw new RuntimeException('MAX_BOT_TOKEN is empty');
        }

        return $token;
    }

    private function baseUrl(): string
    {
        return rtrim((string) config('step.max_api_base_url', 'https://platform-api2.max.ru'), '/');
    }
}
