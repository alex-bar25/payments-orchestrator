const PASS = ["content-type", "idempotency-key", "x-request-id", "accept"];

export async function proxyToApi(req: Request, apiPath: string): Promise<Response> {
  const base = (process.env.API_URL ?? "http://127.0.0.1:8080").replace(/\/$/, "");
  const incoming = new URL(req.url);
  const headers = new Headers();
  for (const name of PASS) {
    const value = req.headers.get(name);
    if (value) {
      headers.set(name, value);
    }
  }
  const init: RequestInit = { method: req.method, headers, redirect: "manual" };
  if (req.method !== "GET" && req.method !== "HEAD") {
    init.body = await req.arrayBuffer();
  }
  try {
    const res = await fetch(`${base}${apiPath}${incoming.search}`, {
      ...init,
      cache: "no-store",
    });
    const out = new Headers();
    const type = res.headers.get("content-type");
    if (type) {
      out.set("content-type", type);
    }
    return new Response(res.body, { status: res.status, headers: out });
  } catch {
    return Response.json(
      {
        error: {
          code: "API_UNREACHABLE",
          message: "Go API is not reachable. Start it on :8080 or set API_URL.",
        },
      },
      { status: 502 },
    );
  }
}
