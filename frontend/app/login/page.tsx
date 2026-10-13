"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import { api, ApiError } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      await api.login(username, password);
      router.push("/dashboard");
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.status === 429 ? "Terlalu banyak percobaan. Coba lagi nanti." : "Username atau password salah.");
      } else {
        setError("Tidak dapat terhubung ke server.");
      }
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="flex min-h-screen flex-col">
      <Navbar />
      <div className="container-shell flex flex-1 items-center justify-center py-16">
        <div className="card w-full max-w-md p-8">
          <h1 className="text-2xl font-bold text-navy-900">Masuk ke Portal</h1>
          <p className="mt-2 text-sm text-navy-600">
            Akses dashboard administrasi GarudaShield Demo Edition.
          </p>

          {error && (
            <div className="mt-4 rounded-xl border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700" role="alert">
              {error}
            </div>
          )}

          <form onSubmit={onSubmit} className="mt-6 space-y-4">
            <div>
              <label htmlFor="username" className="mb-1.5 block text-sm font-medium text-navy-800">
                Username
              </label>
              <input
                id="username"
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                autoComplete="username"
                className="w-full rounded-xl border border-navy-200 px-4 py-2.5 text-sm text-navy-900 focus:border-gold focus:outline-none focus:ring-2 focus:ring-gold/30"
              />
            </div>
            <div>
              <label htmlFor="password" className="mb-1.5 block text-sm font-medium text-navy-800">
                Password
              </label>
              <input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                autoComplete="current-password"
                className="w-full rounded-xl border border-navy-200 px-4 py-2.5 text-sm text-navy-900 focus:border-gold focus:outline-none focus:ring-2 focus:ring-gold/30"
              />
            </div>
            <button type="submit" disabled={loading} className="btn-primary w-full disabled:opacity-60">
              {loading ? "Memproses..." : "Masuk"}
            </button>
          </form>
        </div>
      </div>
      <Footer />
    </main>
  );
}
