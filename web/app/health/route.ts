import { proxyToApi } from "@/lib/backend";

export const dynamic = "force-dynamic";

export function GET(req: Request) {
  return proxyToApi(req, "/health");
}
