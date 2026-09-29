<?php

namespace App\Http\Controllers;

use App\MaxLaunchVerifier;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;

class ProfileController extends Controller
{
    public function show(Request $request, MaxLaunchVerifier $verifier): JsonResponse
    {
        $maxUser = $verifier->authenticate($request);
        if ($maxUser === null) {
            return response()->json(['error' => 'Недействительные данные запуска MAX'], 401);
        }

        $profile = DB::table('max_identities as m')
            ->join('users as u', 'u.id', '=', 'm.user_id')
            ->where('m.max_id', $maxUser['id'])
            ->select('u.full_name', 'u.phone', 'u.gender', 'u.age', 'u.phone_verified')
            ->first();

        return response()->json($profile === null
            ? ['registered' => false]
            : ['registered' => true, 'profile' => $this->profileResponse($profile)]);
    }

    public function update(Request $request, MaxLaunchVerifier $verifier): JsonResponse
    {
        $maxUser = $verifier->authenticate($request);
        if ($maxUser === null) {
            return response()->json(['error' => 'Недействительные данные запуска MAX'], 401);
        }

        if (strlen($request->getContent()) > 16384 || array_diff(array_keys($request->all()), ['fullName', 'phone', 'gender', 'age', 'phoneVerified', 'contactProof']) !== []) {
            return response()->json(['error' => 'Некорректные данные профиля'], 400);
        }

        $name = preg_replace('/\s+/u', ' ', trim((string) $request->input('fullName', '')));
        $phone = preg_replace('/[\s()\-]/u', '', (string) $request->input('phone', ''));
        $parts = explode(' ', $name);
        $validName = count($parts) >= 2 && mb_strlen($name) <= 150;
        foreach ($parts as $part) {
            $validName = $validName && preg_match("/^[\\p{L}][\\p{L}'’-]+$/u", $part) === 1;
        }
        $age = $request->input('age');
        if (! $validName || preg_match('/^\+?[0-9]{10,15}$/', $phone) !== 1 || ! is_int($age) || $age < 1 || $age > 120 || ! in_array($request->input('gender'), ['male', 'female'], true)) {
            return response()->json(['error' => 'Проверьте поля профиля'], 400);
        }

        $proof = $request->input('contactProof');
        $verified = false;
        if ($proof !== null) {
            $verified = is_array($proof) && $verifier->verifyContact(
                $phone,
                (string) ($proof['authDate'] ?? ''),
                (string) ($proof['hash'] ?? ''),
                $maxUser['id'],
                (string) config('step.max_bot_token'),
                time(),
            );
            if (! $verified) {
                return response()->json(['error' => 'Номер из MAX не подтверждён'], 400);
            }
        }

        [$id, $verified] = DB::transaction(function () use ($maxUser, $name, $phone, $age, $request, $verified): array {
            $identity = DB::table('max_identities')->where('max_id', $maxUser['id'])->lockForUpdate()->first();
            $displayName = trim($maxUser['first_name'].' '.$maxUser['last_name']);
            if ($identity === null) {
                $id = DB::table('users')->insertGetId([
                    'full_name' => $name,
                    'phone' => $phone,
                    'phone_verified' => $verified,
                    'gender' => $request->input('gender'),
                    'age' => $age,
                    'created_at' => now(),
                    'updated_at' => now(),
                ]);
                DB::table('max_identities')->insert([
                    'max_id' => $maxUser['id'],
                    'user_id' => $id,
                    'display_name' => $displayName,
                    'updated_at' => now(),
                ]);

                return [$id, $verified];
            }

            $id = $identity->user_id;
            $old = DB::table('users')->where('id', $id)->first();
            $verified = $verified || ($old->phone === $phone && (bool) $old->phone_verified);
            DB::table('users')->where('id', $id)->update([
                'full_name' => $name,
                'phone' => $phone,
                'phone_verified' => $verified,
                'gender' => $request->input('gender'),
                'age' => $age,
                'updated_at' => now(),
            ]);
            DB::table('max_identities')->where('max_id', $maxUser['id'])->update(['display_name' => $displayName, 'updated_at' => now()]);

            return [$id, $verified];
        });

        return response()->json([
            'registered' => true,
            'profile' => [
                'fullName' => $name,
                'phone' => $phone,
                'gender' => $request->input('gender'),
                'age' => $age,
                'phoneVerified' => $verified,
            ],
            'userId' => (string) $id,
        ]);
    }

    private function profileResponse(object $profile): array
    {
        return [
            'fullName' => $profile->full_name,
            'phone' => $profile->phone,
            'gender' => $profile->gender,
            'age' => (int) $profile->age,
            'phoneVerified' => (bool) $profile->phone_verified,
        ];
    }
}
