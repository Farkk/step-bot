<?php

namespace App\Http\Controllers;

use App\AdminAccess;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\RateLimiter;
use Symfony\Component\HttpFoundation\Response;

class AdminAuthController extends Controller
{
    public function login(Request $request): JsonResponse
    {
        $key = 'admin-login:'.$request->ip();
        if (RateLimiter::tooManyAttempts($key, 10)) {
            return response()->json(['error' => 'Слишком много попыток. Попробуйте позже'], 429);
        }
        $email = mb_strtolower(trim((string) $request->input('email', '')));
        $password = (string) $request->input('password', '');
        if (strlen($request->getContent()) > 4096 || strlen($email) > 254 || preg_match('/^[^\s@]+@[^\s@]+\.[^\s@]+$/u', $email) !== 1) {
            return response()->json(['error' => 'Некорректный запрос'], 400);
        }

        $member = DB::table('company_members as m')
            ->join('companies as c', 'c.id', '=', 'm.company_id')
            ->where('m.email', $email)
            ->select('m.*', 'c.name as company')
            ->first();
        $hash = $member?->password_hash ?? '$2a$12$kfb.aNoZb1gBEqZ7PtDb6eBvgLmPAobPfKYZBxGaIkCQfvSEpXXkW';
        if (! password_verify($password, $hash) || $member === null) {
            RateLimiter::hit($key, 600);

            return response()->json(['error' => 'Неверная почта или пароль'], 401);
        }
        RateLimiter::clear($key);

        $token = bin2hex(random_bytes(32));
        $csrf = bin2hex(random_bytes(32));
        DB::table('admin_sessions')->insert([
            'token_hash' => hash('sha256', $token),
            'member_id' => $member->id,
            'csrf_token' => $csrf,
            'expires_at' => now()->addDays(7),
        ]);

        return response()->json([
            'member' => [
                'id' => $member->id,
                'companyId' => $member->company_id,
                'company' => $member->company,
                'name' => $member->full_name,
                'email' => $member->email,
                'role' => $member->role,
            ],
            'csrfToken' => $csrf,
        ])->withCookie(cookie('step_admin', $token, 7 * 24 * 60, '/api/v1', null, ! app()->isLocal(), true, false, 'strict'));
    }

    public function logout(Request $request, AdminAccess $access): Response
    {
        $session = $access->current($request);
        if ($session === null) {
            return response('Требуется вход', 401);
        }
        if (! $access->sameOrigin($request) || ! hash_equals($session['csrfToken'], (string) $request->header('X-CSRF-Token', ''))) {
            return response('Доступ запрещён', 403);
        }

        DB::table('admin_sessions')->where('token_hash', hash('sha256', (string) $request->cookie('step_admin')))->delete();

        return response('', 204)->withCookie(cookie('step_admin', '', -1, '/api/v1', null, ! app()->isLocal(), true, false, 'strict'));
    }

    public function changePassword(Request $request, AdminAccess $access): Response
    {
        $session = $access->current($request);
        if ($session === null) {
            return response('Требуется вход', 401);
        }
        if (! $access->sameOrigin($request) || ! hash_equals($session['csrfToken'], (string) $request->header('X-CSRF-Token', ''))) {
            return response('Доступ запрещён', 403);
        }
        $next = (string) $request->input('next', '');
        if (strlen($request->getContent()) > 4096 || strlen($next) < 12 || strlen($next) > 128) {
            return response('Новый пароль должен содержать 12–128 символов', 400);
        }
        $id = $session['member']['id'];
        $hash = DB::table('company_members')->where('id', $id)->value('password_hash');
        if (! password_verify((string) $request->input('current', ''), $hash)) {
            return response('Текущий пароль неверен', 403);
        }
        DB::table('company_members')->where('id', $id)->update(['password_hash' => password_hash($next, PASSWORD_BCRYPT)]);

        return response('', 204);
    }
}
