"use client";

import { useState } from "react";
import { api, ApiError, ENGINE_BASE, type ProxyResult } from "@/lib/api";

type Preset = {
  label: string;
  method: string;
  path: string;
  note: string;
};

const PRESETS: Preset[] = [
  { label: "Normal", method: "GET", path: "/hello", note: "Expected: 200 allowed" },
  { label: "SQL injection", method: "GET", path: "/search?q=union+select+from+users", note: "Expected: 403 L7-SQLI" },
  { label: "XSS", method: "GET", path: "/search?q=<script>alert(1)</script>", note: "Expected: 403 L7-XSS" },
  { label: "Path traversal", method: "GET", path: "/search?file=../../etc/passwd", note: "Expected: 403 L7-PATH-TRAVERSAL" },
  { label: "Sensitive file", method: "GET", path: "/.env", note: "Expected: 403 L7-SENSITIVE-FILE" },
  { label: "Oversized URL", method: "GET", path: `/search?q=${"A".repeat(2500)}`, note: "Expected: 414 L7-URL-LEN" },
];

type Outcome = { ok: boolean; status: number; verdict: string; rule?: string; message?: string; raw: string };

function interpret(res: ProxyResult): Outcome {
  const body = res.body as Record<string, unknown> | null;
  if (res.status >= 200 && res.status < 400) {
    return { ok: true, status: res.status, verdict: "allowed", raw: describe(body) };
  }
  const rule = typeof body?.rule === "string" ? body.rule : undefined;
  const message = typeof body?.message === "string" ? body.message : undefined;
  return {
    ok: false,
    status: res.status,
    verdict: res.status === 429 ? "rate-limited" : "blocked",
    rule,
    message,
    raw: describe(body),
  };
}

function describe(body: unknown): string {
  if (body === null || body === undefined) return "(empty)";
  if (typeof body === "string") return body.slice(0, 400);
  try {
    return JSON.stringify(body, null, 2).slice(0, 400);
  } catch {
    return String(body);
  }
}

export default function TrafficTester({ onSent }: { onSent: () => void }) {
  const [method, setMethod] = useState("GET");
  const [path, setPath] = useState("/hello");
  const [busy, setBusy] = useState(false);
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function send() {
    setBusy(true);
    setError(null);
    try {
      const res = await api.sendProxy(method.toUpperCase(), path);
      setOutcome(interpret(res));
      onSent();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Gagal mengirim request.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="card p-6">
      <h2 className="text-lg font-bold text-navy-900">Traffic Tester</h2>
      <p className="mt-1 text-sm text-navy-600">
        Kirim request nyata langsung ke protection engine{" "}
        <span className="font-mono text-navy-800">{ENGINE_BASE}</span>. Hasil langsung
        terlihat di metrik dan event log.
      </p>

      <div className="mt-4 flex flex-wrap gap-2">
        {PRESETS.map((p) => (
          <button
            key={p.label}
            type="button"
            title={p.note}
            onClick={() => {
              setMethod(p.method);
              setPath(p.path);
            }}
            className={`badge border ${
              method === p.method && path === p.path
                ? "border-navy-900 bg-navy-900 text-white"
                : "border-navy-200 bg-white text-navy-700 hover:border-navy-400"
            }`}
          >
            {p.label}
          </button>
        ))}
      </div>

      <div className="mt-4 flex flex-col gap-3 sm:flex-row">
        <select
          className="input sm:w-32"
          value={method}
          onChange={(e) => setMethod(e.target.value)}
        >
          {["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"].map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
        <input
          className="input flex-1 font-mono text-sm"
          value={path}
          onChange={(e) => setPath(e.target.value)}
          placeholder="/hello"
        />
        <button type="button" className="btn-primary" onClick={send} disabled={busy}>
          {busy ? "Mengirim..." : "Kirim"}
        </button>
      </div>

      {error && (
        <div className="mt-4 rounded-lg border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700">
          {error}
        </div>
      )}

      {outcome && (
        <div className="mt-4 rounded-xl border border-navy-100 bg-navy-50/60 p-4">
          <div className="flex items-center gap-3">
            <span
              className={`badge ${
                outcome.ok ? "bg-emerald-100 text-emerald-700" : "bg-rose-100 text-rose-700"
              }`}
            >
              {outcome.verdict}
            </span>
            <span className="font-mono text-sm text-navy-800">HTTP {outcome.status}</span>
            {outcome.rule && (
              <span className="badge bg-navy-800 text-white">rule: {outcome.rule}</span>
            )}
          </div>
          {outcome.message && <p className="mt-2 text-sm text-navy-700">{outcome.message}</p>}
          <pre className="mt-3 max-h-48 overflow-auto rounded-lg bg-navy-900 p-3 text-xs text-navy-100">
            {outcome.raw}
          </pre>
        </div>
      )}
    </div>
  );
}
