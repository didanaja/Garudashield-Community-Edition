"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import TrafficChart, { type TrafficSample } from "@/components/TrafficChart";
import TrafficTester from "@/components/TrafficTester";
import ConfigPanel from "@/components/ConfigPanel";
import { api, ApiError } from "@/lib/api";
import type { MetricsSnapshot, RuleInfo, SecurityEvent, SessionResponse } from "@/types";

const MAX_SAMPLES = 24;

function formatNumber(n: number): string {
  return n.toLocaleString("id-ID");
}

function VerdictBadge({ verdict }: { verdict: string }) {
  const styles: Record<string, string> = {
    allowed: "bg-emerald-50 text-emerald-700",
    blocked: "bg-rose-50 text-rose-700",
    rate_limited: "bg-amber-50 text-amber-700",
    evaluation_timeout: "bg-violet-50 text-violet-700",
    error: "bg-navy-100 text-navy-700",
  };
  return (
    <span className={`badge ${styles[verdict] ?? "bg-navy-100 text-navy-700"}`}>
      {verdict.replace("_", " ")}
    </span>
  );
}

function MetricCard({ label, value, accent }: { label: string; value: number; accent?: string }) {
  return (
    <div className="card p-5">
      <p className="text-xs font-semibold uppercase tracking-wider text-navy-500">{label}</p>
      <p className={`mt-2 text-2xl font-bold ${accent ?? "text-navy-900"}`}>{formatNumber(value)}</p>
    </div>
  );
}

type Tab = "overview" | "traffic" | "config";

const TABS: { id: Tab; label: string }[] = [
  { id: "overview", label: "Ringkasan" },
  { id: "traffic", label: "Traffic" },
  { id: "config", label: "Konfigurasi" },
];

export default function DashboardPage() {
  const router = useRouter();
  const [session, setSession] = useState<SessionResponse | null>(null);
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null);
  const [rules, setRules] = useState<RuleInfo[]>([]);
  const [logs, setLogs] = useState<SecurityEvent[]>([]);
  const [samples, setSamples] = useState<TrafficSample[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [tab, setTab] = useState<Tab>("overview");
  const prevRef = useRef<{ allowed: number; blocked: number; errors: number } | null>(null);

  useEffect(() => {
    api
      .session()
      .then((s) => {
        if (!s.authenticated) {
          router.replace("/login");
        } else {
          setSession(s);
        }
      })
      .catch(() => router.replace("/login"));
  }, [router]);

  const load = useCallback(async () => {
    try {
      const [m, r, l] = await Promise.all([api.metrics(), api.rules(), api.logs(30)]);
      setMetrics(m.metrics);
      setRules(r.rules);
      setLogs(l.events);
      setError(null);

      const errTotal = m.metrics.upstreamErrors + m.metrics.evaluationTimeouts;
      const prev = prevRef.current;
      const allowed = prev ? Math.max(0, m.metrics.allowed - prev.allowed) : m.metrics.allowed;
      const blocked = prev ? Math.max(0, m.metrics.blocked - prev.blocked) : m.metrics.blocked;
      const errors = prev ? Math.max(0, errTotal - prev.errors) : errTotal;
      prevRef.current = { allowed: m.metrics.allowed, blocked: m.metrics.blocked, errors: errTotal };
      setSamples((s) => [
        ...s.slice(-(MAX_SAMPLES - 1)),
        { label: new Date().toLocaleTimeString("id-ID"), allowed, blocked, errors },
      ]);
    } catch (e) {
      setError(e instanceof ApiError && e.status === 401 ? "Sesi tidak valid." : "Gagal memuat data.");
    }
  }, []);

  useEffect(() => {
    if (!session) return;
    let cancelled = false;
    const run = () => {
      if (!cancelled) load();
    };
    run();
    const id = setInterval(run, 8000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [session, load]);

  async function onLogout() {
    await api.logout().catch(() => undefined);
    router.push("/login");
  }

  if (!session) {
    return (
      <main className="flex min-h-screen flex-col">
        <Navbar />
        <div className="container-shell flex flex-1 items-center justify-center py-16">
          <p className="text-sm text-navy-500">Memeriksa sesi...</p>
        </div>
        <Footer />
      </main>
    );
  }

  return (
    <main className="flex min-h-screen flex-col">
      <Navbar />
      <div className="container-shell flex-1 py-10">
        <div className="mb-6 flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
          <div>
            <h1 className="text-2xl font-bold text-navy-900">Dashboard</h1>
            <p className="mt-1 text-sm text-navy-600">
              Metrics, traffic, dan konfigurasi dari aktivitas backend yang sebenarnya.
            </p>
          </div>
          <div className="flex items-center gap-3">
            <span className="text-sm text-navy-600">
              Halo, <strong className="text-navy-900">{session.user}</strong>
            </span>
            <button onClick={onLogout} className="btn-secondary !px-4 !py-2 text-xs">
              Keluar
            </button>
          </div>
        </div>

        <div className="mb-6 flex gap-1 rounded-xl border border-navy-100 bg-white p-1">
          {TABS.map((t) => (
            <button
              key={t.id}
              onClick={() => setTab(t.id)}
              className={`flex-1 rounded-lg px-4 py-2 text-sm font-medium transition ${
                tab === t.id ? "bg-navy-900 text-white" : "text-navy-600 hover:bg-navy-50"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>

        {error && (
          <div className="mb-6 rounded-xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700" role="alert">
            {error}
          </div>
        )}

        {tab === "overview" && (
          <>
            {metrics ? (
              <div className="mb-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                <MetricCard label="Requests Total" value={metrics.requestsTotal} />
                <MetricCard label="Allowed" value={metrics.allowed} accent="text-emerald-600" />
                <MetricCard label="Blocked" value={metrics.blocked} accent="text-rose-600" />
                <MetricCard label="Rate Limited" value={metrics.rateLimitEvents} accent="text-amber-600" />
              </div>
            ) : (
              <div className="mb-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                {[0, 1, 2, 3].map((i) => (
                  <div key={i} className="card h-24 animate-pulse p-5" />
                ))}
              </div>
            )}

            <div className="grid gap-6 lg:grid-cols-2">
              <div className="card p-6">
                <h2 className="mb-4 text-lg font-bold text-navy-900">Recent Security Events</h2>
                {logs.length === 0 ? (
                  <p className="text-sm text-navy-500">
                    Belum ada event. Buka tab Traffic untuk mengirim request.
                  </p>
                ) : (
                  <div className="max-h-96 overflow-y-auto">
                    <table className="w-full text-left text-sm">
                      <thead className="sticky top-0 bg-white">
                        <tr className="border-b border-navy-100 text-xs uppercase tracking-wider text-navy-500">
                          <th className="pb-2 pr-4">Waktu</th>
                          <th className="pb-2 pr-4">Method</th>
                          <th className="pb-2 pr-4">Path</th>
                          <th className="pb-2">Verdict</th>
                        </tr>
                      </thead>
                      <tbody>
                        {logs.map((e, i) => (
                          <tr key={i} className="border-b border-navy-50 last:border-0">
                            <td className="py-2 pr-4 font-mono text-xs text-navy-500">
                              {new Date(e.timestamp).toLocaleTimeString("id-ID")}
                            </td>
                            <td className="py-2 pr-4 font-medium text-navy-800">{e.method}</td>
                            <td className="max-w-[120px] truncate py-2 pr-4 text-navy-600">{e.path}</td>
                            <td className="py-2">
                              <VerdictBadge verdict={e.verdict} />
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>

              <div className="card p-6">
                <h2 className="mb-4 text-lg font-bold text-navy-900">Active Rules</h2>
                {rules.length === 0 ? (
                  <p className="text-sm text-navy-500">Rules tidak tersedia.</p>
                ) : (
                  <ul className="space-y-3">
                    {rules.map((r) => (
                      <li key={r.id} className="flex items-start justify-between gap-4">
                        <div>
                          <p className="text-sm font-semibold text-navy-800">{r.name}</p>
                          <p className="text-xs text-navy-500">{r.description}</p>
                        </div>
                        <span className="badge bg-navy-50 text-navy-600">{r.category}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </div>
          </>
        )}

        {tab === "traffic" && (
          <div className="space-y-6">
            <TrafficChart samples={samples} />
            <TrafficTester onSent={load} />
          </div>
        )}

        {tab === "config" && <ConfigPanel csrf={session.csrf ?? ""} />}
      </div>
      <Footer />
    </main>
  );
}
