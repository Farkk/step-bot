<?php

namespace App\Http\Controllers;

use App\AdminAccess;
use Illuminate\Database\QueryException;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;

class AdminMemberController extends Controller
{
    public function index(Request $request, AdminAccess $access): JsonResponse
    {
        $member = $access->requireMember($request);
        $members = DB::table('company_members')
            ->where('company_id', $member['companyId'])
            ->orderBy('id')
            ->get(['id', 'full_name as name', 'email', 'role']);

        return response()->json($members);
    }

    public function store(Request $request, AdminAccess $access): JsonResponse
    {
        $member = $access->requireMember($request, true);
        if ($member['role'] !== 'owner') {
            return response()->json(['error' => 'Только владелец управляет командой'], 403);
        }

        $name = trim((string) $request->input('name', ''));
        $email = mb_strtolower(trim((string) $request->input('email', '')));
        $role = $request->input('role');
        $password = (string) $request->input('password', '');
        if (strlen($request->getContent()) > 4096 || mb_strlen($name) < 3 || mb_strlen($name) > 150 || strlen($email) > 254 || ! filter_var($email, FILTER_VALIDATE_EMAIL) || ! in_array($role, ['manager', 'viewer'], true) || strlen($password) < 12) {
            return response()->json(['error' => 'Укажите имя, email, роль и пароль от 12 символов'], 400);
        }

        try {
            $id = DB::table('company_members')->insertGetId([
                'company_id' => $member['companyId'],
                'email' => $email,
                'password_hash' => password_hash($password, PASSWORD_BCRYPT),
                'full_name' => $name,
                'role' => $role,
                'created_at' => now(),
            ]);
        } catch (QueryException) {
            return response()->json(['error' => 'Не удалось создать сотрудника; проверьте email'], 409);
        }

        return response()->json(['id' => $id, 'name' => $name, 'email' => $email, 'role' => $role], 201);
    }

    public function changeRole(Request $request, AdminAccess $access, string $id): JsonResponse
    {
        $member = $access->requireMember($request, true);
        if ($member['role'] !== 'owner') {
            return response()->json(['error' => 'Только владелец управляет командой'], 403);
        }
        $role = $request->input('role');
        if (! ctype_digit($id) || (int) $id < 1 || ! in_array($role, ['manager', 'viewer'], true)) {
            return response()->json(['error' => 'Некорректная роль'], 400);
        }
        $changed = DB::table('company_members')
            ->where('id', (int) $id)
            ->where('company_id', $member['companyId'])
            ->where('role', '!=', 'owner')
            ->update(['role' => $role]);

        return $changed === 0
            ? response()->json(['error' => 'Сотрудник не найден'], 404)
            : response()->json(['id' => (int) $id, 'role' => $role]);
    }
}
