<?php

namespace App;

use Illuminate\Http\Request;

class MaxLaunchVerifier
{
    /** @return array{id: int, first_name: string, last_name: string}|null */
    public function authenticate(Request $request): ?array
    {
        $raw = (string) $request->header('X-Max-Init-Data', '');

        if ($raw === 'local-preview') {
            return config('app.env') === 'local' && in_array($request->getHost(), ['localhost', '127.0.0.1', '::1'], true)
                ? ['id' => -1, 'first_name' => '', 'last_name' => '']
                : null;
        }

        return $this->verify($raw, (string) config('step.max_bot_token'), time());
    }

    /** @return array{id: int, first_name: string, last_name: string}|null */
    public function verify(string $raw, string $token, int $now): ?array
    {
        if ($raw === '' || $token === '' || strlen($raw) > 16384) {
            return null;
        }

        $params = [];
        foreach (explode('&', $raw) as $pair) {
            $parts = explode('=', $pair, 2);
            $key = urldecode($parts[0]);
            if ($key === '' || count($parts) !== 2 || array_key_exists($key, $params)) {
                return null;
            }
            $params[$key] = urldecode($parts[1]);
        }

        if (! isset($params['hash'], $params['auth_date'], $params['user']) || ! ctype_digit($params['auth_date'])) {
            return null;
        }

        $issued = (int) $params['auth_date'];
        if ($now - $issued > 3600 || $issued > $now + 60) {
            return null;
        }

        $provided = $params['hash'];
        unset($params['hash']);
        ksort($params, SORT_STRING);
        $lines = [];
        foreach ($params as $key => $value) {
            $lines[] = $key.'='.$value;
        }
        $secret = hash_hmac('sha256', $token, 'WebAppData', true);
        $expected = hash_hmac('sha256', implode("\n", $lines), $secret);
        $providedBytes = ctype_xdigit($provided) && strlen($provided) === 64 ? hex2bin($provided) : false;
        if ($providedBytes === false || ! hash_equals(hex2bin($expected), $providedBytes)) {
            return null;
        }

        $user = json_decode($params['user'], true);
        if (! is_array($user) || ! isset($user['id']) || ! is_int($user['id']) || $user['id'] <= 0) {
            return null;
        }

        return [
            'id' => (int) $user['id'],
            'first_name' => (string) ($user['first_name'] ?? ''),
            'last_name' => (string) ($user['last_name'] ?? ''),
        ];
    }

    public function verifyContact(string $phone, string $authDate, string $hash, int $userId, string $token, int $now): bool
    {
        if ($token === '' || $userId <= 0 || ! ctype_digit($authDate)) {
            return false;
        }
        $issued = (int) $authDate;
        if ($now - $issued > 3600 || $issued > $now + 60) {
            return false;
        }
        $message = 'authDate='.$authDate."\nphone=".ltrim($phone, '+')."\nuserId=".$userId;

        $providedBytes = ctype_xdigit($hash) && strlen($hash) === 64 ? hex2bin($hash) : false;

        return $providedBytes !== false && hash_equals(hash_hmac('sha256', $message, $token, true), $providedBytes);
    }
}
