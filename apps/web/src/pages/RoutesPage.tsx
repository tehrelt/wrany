import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { AppLayout } from "@/components/layout/AppLayout";
import { RouteMap } from "@/components/map/RouteMap";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { useAuth } from "@/features/auth/useAuth";
import {
  listRoutes,
  getRoutePoints,
  getRouteResults,
  getRouteAttempts,
  formatDistance,
  formatDuration,
  type Route,
  type RouteResultResponse,
  type TripAttemptItem,
} from "@/features/routes/routesApi";

interface Props {
  onLogout: () => void;
}

function RouteCard({
  route,
  selected,
  onClick,
}: {
  route: Route;
  selected: boolean;
  onClick: () => void;
}) {
  const date = new Date(route.updated_at);
  const dateStr = date.toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
  });

  return (
    <button
      onClick={onClick}
      className={[
        "w-full text-left rounded-lg border p-3 transition-colors hover:bg-accent",
        selected ? "border-primary bg-accent" : "border-border",
      ].join(" ")}
    >
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs text-muted-foreground">{dateStr}</span>
        <Badge variant="secondary" className="text-xs">
          {route.trips_count} {route.trips_count !== 1 ? "runs" : "run"}
        </Badge>
      </div>
      <div className="flex gap-4 text-sm font-medium">
        <span>{formatDistance(route.distance_m)}</span>
      </div>
      <div className="text-xs text-muted-foreground mt-1 truncate">
        {route.name ?? `Route ${route.id.slice(0, 8)}`}
      </div>
    </button>
  );
}

function formatSpeed(mps: number | undefined): string {
  if (!mps) return "—";
  return `${mps} m/s`;
}

function TripResultCard({
  label,
  trip,
}: {
  label: string;
  trip: {
    trip_id?: string;
    started_at?: string;
    duration_sec?: number;
    distance_m?: number;
    avg_speed_mps?: number;
  };
}) {
  const date = trip.started_at
    ? new Date(trip.started_at).toLocaleDateString(undefined, {
        month: "short",
        day: "numeric",
      })
    : "—";
  return (
    <div className="rounded-lg bg-muted/40 p-3 flex-1">
      <div className="text-xs text-muted-foreground font-medium mb-1">
        {label}
      </div>
      <div className="font-semibold text-sm">
        {formatDuration(trip.duration_sec ?? 0)}
      </div>
      <div className="text-xs text-muted-foreground mt-0.5">
        {date} · {formatDistance(trip.distance_m ?? 0)} ·{" "}
        {formatSpeed(trip.avg_speed_mps)}
      </div>
    </div>
  );
}

function PersonalRecordsSection({ result }: { result: RouteResultResponse }) {
  const { best, latest, comparison, attempts_count } = result;

  if (!attempts_count) {
    return (
      <div className="py-4 text-center">
        <p className="text-sm text-muted-foreground">No runs recorded yet.</p>
        <p className="text-xs text-muted-foreground mt-1">
          Complete a tracked trip on this route to see your results.
        </p>
      </div>
    );
  }

  const isPersonalRecord = comparison?.latest_vs_best_sec === 0;
  const diff = comparison?.latest_vs_best_sec ?? 0;

  return (
    <div className="space-y-2">
      <div className="flex gap-2">
        {best && <TripResultCard label="Best" trip={best} />}
        {latest && <TripResultCard label="Latest" trip={latest} />}
        <div className="rounded-lg bg-muted/40 p-3 flex flex-col justify-center items-center gap-1 min-w-[80px]">
          <div className="text-xs text-muted-foreground font-medium">
            vs Best
          </div>
          {isPersonalRecord ? (
            <Badge variant="default" className="text-xs">
              PR
            </Badge>
          ) : (
            <span
              className={
                diff > 0
                  ? "text-orange-500 font-semibold text-sm"
                  : "text-green-600 font-semibold text-sm"
              }
            >
              {diff > 0 ? `+${diff}s` : `${diff}s`}
            </span>
          )}
          <div className="text-xs text-muted-foreground">
            {attempts_count} {attempts_count !== 1 ? "runs" : "run"}
          </div>
        </div>
      </div>
    </div>
  );
}

function AttemptsTable({ attempts }: { attempts: TripAttemptItem[] }) {
  if (attempts.length === 0) {
    return (
      <div className="py-4 text-center">
        <p className="text-xs text-muted-foreground">
          No attempts recorded for this route yet.
        </p>
      </div>
    );
  }
  return (
    <table className="w-full text-xs">
      <thead>
        <tr className="text-muted-foreground border-b">
          <th className="text-left pb-1 pr-3 font-medium">Date</th>
          <th className="text-right pb-1 pr-3 font-medium">Distance</th>
          <th className="text-right pb-1 pr-3 font-medium">Duration</th>
          <th className="text-right pb-1 pr-3 font-medium">Speed</th>
          <th className="text-right pb-1 font-medium">Match</th>
        </tr>
      </thead>
      <tbody>
        {attempts.map((a) => {
          const date = a.started_at
            ? new Date(a.started_at).toLocaleDateString(undefined, {
                month: "short",
                day: "numeric",
                hour: "2-digit",
                minute: "2-digit",
              })
            : "—";
          return (
            <tr
              key={a.trip_id}
              className={[
                "border-b last:border-0 hover:bg-accent/50",
                a.is_best ? "bg-yellow-50 dark:bg-yellow-950/20" : "",
              ].join(" ")}
            >
              <td className="py-1.5 pr-3 text-muted-foreground">
                {a.is_best && <span className="text-yellow-600 mr-1">★</span>}
                {date}
              </td>
              <td className="py-1.5 pr-3 text-right">
                {formatDistance(a.distance_m ?? 0)}
              </td>
              <td className="py-1.5 pr-3 text-right font-medium">
                {formatDuration(a.duration_sec ?? 0)}
              </td>
              <td className="py-1.5 pr-3 text-right">
                {formatSpeed(a.avg_speed_mps)}
              </td>
              <td className="py-1.5 text-right">
                {((a.match_score ?? 0) * 100).toFixed(0)}%
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

export function RoutesPage({ onLogout }: Props) {
  const { token } = useAuth();
  const [selectedRoute, setSelectedRoute] = useState<Route | null>(null);

  const routesQuery = useQuery({
    queryKey: ["routes"],
    queryFn: () => listRoutes({ limit: 50 }),
  });

  const resultsQuery = useQuery({
    queryKey: ["route-results", selectedRoute?.id],
    queryFn: () => getRouteResults(selectedRoute!.id),
    enabled: !!selectedRoute,
  });

  const attemptsQuery = useQuery({
    queryKey: ["route-attempts", selectedRoute?.id],
    queryFn: () => getRouteAttempts(selectedRoute!.id, { limit: 50 }),
    enabled: !!selectedRoute,
  });

  const pointsQuery = useQuery({
    queryKey: ["route-points", selectedRoute?.id],
    queryFn: () => getRoutePoints(selectedRoute!.id),
    enabled: !!selectedRoute,
  });

  let userEmail = "";
  if (token) {
    try {
      userEmail =
        (JSON.parse(atob(token.split(".")[1])) as { sub?: string }).sub ?? "";
    } catch {
      // ignore decode errors
    }
  }

  const routes = routesQuery.data?.items ?? [];

  const sidebar = (
    <div className="flex flex-col gap-3 h-full">
      <div className="shrink-0">
        <h2 className="text-sm font-semibold">Routes</h2>
        <p className="text-xs text-muted-foreground mt-0.5">
          {routes.length} {routes.length !== 1 ? "routes" : "route"} detected
        </p>
      </div>

      {routesQuery.isLoading && (
        <div className="flex flex-col gap-2">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full rounded-lg" />
          ))}
        </div>
      )}

      {routesQuery.isError && (
        <Alert variant="destructive">
          <AlertDescription>Could not load routes. Try again.</AlertDescription>
        </Alert>
      )}

      {!routesQuery.isLoading &&
        !routesQuery.isError &&
        routes.length === 0 && (
          <div className="flex flex-col items-center gap-2 py-10 px-2 text-center">
            <p className="text-sm font-medium text-muted-foreground">
              No routes discovered yet
            </p>
            <p className="text-xs text-muted-foreground leading-relaxed">
              Routes are detected automatically when you repeat similar trips.
            </p>
          </div>
        )}

      <div className="flex flex-col gap-2 overflow-y-auto flex-1">
        {routes.map((r) => (
          <RouteCard
            key={r.id}
            route={r}
            selected={selectedRoute?.id === r.id}
            onClick={() => setSelectedRoute(r)}
          />
        ))}
      </div>
    </div>
  );

  return (
    <AppLayout userEmail={userEmail} onLogout={onLogout} sidebar={sidebar}>
      <div className="flex flex-col flex-1 overflow-hidden">
        <div className="flex-1 min-h-0">
          <RouteMap
            points={pointsQuery.data ?? []}
            startPoint={
              selectedRoute
                ? {
                    lat: selectedRoute.start_lat,
                    lon: selectedRoute.start_lon,
                  }
                : undefined
            }
            finishPoint={
              selectedRoute
                ? {
                    lat: selectedRoute.end_lat,
                    lon: selectedRoute.end_lon,
                  }
                : undefined
            }
          />
        </div>

        {selectedRoute && (
          <div className="border-t shrink-0 overflow-y-auto max-h-80">
            <div className="p-4 space-y-4">
              <h3 className="text-sm font-semibold">
                {selectedRoute.name ?? `Route ${selectedRoute.id.slice(0, 8)}`}
              </h3>

              {(resultsQuery.isLoading || attemptsQuery.isLoading) && (
                <Skeleton className="h-16 w-full" />
              )}

              {(resultsQuery.isError || attemptsQuery.isError) && (
                <Alert variant="destructive">
                  <AlertDescription>
                    Could not load results. Try again.
                  </AlertDescription>
                </Alert>
              )}

              {resultsQuery.data && (
                <PersonalRecordsSection result={resultsQuery.data} />
              )}

              {attemptsQuery.data && (
                <AttemptsTable attempts={attemptsQuery.data.items} />
              )}
            </div>
          </div>
        )}
      </div>
    </AppLayout>
  );
}
