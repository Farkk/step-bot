<?php

namespace App;

use Illuminate\Http\Exceptions\HttpResponseException;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;

class AdminAccess
{
    /** @return array{member: array<string, mixed>, csrfToken: string}|null */
    public function current(Request $request): ?array
    {
        $token = $request->cookie('step_admin');
        if (! is_string($token) || ! preg_match('/^[a-f0-9]{64}$/', $token)) {
            return null;
        }

        $session = DB::table('admin_sessions as s')
            ->join('company_members as m', 'm.id', '=', 's.member_id')
            ->join('companies as c', 'c.id', '=', 'm.company_id')
            ->where('s.token_hash', hash('sha256', $token))
            ->where('s.expires_at', '>', now())
            ->select('m.id', 'm.company_id', 'c.name as company', 'm.full_name', 'm.email', 'm.role', 's.csrf_token')
            ->first();

        if ($session === null) {
            return null;
        }

        return [
            'member' => [
                'id' => $session->id,
                'companyId' => $session->company_id,
                'company' => $session->company,
                'name' => $session->full_name,
                'email' => $session->email,
                'role' => $session->role,
            ],
            'csrfToken' => $session->csrf_token,
        ];
    }

    /** @return array<string, mixed> */
    public function requireMember(Request $request, bool $write = false): array
    {
        $session = $this->current($request);
        if ($session === null) {
            throw new HttpResponseException(response()->json(['error' => 'Требуется вход'], 401));
        }
        if ($write && ($session['member']['role'] === 'viewer' || ! hash_equals($session['csrfToken'], (string) $request->header('X-CSRF-Token', '')) || ! $this->sameOrigin($request))) {
            throw new HttpResponseException(response()->json(['error' => 'Доступ запрещён'], 403));
        }

        return $session['member'];
    }

    public function sameOrigin(Request $request): bool
    {
        $origin = $request->header('Origin');
        if ($origin === null) {
            return true;
        }

        $url = parse_url((string) config('app.url'));
        if (! is_array($url) || ! isset($url['scheme'], $url['host'])) {
            return false;
        }

        $publicOrigin = $url['scheme'].'://'.$url['host'].(isset($url['port']) ? ':'.$url['port'] : '');

        return hash_equals($publicOrigin, $origin);
    }
}
