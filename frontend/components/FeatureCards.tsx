const FEATURES = [
  {
    title: "L4 + L7",
    description: "Mitigasi di transport & aplikasi.",
    icon: (
      <svg viewBox="0 0 24 24" fill="none" className="h-6 w-6" aria-hidden="true">
        <path d="M12 2L4 5v6c0 5 3.4 9.4 8 11 4.6-1.6 8-6 8-11V5l-8-3z" stroke="currentColor" strokeWidth="1.8" />
        <path d="M8 12h8M12 8v8" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
      </svg>
    ),
  },
  {
    title: "50 ms",
    description: "Configurable demo evaluation budget; never claim this is a guaranteed latency.",
    icon: (
      <svg viewBox="0 0 24 24" fill="none" className="h-6 w-6" aria-hidden="true">
        <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="1.8" />
        <path d="M12 7v5l3 3" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
      </svg>
    ),
  },
  {
    title: "Fail-closed",
    description: "Conservative behavior when the demo evaluation budget is exceeded.",
    icon: (
      <svg viewBox="0 0 24 24" fill="none" className="h-6 w-6" aria-hidden="true">
        <path d="M12 9v4m0 4h.01M10.3 3.9L1.8 18a2 2 0 001.7 3h17a2 2 0 001.7-3L13.7 3.9a2 2 0 00-3.4 0z" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    ),
  },
  {
    title: "Go Engine",
    description: "Single-process demonstration engine written in Go standard library.",
    icon: (
      <svg viewBox="0 0 24 24" fill="none" className="h-6 w-6" aria-hidden="true">
        <path d="M4 17l6-6-6-6M12 19h8" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    ),
  },
];

export default function FeatureCards() {
  return (
    <section id="fitur" className="container-shell py-16">
      <div className="mb-8">
        <p className="text-sm font-semibold uppercase tracking-wider text-gold-dark">Fitur Demo</p>
        <h2 className="mt-2 text-2xl font-bold text-navy-900">Perlindungan berlapis untuk demonstrasi</h2>
      </div>
      <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {FEATURES.map((f) => (
          <div
            key={f.title}
            className="rounded-2xl bg-navy-900 p-6 shadow-md transition hover:shadow-lg"
          >
            <div className="mb-4 inline-flex rounded-xl bg-gold/15 p-2.5 text-gold-light">
              {f.icon}
            </div>
            <h3 className="text-lg font-bold text-gold-light">{f.title}</h3>
            <p className="mt-2 text-sm leading-relaxed text-navy-100">{f.description}</p>
          </div>
        ))}
      </div>
    </section>
  );
}
