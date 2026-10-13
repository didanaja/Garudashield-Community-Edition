"use client";

export type TrafficSample = {
  label: string;
  allowed: number;
  blocked: number;
  errors: number;
};

export default function TrafficChart({ samples }: { samples: TrafficSample[] }) {
  const max = Math.max(1, ...samples.map((s) => s.allowed + s.blocked + s.errors));

  return (
    <div className="card p-6">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-bold text-navy-900">Traffic (interval)</h2>
        <div className="flex items-center gap-3 text-xs text-navy-600">
          <span className="flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-sm bg-emerald-500" /> Allowed
          </span>
          <span className="flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-sm bg-rose-500" /> Blocked
          </span>
          <span className="flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-sm bg-navy-400" /> Errors
          </span>
        </div>
      </div>

      {samples.length === 0 ? (
        <div className="flex h-40 items-center justify-center rounded-xl border border-dashed border-navy-200 text-sm text-navy-500">
          Belum ada traffic. Kirim request lewat Traffic Tester atau ke engine di :8080.
        </div>
      ) : (
        <div className="flex h-40 items-end gap-1.5">
          {samples.map((s, i) => {
            const total = s.allowed + s.blocked + s.errors;
            return (
              <div key={i} className="group flex flex-1 flex-col items-center justify-end gap-1">
                <div className="relative flex w-full flex-col justify-end" style={{ height: "8rem" }}>
                  <div
                    className="w-full rounded-t bg-emerald-500/90 transition-all"
                    style={{ height: `${(s.allowed / max) * 100}%` }}
                    title={`Allowed: ${s.allowed}`}
                  />
                  <div
                    className="w-full bg-rose-500/90 transition-all"
                    style={{ height: `${(s.blocked / max) * 100}%` }}
                    title={`Blocked: ${s.blocked}`}
                  />
                  <div
                    className="w-full bg-navy-400/90 transition-all"
                    style={{ height: `${(s.errors / max) * 100}%` }}
                    title={`Errors: ${s.errors}`}
                  />
                  {total === 0 && <div className="h-1 w-full rounded bg-navy-100" />}
                </div>
                <span className="text-[10px] text-navy-400 opacity-0 transition group-hover:opacity-100">
                  {s.label}
                </span>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
