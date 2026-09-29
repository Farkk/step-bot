<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Artisan;
use Illuminate\Support\Facades\Log;
use Symfony\Component\HttpFoundation\Response;
use Throwable;

class DeliverMaxAfterResponse
{
    public function handle(Request $request, Closure $next): Response
    {
        return $next($request);
    }

    public function terminate(Request $request, Response $response): void
    {
        if ($response->getStatusCode() >= 400 || ! in_array($request->method(), ['POST', 'PUT', 'PATCH', 'DELETE'], true)) {
            return;
        }
        if (! $request->is('api/v1/*') && ! $request->is('integrations/max/webhook')) {
            return;
        }

        try {
            Artisan::call('max:tick', ['--limit' => 100, '--seconds' => 15]);
        } catch (Throwable $exception) {
            Log::warning('Immediate MAX delivery failed; cron will retry', ['exception' => $exception]);
        }
    }
}
