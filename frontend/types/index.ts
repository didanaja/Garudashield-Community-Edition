export type ComponentHealth = {
  name: string;
  status: "healthy" | "degraded" | "down";
  detail: string;
  latencyMs: number;
};

export type StatusResponse = {
  edition: string;
  overall: "healthy" | "degraded" | "down";
  engine: {
    name: string;
    version: string;
    startedAt: string;
    uptimeSeconds: number;
    upstreamHost: string;
  };
  components: ComponentHealth[];
  checkedAt: string;
};

export type MetricsSnapshot = {
  requestsTotal: number;
  requestsEvaluated: number;
  allowed: number;
  blocked: number;
  rateLimitEvents: number;
  evaluationTimeouts: number;
  upstreamErrors: number;
  uptimeSeconds: number;
  startedAt: string;
};

export type MetricsResponse = {
  edition: string;
  actual: boolean;
  note: string;
  metrics: MetricsSnapshot;
  recordedAt: string;
};

export type RuleInfo = {
  id: string;
  name: string;
  description: string;
  category: string;
  action: string;
  enabled: boolean;
};

export type RulesResponse = {
  edition: string;
  disclaimer: string;
  rules: RuleInfo[];
};

export type SecurityEvent = {
  timestamp: string;
  requestId: string;
  clientIp: string;
  method: string;
  path: string;
  verdict: "allowed" | "blocked" | "rate_limited" | "evaluation_timeout" | "error";
  status: number;
  ruleId?: string;
  reason?: string;
  durationMs: number;
  upstream?: string;
};

export type LogsResponse = {
  edition: string;
  count: number;
  events: SecurityEvent[];
};

export type SessionResponse = {
  authenticated: boolean;
  user?: string;
  csrf?: string;
  expiresAt?: string;
};

export type SettingsView = {
  rateLimitEnabled: boolean;
  rateLimitBurst: number;
  rateLimitWindowSeconds: number;
  maxBodyBytes: number;
  maxUrlBytes: number;
  maxHeaderBytes: number;
  evaluationBudgetMs: number;
  failClosed: boolean;
  note: string;
};

export type UpstreamView = {
  upstreamUrl: string;
  upstreamHost: string;
  note: string;
};
