export async function GET() {
  return Response.json({
    service: "frontend",
    framework: "nextjs",
    message: "AegisCortex Next.js service",
  });
}
