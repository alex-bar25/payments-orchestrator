export type Status =
  | "requires_payment"
  | "processing"
  | "authorized"
  | "captured"
  | "refunded"
  | "failed"
  | "cancelled";

export type Payment = {
  id: string;
  merchant_id: string;
  amount: number;
  currency: string;
  status: Status;
  provider_payment_id?: string;
  created_at: string;
  updated_at: string;
};

export type PaymentEvent = {
  id: string;
  payment_id: string;
  type: string;
  metadata: string;
  request_id?: string;
  created_at: string;
};

export type Discrepancy = {
  id: string;
  payment_id: string;
  internal_status: string;
  provider_status: string;
  created_at: string;
};

export type HttpExchange = {
  method: string;
  path: string;
  status: number;
  requestHeaders: Record<string, string>;
  requestBody: unknown;
  responseBody: unknown;
};

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

type RequestOpts = {
  body?: unknown;
  idempotencyKey?: string;
  silent?: boolean;
};

let httpListener: ((exchange: HttpExchange) => void) | null = null;

export function onHttp(listener: (exchange: HttpExchange) => void) {
  httpListener = listener;
  return () => {
    if (httpListener === listener) {
      httpListener = null;
    }
  };
}

async function request<T>(method: string, path: string, opts: RequestOpts = {}): Promise<T> {
  const headers = new Headers();
  const requestHeaders: Record<string, string> = {};
  if (opts.body !== undefined) {
    headers.set("content-type", "application/json");
    requestHeaders["Content-Type"] = "application/json";
  }
  if (opts.idempotencyKey) {
    headers.set("Idempotency-Key", opts.idempotencyKey);
    requestHeaders["Idempotency-Key"] = opts.idempotencyKey;
  }
  if (method !== "GET") {
    const requestId = crypto.randomUUID();
    headers.set("X-Request-Id", requestId);
    requestHeaders["X-Request-Id"] = requestId;
  }
  const res = await fetch(path, {
    method,
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  });
  const text = await res.text();
  let data: unknown = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = text;
      if (!opts.silent) {
        httpListener?.({
          method,
          path,
          status: res.status,
          requestHeaders,
          requestBody: opts.body ?? null,
          responseBody: text.slice(0, 2000),
        });
      }
      throw new ApiError(res.status, "INVALID_JSON", text.slice(0, 200));
    }
  }
  if (!opts.silent) {
    httpListener?.({
      method,
      path,
      status: res.status,
      requestHeaders,
      requestBody: opts.body ?? null,
      responseBody: data,
    });
  }
  if (!res.ok) {
    const err = (data as { error?: { code?: string; message?: string } } | null)?.error;
    throw new ApiError(res.status, err?.code ?? "HTTP", err?.message ?? res.statusText);
  }
  return data as T;
}

export const api = {
  health: () => request<{ status: string }>("GET", "/health", { silent: true }),
  create: (amount: number, currency: string, key: string) =>
    request<Payment>("POST", "/api/v1/payments", {
      body: { amount, currency },
      idempotencyKey: key,
    }),
  get: (id: string, silent = false) =>
    request<Payment>("GET", `/api/v1/payments/${id}`, { silent }),
  events: (id: string, silent = false) =>
    request<PaymentEvent[]>("GET", `/api/v1/payments/${id}/events`, { silent }),
  authorize: (id: string, key: string) =>
    request<Payment>("POST", `/api/v1/payments/${id}/authorize`, { idempotencyKey: key }),
  capture: (id: string, key: string) =>
    request<Payment>("POST", `/api/v1/payments/${id}/capture`, { idempotencyKey: key }),
  refund: (id: string, key: string) =>
    request<Payment>("POST", `/api/v1/payments/${id}/refund`, { idempotencyKey: key }),
  cancel: (id: string, key: string) =>
    request<Payment>("POST", `/api/v1/payments/${id}/cancel`, { idempotencyKey: key }),
  discrepancies: (silent = false) =>
    request<Discrepancy[]>("GET", "/api/v1/discrepancies", { silent }),
  inbound: (id: string, type: string, paymentId: string) =>
    request<Payment>("POST", "/api/v1/webhooks/mock", {
      body: { id, type, payment_id: paymentId },
    }),
};
