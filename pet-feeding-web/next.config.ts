import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Minimal production image for Docker / self-hosting (next start via node server.js).
  output: "standalone",
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
