import Link from "next/link";

function GarudaShieldLogo({ compact = false }: { compact?: boolean }) {
  return (
    <Link href="/" className="flex items-center gap-2.5">
      <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-navy-900 text-gold shadow-sm">
        <svg viewBox="0 0 24 24" fill="none" className="h-5 w-5" aria-hidden="true">
          <path
            d="M12 2L4 5v6c0 5 3.4 9.4 8 11 4.6-1.6 8-6 8-11V5l-8-3z"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinejoin="round"
          />
          <path
            d="M9 12l2 2 4-4"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </span>
      {!compact && (
        <span className="text-lg font-bold tracking-tight text-navy-900">
          Garuda<span className="text-gold">Shield</span>
        </span>
      )}
    </Link>
  );
}

export default function Navbar() {
  return (
    <header className="sticky top-0 z-50 border-b border-navy-100 bg-white/85 backdrop-blur">
      <div className="container-shell flex h-16 items-center justify-between">
        <GarudaShieldLogo />
        <nav className="hidden items-center gap-7 text-sm font-medium text-navy-700 md:flex">
          <a href="/#fitur" className="transition hover:text-navy-900">Fitur</a>
          <a href="/#status" className="transition hover:text-navy-900">Status</a>
          <Link href="/docs" className="transition hover:text-navy-900">Dokumentasi</Link>
          <a href="#" className="transition hover:text-navy-900">AI Garuda</a>
        </nav>
        <div className="flex items-center gap-3">
          <Link
            href="/login"
            className="rounded-lg border border-navy-200 px-4 py-2 text-sm font-semibold text-navy-900 transition hover:border-navy-300 hover:bg-navy-50"
          >
            Masuk
          </Link>
        </div>
      </div>
    </header>
  );
}
