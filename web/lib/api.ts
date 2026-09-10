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

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  idempotencyKey?: string,
): Promise<T> {
  const headers = new Headers();
  if (body !== undefined) {
    headers.set("content-type", "application/json");
  }
  if (idempotencyKey) {
    headers.set("Idempotency-Key", idempotencyKey);
  }
  if (method !== "GET") {
    headers.set("X-Request-Id", crypto.randomUUID());
  }
  const res = await fetch(path, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  let data: unknown = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      throw new ApiError(res.status, "INVALID_JSON", text.slice(0, 200));
    }
  }
  if (!res.ok) {
    const err = (data as { error?: { code?: string; message?: string } } | null)?.error;
    throw new ApiError(res.status, err?.code ?? "HTTP", err?.message ?? res.statusText);
  }
  return data as T;
}

export const api = {
  health: () => request<{ status: string }>("GET", "/health"),
  create: (amount: number, currency: string, key: string) =>
    request<Payment>("POST", "/api/v1/payments", { amount, currency }, key),
  get: (id: string) => request<Payment>("GET", `/api/v1/payments/${id}`),
  events: (id: string) => request<PaymentEvent[]>("GET", `/api/v1/payments/${id}/events`),
  authorize: (id: string, key: string) =>
    request<Payment>("POST", `/api/v1/payments/${id}/authorize`, undefined, key),
  capture: (id: string, key: string) =>
    request<Payment>("POST", `/api/v1/payments/${id}/capture`, undefined, key),
  refund: (id: string, key: string) =>
    request<Payment>("POST", `/api/v1/payments/${id}/refund`, undefined, key),
  cancel: (id: string, key: string) =>
    request<Payment>("POST", `/api/v1/payments/${id}/cancel`, undefined, key),
  discrepancies: () => request<Discrepancy[]>("GET", "/api/v1/discrepancies"),
  inbound: (id: string, type: string, paymentId: string) =>
    request<Payment>("POST", "/api/v1/webhooks/mock", {
      id,
      type,
      payment_id: paymentId,
    }),
};
