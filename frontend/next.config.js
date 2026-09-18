const path = require("path");

/** @type {import("next").NextConfig} */
const nextConfig = {
  turbopack: {
    root: path.join(__dirname),
  },
  async rewrites() {
    if (process.env.VERCEL) return [];
    return [
      {
        source: "/svc/api/:path*",
        destination: "http://127.0.0.1:8090/svc/api/:path*",
      },
    ];
  },
};

module.exports = nextConfig;
