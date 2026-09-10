import { proxyToApi } from "@/lib/backend";

export const dynamic = "force-dynamic";

async function handle(req: Request, ctx: { params: Promise<{ path: string[] }> }) {
  const { path } = await ctx.params;
  return proxyToApi(req, `/api/v1/${path.join("/")}`);
}

export const GET = handle;
export const POST = handle;
