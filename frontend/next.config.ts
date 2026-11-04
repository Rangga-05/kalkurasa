import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  async rewrites() {
    const ginApiUrl = process.env.GIN_API_URL || "http://localhost:8080";
    return [
      {
        source: "/api/:path*",
        destination: `${ginApiUrl}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;

