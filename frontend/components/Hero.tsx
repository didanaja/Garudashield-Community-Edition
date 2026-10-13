import StatusCard from "./StatusCard";

export default function Hero() {
  return (
    <section className="container-shell grid gap-10 py-16 lg:grid-cols-2 lg:items-center lg:py-24">
      <div>
        <p className="mb-4 inline-flex items-center gap-2 rounded-full border border-gold/30 bg-gold/10 px-3 py-1 text-xs font-semibold uppercase tracking-wider text-gold-dark">
          Experimental Protection Engine
        </p>
        <h1 className="text-4xl font-extrabold leading-tight tracking-tight text-navy-900 sm:text-5xl">
          Tetap online, bahkan saat diserang.
        </h1>
        <p className="mt-5 max-w-xl text-lg leading-relaxed text-navy-600">
          Demonstrasi edukasi multi-layer reverse proxy dan web protection engine.
          GarudaShield Demo Edition mengevaluasi setiap request dengan rule engine
          yang dapat dijelaskan, rate limiting, dan fail-closed budget — tanpa
          mengklaim perlindungan produksi penuh.
        </p>
        <div className="mt-8 flex flex-wrap items-center gap-4">
          <a
            href="https://wa.me/6281234567890?text=Halo%20GarudaShield"
            target="_blank"
            rel="noopener noreferrer"
            className="btn-primary"
          >
            Order via WhatsApp
          </a>
          <a href="#fitur" className="btn-secondary">
            Lihat paket &amp; fitur
          </a>
        </div>
        <p className="mt-4 text-sm text-navy-500">
          Sudah punya akun?{" "}
          <a href="/login" className="font-semibold text-gold-dark hover:underline">
            Masuk ke portal
          </a>
        </p>
      </div>
      <div className="flex justify-center lg:justify-end">
        <StatusCard />
      </div>
    </section>
  );
}
