<?php

namespace Tests\Unit;

use App\MaxLaunchVerifier;
use PHPUnit\Framework\TestCase;

class MaxLaunchVerifierTest extends TestCase
{
    public function test_valid_signed_launch_and_tampering(): void
    {
        $token = 'test-bot-token';
        $now = 1790668800;
        $user = json_encode(['id' => 12345, 'first_name' => 'Иван', 'last_name' => 'Петров'], JSON_UNESCAPED_UNICODE);
        $params = ['auth_date' => (string) $now, 'user' => $user];
        $secret = hash_hmac('sha256', $token, 'WebAppData', true);
        $hash = hash_hmac('sha256', "auth_date={$now}\nuser={$user}", $secret);
        $raw = http_build_query($params + ['hash' => strtoupper($hash)]);
        $verifier = new MaxLaunchVerifier;

        $this->assertSame(12345, $verifier->verify($raw, $token, $now)['id']);
        $this->assertNull($verifier->verify(str_replace('12345', '12346', urldecode($raw)), $token, $now));
        $this->assertNull($verifier->verify($raw, $token, $now + 3601));
        $this->assertNull($verifier->verify($raw.'&user=duplicate', $token, $now));
    }

    public function test_contact_proof_uses_max_user_and_phone(): void
    {
        $now = 1790668800;
        $hash = strtoupper(hash_hmac('sha256', "authDate={$now}\nphone=79991234567\nuserId=12345", 'token'));
        $verifier = new MaxLaunchVerifier;

        $this->assertTrue($verifier->verifyContact('+79991234567', (string) $now, $hash, 12345, 'token', $now));
        $this->assertFalse($verifier->verifyContact('+79991234568', (string) $now, $hash, 12345, 'token', $now));
    }
}
