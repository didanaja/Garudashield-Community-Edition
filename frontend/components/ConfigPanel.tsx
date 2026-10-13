"use client";

import { useCallback, useEffect, useState } from "react";
import { api, ApiError } from "@/lib/api";
import type { SettingsView, UpstreamView } from "@/types";

type Msg = { kind: "ok" | "err"; text: string } | null;

export default function ConfigPanel({ csrf }: { csrf: string }) {
  const [settings, setSettings] = useState<SettingsView | null>(null);
  const [upstream, setUpstream] = useState<UpstreamView | null>(null);
  const [upstreamUrl, setUpstreamUrl] = useState("");
  const [msg, setMsg] = useState<Msg>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<"settings" | "upstream" | null>(null);

  const load = useCallback(async () => {
    try {
      const [s, u] = await Promise.all([api.settings(), api.upstream()]);
      setSettings(s);
      setUpstream(u);
      setUpstreamUrl(u.upstreamUrl);
    } catch (e) {
      setMsg({ kind: "err", text: e instanceof ApiError ? "Gagal memuat konfigurasi." : "Gagal memuat konfigurasi." });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  function patch<K extends keyof SettingsView>(key: K, value: SettingsView[K]) {
    setSettings((prev) => (prev ? { ...prev, [key]: value } : prev));
  }

  async function saveSettings() {
    if (!settings) return;
    setBusy("settings");
    setMsg(null);
    try {
      const payload = {
        rateLimitEnabled: settings.rateLimitEnabled,
        rateLimitBurst: settings.rateLimitBurst,
        rateLimitWindowSeconds: settings.rateLimitWindowSeconds,
        maxBodyBytes: settings.maxBodyBytes,
        maxUrlBytes: settings.maxUrlBytes,
        maxHeaderBytes: settings.maxHeaderBytes,
        evaluationBudgetMs: settings.evaluationBudgetMs,
        failClosed: settings.failClosed,
      };
      const updated = await api.saveSettings(payload, csrf);
      setSettings(updated);
      setMsg({ kind: "ok", text: "Settings diterapkan." });
    } catch (e) {
      setMsg({ kind: "err", text: e instanceof ApiError ? e.message : "Gagal menyimpan settings." });
    } finally {
      setBusy(null);
    }
  }

  async function saveUpstream() {
    setBusy("upstream");
    setMsg(null);
    try {
      const updated = await api.setUpstream(upstreamUrl, csrf);
      setUpstream(updated);
      setUpstreamUrl(updated.upstreamUrl);
      setMsg({ kind: "ok", text: "Upstream diperbarui." });
    } catch (e) {
      setMsg({ kind: "err", text: e instanceof ApiError ? "Upstream tidak diizinkan." : "Gagal menyimpan upstream." });
    } finally {
      setBusy(null);
    }
  }

  if (loading) {
    return <div className="card p-6 text-sm text-navy-500">Memuat konfigurasi...</div>;
  }

  return (
    <div className="space-y-6">
      {msg && (
        <div
          className={`rounded-lg border p-3 text-sm ${
            msg.kind === "ok"
              ? "border-emerald-200 bg-emerald-50 text-emerald-700"
              : "border-rose-200 bg-rose-50 text-rose-700"
          }`}
        >
          {msg.text}
        </div>
      )}

      <div className="card p-6">
        <h2 className="text-lg font-bold text-navy-900">Upstream</h2>
        <p className="mt-1 text-sm text-navy-600">
          {upstream?.note ?? "Tujuan request. Klien tidak bisa memilih destinasinya (anti-SSRF)."}
        </p>
        <div className="mt-4 flex flex-col gap-3 sm:flex-row">
          <input
            className="input flex-1 font-mono text-sm"
            value={upstreamUrl}
            onChange={(e) => setUpstreamUrl(e.target.value)}
            placeholder="http://127.0.0.1:8099"
          />
          <button type="button" className="btn-primary" onClick={saveUpstream} disabled={busy !== null}>
            {busy === "upstream" ? "Menyimpan..." : "Simpan"}
          </button>
        </div>
        {upstream && (
          <p className="mt-2 text-xs text-navy-500">
            Host aktif: <span className="font-mono">{upstream.upstreamHost}</span>
          </p>
        )}
      </div>

      {settings && (
        <div className="card p-6">
          <h2 className="text-lg font-bold text-navy-900">Settings</h2>
          <p className="mt-1 text-sm text-navy-600">{settings.note}</p>

          <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field label="Rate limit burst" hint="1 - 100000">
              <input
                type="number"
                className="input"
                value={settings.rateLimitBurst}
                min={1}
                max={100000}
                onChange={(e) => patch("rateLimitBurst", Number(e.target.value))}
              />
            </Field>
            <Field label="Rate limit window (s)" hint="1 - 3600">
              <input
                type="number"
                className="input"
                value={settings.rateLimitWindowSeconds}
                min={1}
                max={3600}
                onChange={(e) => patch("rateLimitWindowSeconds", Number(e.target.value))}
              />
            </Field>
            <Field label="Max body (bytes)" hint={`≈ ${(settings.maxBodyBytes / (1 << 20)).toFixed(2)} MB`}>
              <input
                type="number"
                className="input"
                value={settings.maxBodyBytes}
                min={1}
                onChange={(e) => patch("maxBodyBytes", Number(e.target.value))}
              />
            </Field>
            <Field label="Max URL (bytes)" hint="1 - 16384">
              <input
                type="number"
                className="input"
                value={settings.maxUrlBytes}
                min={1}
                max={16384}
                onChange={(e) => patch("maxUrlBytes", Number(e.target.value))}
              />
            </Field>
            <Field label="Max header (bytes)" hint="1 - 1048576">
              <input
                type="number"
                className="input"
                value={settings.maxHeaderBytes}
                min={1}
                max={1 << 20}
                onChange={(e) => patch("maxHeaderBytes", Number(e.target.value))}
              />
            </Field>
            <Field label="Evaluation budget (ms)" hint="1 - 5000">
              <input
                type="number"
                className="input"
                value={settings.evaluationBudgetMs}
                min={1}
                max={5000}
                onChange={(e) => patch("evaluationBudgetMs", Number(e.target.value))}
              />
            </Field>
          </div>

          <div className="mt-5 space-y-3">
            <Toggle
              label="Rate limit"
              checked={settings.rateLimitEnabled}
              onChange={(v) => patch("rateLimitEnabled", v)}
            />
            <Toggle
              label="Fail closed (reject when eval budget exceeded)"
              checked={settings.failClosed}
              onChange={(v) => patch("failClosed", v)}
            />
          </div>

          <div className="mt-5">
            <button type="button" className="btn-primary" onClick={saveSettings} disabled={busy !== null}>
              {busy === "settings" ? "Menyimpan..." : "Simpan settings"}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 flex items-center justify-between text-sm font-medium text-navy-800">
        {label}
        {hint && <span className="text-xs font-normal text-navy-400">{hint}</span>}
      </span>
      {children}
    </label>
  );
}

function Toggle({ label, checked, onChange }: { label: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex cursor-pointer items-center gap-3">
      <input
        type="checkbox"
        className="h-4 w-4 rounded border-navy-300 text-navy-900 focus:ring-navy-500"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span className="text-sm text-navy-800">{label}</span>
    </label>
  );
}
