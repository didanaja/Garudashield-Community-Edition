import type {
  LogsResponse,
  MetricsResponse,
  RulesResponse,
  SettingsView,
  StatusResponse,
  UpstreamView,
} from "@/types";

// Default to a relative base so requests go through the Next.js origin and the
// session cookie is treated as same-origin. Override with NEXT_PUBLIC_API_BASE_URL
// when the API is served from a different origin.
const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "";

// The protection engine is contacted directly (not through the Next.js rewrite)
// so the raw request target is preserved and the engine sees exactly what a real
// client would send. The engine allows this origin via a restrictive CORS policy.
const ENGINE_BASE = process.env.NEXT_PUBLIC_ENGINE_URL ?? "http://127.0.0.1:8080";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: "include",
    headers: init?.body ? { "Content-Type": "application/json" } : undefined,
    ...init,
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new ApiError(res.status, text);
  }
  return (await res.json()) as T;
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, body: string) {
    super(body || `HTTP ${status}`);
    this.status = status;
  }
}

export type ProxyResult = {
  status: number;
  body: unknown;
};

export const api = {
  status: () => request<StatusResponse>("/api/v1/status"),
  metrics: () => request<MetricsResponse>("/api/v1/metrics"),
  rules: () => request<RulesResponse>("/api/v1/rules"),
  logs: (limit = 50) => request<LogsResponse>(`/api/v1/logs?limit=${limit}`),

  // Send real traffic directly to the protection engine. The raw request target
  // is preserved (unlike a same-origin rewrite) so the engine's rule verdicts
  // match what a real client would trigger.
  sendProxy: async (method: string, path: string): Promise<ProxyResult> => {
    const res = await fetch(`${ENGINE_BASE}${path}`, { method, cache: "no-store" });
    const text = await res.text().catch(() => "");
    let body: unknown = text;
    try {
      body = text ? JSON.parse(text) : null;
    } catch {
      body = text;
    }
    return { status: res.status, body };
  },

  session: () => request<{ authenticated: boolean; user?: string; csrf?: string }>("/api/v1/auth/session"),
  csrf: () => request<{ csrf: string }>("/api/v1/auth/csrf"),

  login: (username: string, password: string) =>
    request<{ authenticated: boolean; user: string; csrf: string }>("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),
  logout: () =>
    fetch(`${API_BASE}/api/v1/auth/logout`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({}),
    }).then(() => undefined),

  settings: () => request<SettingsView>("/api/v1/admin/settings"),
  saveSettings: (body: Record<string, unknown>, csrf: string) =>
    request<SettingsView>("/api/v1/admin/settings", {
      method: "PUT",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: JSON.stringify(body),
    }),

  upstream: () => request<UpstreamView>("/api/v1/admin/upstream"),
  setUpstream: (url: string, csrf: string) =>
    request<UpstreamView>("/api/v1/admin/upstream", {
      method: "PUT",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: JSON.stringify({ url }),
    }),
};

export { API_BASE, ENGINE_BASE };
