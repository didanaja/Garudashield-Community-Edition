import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";

export default function DocsPage() {
  return (
    <main className="flex min-h-screen flex-col">
      <Navbar />
      <div className="container-shell flex-1 py-12">
        <div className="prose prose-navy max-w-none">
          <h1 className="text-3xl font-bold text-navy-900">Dokumentasi GarudaShield Demo Edition</h1>
          <p className="mt-4 text-navy-600">
            Dokumentasi ini menjelaskan arsitektur, model keamanan, dan batasan demonstrasi.
          </p>

          <h2 className="mt-8 text-xl font-bold text-navy-900">Arsitektur</h2>
          <p className="mt-2 text-navy-600">
            Engine terdiri dari reverse proxy (net/http/httputil), middleware pipeline
            (request ID, logging, security headers, CORS, rate limiting, evaluasi L7),
            dan control plane REST API di bawah <code>/api/v1</code>. Redis-compatible
            storage interface menyediakan in-memory implementation untuk demonstrasi lokal.
          </p>

          <h2 className="mt-8 text-xl font-bold text-navy-900">Model Keamanan</h2>
          <ul className="mt-2 list-disc pl-5 text-navy-600">
            <li>Method allowlisting</li>
            <li>Batasan ukuran URL, header, dan body</li>
            <li>Rate limiting per-client (token bucket, single-process)</li>
            <li>Deteksi pola sederhana (path traversal, SQLi, XSS, scanner UA)</li>
            <li>Fail-closed evaluation budget</li>
            <li>SSRF prevention pada konfigurasi upstream</li>
          </ul>

          <h2 className="mt-8 text-xl font-bold text-navy-900">Batasan</h2>
          <p className="mt-2 text-navy-600">
            Demo ini <strong>bukan</strong> WAF produksi. Pattern matcher tidak mengklaim
            OWASP Top 10 coverage. Rate limiter single-process, bukan distributed.
            Tidak ada TCP/UDP scrubbing network atau DDoS mitigation infra.
          </p>

          <h2 className="mt-8 text-xl font-bold text-navy-900">Menjalankan Demo</h2>
          <pre className="mt-2 overflow-x-auto rounded-xl bg-navy-900 p-4 text-sm text-navy-100">
{`# Terminal 1: test upstream
cd backend && go run ./cmd/server -hash-password "your-password"
# lalu set GARUDA_ADMIN_PASSWORD_HASH atau GARUDA_ADMIN_PASSWORD

# Terminal 2: backend
cd backend && go run ./cmd/server

# Terminal 3: frontend
cd frontend && npm install && npm run dev

# Buka http://localhost:3000`}
          </pre>
        </div>
      </div>
      <Footer />
    </main>
  );
}
