<?php

namespace App\Http\Controllers;

use App\AdminAccess;
use App\Analytics;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Symfony\Component\HttpFoundation\Response;

class AnalyticsController extends Controller
{
    public function index(Request $request, AdminAccess $access, Analytics $analytics): JsonResponse
    {
        $member = $access->requireMember($request);
        $period = $analytics->period($request);

        return $period === null ? response()->json(['error' => 'Проверьте период'], 400)
            : response()->json($analytics->metrics($member['companyId'], ...$period));
    }

    public function trend(Request $request, AdminAccess $access, Analytics $analytics): JsonResponse
    {
        $member = $access->requireMember($request);
        $period = $analytics->period($request);
        if ($period === null) {
            return response()->json(['error' => 'Проверьте период'], 400);
        }
        [$from, $to] = $period;
        $days = max(1, (int) ceil($from->diffInSeconds($to) / 86400));
        $buckets = min(7, $days);
        $points = [];
        for ($i = 0; $i < $buckets; $i++) {
            $start = $from->addDays(intdiv($i * $days, $buckets));
            $end = $from->addDays(intdiv(($i + 1) * $days, $buckets))->min($to);
            $metric = $analytics->metrics($member['companyId'], $start, $end);
            $metric['from'] = $start->format('d.m');
            $metric['to'] = $end->subMicrosecond()->format('d.m');
            $points[] = $metric;
        }

        return response()->json($points);
    }

    public function csv(Request $request, AdminAccess $access, Analytics $analytics): Response
    {
        $member = $access->requireMember($request);
        $period = $analytics->period($request);
        if ($period === null) {
            return response('Проверьте период', 400);
        }
        $metric = $analytics->metrics($member['companyId'], ...$period);
        $stream = fopen('php://temp', 'w+');
        fputcsv($stream, ['Период с', 'Период по', 'Средний рейтинг', 'Количество оценок', 'Время реакции, часы', 'Количество реакций', 'Закрытые задачи']);
        fputcsv($stream, [$metric['from'], $metric['to'], number_format($metric['averageRating'], 2, '.', ''), $metric['ratingCount'], number_format($metric['reactionHours'], 2, '.', ''), $metric['reactionCount'], $metric['completed']]);
        rewind($stream);
        $body = stream_get_contents($stream);
        fclose($stream);

        return response($body, 200, ['Content-Type' => 'text/csv; charset=utf-8', 'Content-Disposition' => 'attachment; filename="step-analytics.csv"']);
    }
}
