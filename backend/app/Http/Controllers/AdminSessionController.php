<?php

namespace App\Http\Controllers;

use App\AdminAccess;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

class AdminSessionController extends Controller
{
    public function show(Request $request, AdminAccess $access): JsonResponse
    {
        $session = $access->current($request);

        return $session === null
            ? response()->json(['error' => 'Требуется вход'], 401)
            : response()->json($session);
    }
}
