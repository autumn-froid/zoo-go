import type { NextConfig } from "next";

// Vercel provides its own output; standalone is for Docker / ECS self-hosting.
const isVercel = process.env.VERCEL === "1";

// Server-only: where Next rewrites /v1/* (avoids browser mixed-content HTTPS→HTTP).
// Example for ECS before ICP: http://1.92.99.17  (Caddy :80) or http://1.92.99.17:8080
const apiProxyOrigin = process.env.API_PROXY_ORIGIN?.replace(/\/$/, "") || "";

const nextConfig: NextConfig = {
  ...(isVercel ? {} : { output: "standalone" as const }),
  // Dev browsers hit 127.0.0.1; without this, Next blocks /_next/hmr and client updates.
  allowedDevOrigins: ["127.0.0.1", "localhost"],
  experimental: {
    agentFeedback: true,
  },
  cacheComponents: true,
  partialPrefetching: true,
  reactCompiler: true,
  images: {
    remotePatterns: [
      {
        protocol: "https",
        hostname: "images.unsplash.com",
      },
    ],
  },
  // Self-hosted nginx/Caddy: disable proxy buffering so App Router streaming works.
  async headers() {
    return [
      {
        source: "/:path*{/}?",
        headers: [{ key: "X-Accel-Buffering", value: "no" }],
      },
    ];
  },
  async rewrites() {
    if (!apiProxyOrigin) {
      return [];
    }
    return [
      {
        source: "/v1/:path*",
        destination: `${apiProxyOrigin}/v1/:path*`,
      },
    ];
  },
  turbopack: {
    root: __dirname,
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
};

export default nextConfig;
