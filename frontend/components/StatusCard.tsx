"use client";

import { useEffect, useState } from "react";
import type { StatusResponse } from "@/types";

const COMPONENTS = ["Shield", "Proxy", "RedisStore", "Upstream"] as const;

function StatusDot({ status }: { status: string }) {
  const color =
    status === "healthy"
      ? "bg-emerald-500"
      : status === "degraded"
        ? "bg-amber-500"
        : "bg-rose-500";
  return <span className={`h-2 w-2 rounded-full ${color}`} aria-hidden="true" />;
}

export default function StatusCard() {
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    async function poll() {
      try {
        const res = await fetch("/api/v1/status", { cache: "no-store" });
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const data = (await res.json()) as StatusResponse;
        if (!cancelled) {
          setStatus(data);
          setError(null);
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : "offline");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    poll();
    const id = setInterval(poll, 10000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

  return (
    <div className="card w-full max-w-sm p-6" id="status">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-navy-500">
          Status Engine
        </h2>
        <span className="badge bg-navy-50 text-navy-700">Demo Edition</span>
      </div>

      {loading && (
        <div className="space-y-3" aria-live="polite">
          {COMPONENTS.map((c) => (
            <div key={c} className="flex items-center justify-between">
              <span className="text-sm text-navy-600">{c}</span>
              <span className="h-2 w-16 animate-pulse rounded bg-navy-100" />
            </div>
          ))}
        </div>
      )}

      {error && !loading && (
        <div className="rounded-xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700" role="alert">
          <p className="font-semibold">Engine offline</p>
          <p className="mt-1 text-rose-600">Tidak dapat terhubung ke backend. Pastikan server Go berjalan.</p>
        </div>
      )}

      {status && !loading && (
        <ul className="space-y-3">
          {status.components.map((c) => (
            <li key={c.name} className="flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <StatusDot status={c.status} />
                <span className="text-sm font-medium text-navy-800">{c.name}</span>
              </div>
              <span
                className={`badge ${
                  c.status === "healthy"
                    ? "bg-emerald-50 text-emerald-700"
                    : c.status === "degraded"
                      ? "bg-amber-50 text-amber-700"
                      : "bg-rose-50 text-rose-700"
                }`}
              >
                {c.status === "healthy" ? "Healthy" : c.status === "degraded" ? "Degraded" : "Down"}
              </span>
            </li>
          ))}
        </ul>
      )}

      {status && !loading && (
        <p className="mt-4 border-t border-navy-100 pt-3 text-xs text-navy-500">
          Overall: <span className="font-semibold capitalize text-navy-800">{status.overall}</span>
        </p>
      )}
    </div>
  );
}
