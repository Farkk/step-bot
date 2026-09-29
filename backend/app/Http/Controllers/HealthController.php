<?php

namespace App\Http\Controllers;

use Illuminate\Http\JsonResponse;
use Illuminate\Support\Facades\DB;
use Throwable;

class HealthController extends Controller
{
    public function live(): JsonResponse
    {
        return response()->json(['status' => 'live']);
    }

    public function ready(): JsonResponse
    {
        try {
            DB::select('SELECT 1');

            if (! is_writable(storage_path('app/private'))) {
                throw new \RuntimeException('private storage unavailable');
            }

            return response()->json(['status' => 'ready']);
        } catch (Throwable) {
            return response()->json(['error' => 'dependency unavailable'], 503);
        }
    }
}
