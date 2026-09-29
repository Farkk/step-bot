<?php

namespace App;

use Carbon\CarbonImmutable;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;

class Analytics
{
    /** @return array{CarbonImmutable, CarbonImmutable}|null */
    public function period(Request $request): ?array
    {
        $now = CarbonImmutable::now('UTC');
        $from = $request->query('from') === null ? $now->subMonth() : $this->date((string) $request->query('from'));
        $to = $request->query('to') === null ? $now->addSecond() : $this->date((string) $request->query('to'))?->addDay();
        if ($from === null || $to === null || $from >= $to || $from->diffInDays($to) > 366) {
            return null;
        }

        return [$from, $to];
    }

    public function metrics(int $companyId, CarbonImmutable $from, CarbonImmutable $to): array
    {
        $ratings = DB::table('task_ratings as r')->join('tasks as t', 't.id', '=', 'r.task_id')
            ->where('t.company_id', $companyId)->where('t.completed_at', '>=', $from)->where('t.completed_at', '<', $to)
            ->selectRaw('count(*) as total, avg(r.score) as average')->first();
        $completed = DB::table('tasks')->where('company_id', $companyId)->where('status', 'completed')
            ->where('completed_at', '>=', $from)->where('completed_at', '<', $to)->count();
        $events = DB::table('task_events as e')->join('tasks as t', 't.id', '=', 'e.task_id')
            ->where('t.company_id', $companyId)->whereIn('e.kind', ['application_created', 'application_accepted', 'application_rejected'])
            ->orderBy('e.created_at')->orderBy('e.id')->get(['e.task_id', 'e.kind', 'e.created_at']);
        $firstApplications = [];
        $firstDecisions = [];
        foreach ($events as $event) {
            if ($event->kind === 'application_created') {
                $firstApplications[$event->task_id] ??= CarbonImmutable::parse($event->created_at, 'UTC');
            } else {
                $firstDecisions[$event->task_id] ??= CarbonImmutable::parse($event->created_at, 'UTC');
            }
        }
        $reactionHours = [];
        foreach ($firstDecisions as $taskId => $decision) {
            if (isset($firstApplications[$taskId]) && $decision >= $from && $decision < $to) {
                $reactionHours[] = $firstApplications[$taskId]->diffInSeconds($decision) / 3600;
            }
        }

        return [
            'averageRating' => (float) ($ratings->average ?? 0), 'ratingCount' => (int) ($ratings->total ?? 0),
            'reactionHours' => $reactionHours === [] ? 0 : array_sum($reactionHours) / count($reactionHours),
            'reactionCount' => count($reactionHours), 'completed' => $completed,
            'from' => $from->format('Y-m-d'), 'to' => $to->subMicrosecond()->format('Y-m-d'),
        ];
    }

    private function date(string $value): ?CarbonImmutable
    {
        $date = CarbonImmutable::createFromFormat('!Y-m-d', $value, 'UTC');

        return $date !== false && $date->format('Y-m-d') === $value ? $date : null;
    }
}
