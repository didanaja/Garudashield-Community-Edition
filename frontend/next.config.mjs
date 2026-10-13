/** @type {import('next').NextConfig} */

// Proxy API and health probes through the Next.js origin so the browser treats
// the backend as same-origin. This makes session cookies (SameSite=Strict)
// work reliably and removes any cross-site concerns during local development.
const API_TARGET = process.env.API_PROXY_TARGET ?? "http://127.0.0.1:8080";

const nextConfig = {
  reactStrictMode: true,
  async rewrites() {
    return [
      { source: "/api/:path*", destination: `${API_TARGET}/api/:path*` },
      { source: "/healthz", destination: `${API_TARGET}/healthz` },
      { source: "/readyz", destination: `${API_TARGET}/readyz` },
    ];
  },
};

export default nextConfig;
