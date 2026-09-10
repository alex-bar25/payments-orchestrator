"use client";

import { useCallback, useEffect, useState, useSyncExternalStore } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  api,
  ApiError,
  onHttp,
  type Discrepancy,
  type HttpExchange,
  type Payment,
  type PaymentEvent,
} from "@/lib/api";
import {
  ACTION_LABEL,
  actionRoute,
  actionsFor,
  formatMinor,
  formatTime,
  stampClass,
  ticketRule,
  type Action,
} from "@/lib/payment-ui";
import { cn } from "@/lib/utils";
import {
  getServerSessionSnapshot,
  getSessionSnapshot,
  rememberPayment,
  subscribeSession,
  writeSession,
  type LastCreate,
} from "@/lib/session";

const PRESETS = [
  { amount: 4999, hint: "auth, capture, refund" },
  { amount: 1, hint: "authorize declined" },
  { amount: 2, hint: "capture declined" },
  { amount: 3, hint: "refund declined" },
] as const;

const INBOUND_TYPES = [
  "payment.authorized",
  "payment.failed",
  "payment.captured",
  "payment.refunded",
  "payment.cancelled",
] as const;

function pretty(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }
  return JSON.stringify(value, null, 2);
}

export function Desk() {
  const session = useSyncExternalStore(
    subscribeSession,
    getSessionSnapshot,
    getServerSessionSnapshot,
  );
  const { currentId, ids, lastCreate } = session;
  const [apiUp, setApiUp] = useState<boolean | null>(null);
  const [payment, setPayment] = useState<Payment | null>(null);
  const [events, setEvents] = useState<PaymentEvent[]>([]);
  const [discrepancies, setDiscrepancies] = useState<Discrepancy[]>([]);
  const [amount, setAmount] = useState("4999");
  const [currency, setCurrency] = useState("EUR");
  const [lookup, setLookup] = useState("");
  const [inboundType, setInboundType] = useState<string>("payment.authorized");
  const [lastInboundId, setLastInboundId] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [exchange, setExchange] = useState<HttpExchange | null>(null);

  const remember = useCallback(
    (id: string, create?: LastCreate) => {
      writeSession(rememberPayment(session, id, create));
    },
    [session],
  );

  const loadPayment = useCallback(async (id: string, silent = false) => {
    const [p, ev] = await Promise.all([api.get(id, silent), api.events(id, true)]);
    setPayment(p);
    setEvents(ev);
    return p;
  }, []);

  useEffect(() => onHttp(setExchange), []);

  useEffect(() => {
    let cancelled = false;
    const ping = async () => {
      try {
        await api.health();
        if (!cancelled) setApiUp(true);
      } catch {
        if (!cancelled) setApiUp(false);
      }
    };
    ping();
    const t = setInterval(ping, 15000);
    return () => {
      cancelled = true;
      clearInterval(t);
    };
  }, []);

  useEffect(() => {
    if (!currentId) return;
    let cancelled = false;
    (async () => {
      try {
        const [p, ev] = await Promise.all([api.get(currentId, true), api.events(currentId, true)]);
        if (cancelled) return;
        setPayment(p);
        setEvents(ev);
      } catch (err) {
        if (cancelled) return;
        if (err instanceof ApiError && err.code === "PAYMENT_NOT_FOUND") {
          setPayment(null);
          setEvents([]);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [currentId]);

  useEffect(() => {
    let cancelled = false;
    const tick = async () => {
      try {
        const rows = await api.discrepancies(true);
        if (!cancelled) setDiscrepancies(rows);
      } catch {
        if (!cancelled) setDiscrepancies([]);
      }
    };
    tick();
    const t = setInterval(tick, 5000);
    return () => {
      cancelled = true;
      clearInterval(t);
    };
  }, []);

  useEffect(() => {
    if (!payment || payment.status !== "processing") return;
    const t = setInterval(() => {
      void loadPayment(payment.id, true).catch(() => undefined);
    }, 2000);
    return () => clearInterval(t);
  }, [payment, loadPayment]);

  async function run(label: string, fn: () => Promise<void>) {
    setBusy(label);
    try {
      await fn();
    } catch (err) {
      if (!(err instanceof ApiError)) {
        setExchange({
          method: "CLIENT",
          path: label,
          status: 0,
          requestHeaders: {},
          requestBody: null,
          responseBody: { error: { code: "CLIENT", message: "request failed" } },
        });
      }
    } finally {
      setBusy(null);
    }
  }

  function createWith(
    nextAmount: number,
    nextCurrency: string,
    opts?: { key?: string; fillForm?: boolean },
  ) {
    const key = opts?.key ?? crypto.randomUUID();
    if (opts?.fillForm !== false) {
      setAmount(String(nextAmount));
      setCurrency(nextCurrency);
    }
    return run(`create-${nextAmount}`, async () => {
      const p = await api.create(nextAmount, nextCurrency, key);
      remember(p.id, { key, amount: nextAmount, currency: nextCurrency });
      setPayment(p);
      setEvents(await api.events(p.id, true));
    });
  }

  function act(action: Action) {
    if (!payment) return;
    return run(action, async () => {
      const p = await api[action](payment.id, crypto.randomUUID());
      setPayment(p);
      setEvents(await api.events(p.id, true));
    });
  }

  const legal = payment ? actionsFor(payment.status) : [];

  return (
    <div className="grid min-h-dvh grid-cols-1 lg:grid-cols-[18rem_minmax(0,1fr)_22rem]">
      <aside className="flex flex-col gap-6 border-b border-border px-4 py-4 lg:border-r lg:border-b-0">
        <div>
          <p className="text-[15px] font-semibold tracking-tight">Payment Orchestrator</p>
          <p className="mt-1 font-mono text-[12px] text-[var(--muted-ink)]">
            merch_demo
            {apiUp === null ? "" : apiUp ? "  API ok" : "  API down"}
          </p>
        </div>

        <section className="flex flex-col gap-2">
          <h2 className="text-sm font-semibold">Create</h2>
          <p className="text-[12px] leading-5 text-[var(--muted-ink)]">
            Amounts are integer minor units. Mock PSP: 1 fails authorize, 2 fails capture, 3 fails
            refund.
          </p>
          <ul className="flex flex-col border border-border bg-card">
            {PRESETS.map((preset) => (
              <li key={preset.amount} className="border-b border-border last:border-b-0">
                <button
                  type="button"
                  disabled={busy !== null}
                  onClick={() => void createWith(preset.amount, currency || "EUR")}
                  className="flex w-full items-baseline justify-between gap-3 px-3 py-2 text-left hover:bg-muted disabled:opacity-50"
                >
                  <span className="font-mono text-sm">{preset.amount}</span>
                  <span className="text-[12px] text-[var(--muted-ink)]">{preset.hint}</span>
                </button>
              </li>
            ))}
          </ul>
          <form
            className="flex flex-col gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              void createWith(Number(amount), currency.trim().toUpperCase() || "EUR");
            }}
          >
            <div className="grid grid-cols-[1fr_4.5rem] gap-2">
              <div className="flex flex-col gap-1">
                <Label htmlFor="amount">Amount</Label>
                <Input
                  id="amount"
                  inputMode="numeric"
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1">
                <Label htmlFor="currency">ISO</Label>
                <Input
                  id="currency"
                  value={currency}
                  onChange={(e) => setCurrency(e.target.value.toUpperCase())}
                  maxLength={3}
                />
              </div>
            </div>
            <Button type="submit" disabled={busy !== null}>
              Create
            </Button>
          </form>
        </section>

        {lastCreate ? (
          <section className="flex flex-col gap-2">
            <h2 className="text-sm font-semibold">Idempotency-Key</h2>
            <p className="break-all font-mono text-[11px] text-[var(--muted-ink)]">{lastCreate.key}</p>
            <Button
              type="button"
              variant="outline"
              disabled={busy !== null}
              onClick={() =>
                void createWith(lastCreate.amount, lastCreate.currency, {
                  key: lastCreate.key,
                  fillForm: false,
                })
              }
            >
              Same body
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={busy !== null}
              onClick={() =>
                void createWith(lastCreate.amount + 1, lastCreate.currency, {
                  key: lastCreate.key,
                  fillForm: false,
                })
              }
            >
              Different body
            </Button>
          </section>
        ) : null}

        <form
          className="flex flex-col gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            const id = lookup.trim();
            if (!id) return;
            void run("lookup", async () => {
              await loadPayment(id);
              remember(id);
            });
          }}
        >
          <Label htmlFor="lookup">GET by id</Label>
          <div className="flex gap-2">
            <Input
              id="lookup"
              value={lookup}
              onChange={(e) => setLookup(e.target.value)}
              placeholder="pay_"
              className="font-mono"
            />
            <Button type="submit" variant="outline" disabled={busy !== null}>
              Load
            </Button>
          </div>
        </form>

        {ids.length > 0 ? (
          <section className="flex flex-col gap-1">
            <h2 className="text-sm font-semibold">This session</h2>
            {ids.map((id) => (
              <button
                key={id}
                type="button"
                onClick={() => writeSession({ ...session, currentId: id })}
                className={cn(
                  "truncate px-2 py-1 text-left font-mono text-[11px] hover:bg-muted",
                  id === currentId && "bg-card",
                )}
              >
                {id}
              </button>
            ))}
          </section>
        ) : null}
      </aside>

      <main className="flex min-w-0 flex-col gap-8 border-b border-border px-4 py-4 lg:border-b-0 lg:px-6">
        {payment ? (
          <section className={cn("border border-border border-l-4 bg-card p-4", ticketRule(payment.status))}>
            <div className="flex flex-wrap items-start justify-between gap-3">
              <p className="font-mono text-[12px] break-all text-[var(--muted-ink)]">{payment.id}</p>
              <span className={cn("border px-2 py-0.5 font-mono text-[12px]", stampClass(payment.status))}>
                {payment.status}
              </span>
            </div>
            <p className="mt-4 font-mono text-[clamp(2rem,5vw,3.4rem)] leading-none tracking-tight">
              {formatMinor(payment.amount, payment.currency)}
            </p>
            <p className="mt-2 font-mono text-[12px] text-[var(--muted-ink)]">
              {payment.amount} {payment.currency}
              {payment.provider_payment_id ? `  ${payment.provider_payment_id}` : ""}
            </p>
            <div className="mt-5 flex flex-col gap-3">
              {legal.length > 0 ? (
                <div className="flex flex-wrap gap-2">
                  {legal.map((action) => (
                    <Button
                      key={action}
                      type="button"
                      variant={action === "cancel" || action === "refund" ? "outline" : "default"}
                      disabled={busy !== null}
                      onClick={() => void act(action)}
                    >
                      {ACTION_LABEL[action]}
                    </Button>
                  ))}
                </div>
              ) : (
                <p className="text-[13px] text-[var(--muted-ink)]">Terminal status. Create another payment to continue.</p>
              )}
              {legal.map((action) => (
                <p key={action} className="font-mono text-[11px] text-[var(--muted-ink)]">
                  {actionRoute(action)}
                </p>
              ))}
            </div>
          </section>
        ) : (
          <section className="border border-dashed border-border px-4 py-8">
            <p className="text-base font-semibold">No payment loaded</p>
            <p className="mt-2 max-w-md text-[13px] leading-6 text-[var(--muted-ink)]">
              Create 4999, then Authorize, Capture, Refund. Or create 1, 2, or 3 for a decline at
              that step.
            </p>
          </section>
        )}

        <section className="flex flex-col gap-2">
          <h2 className="text-sm font-semibold">GET /payments/:id/events</h2>
          {events.length === 0 ? (
            <p className="text-[13px] text-[var(--muted-ink)]">No events yet.</p>
          ) : (
            <ol className="flex flex-col border-t border-border">
              {events.map((event) => (
                <li
                  key={event.id}
                  className="grid grid-cols-1 gap-1 border-b border-border py-2 sm:grid-cols-[11rem_1fr] sm:items-baseline sm:gap-4"
                >
                  <span className="font-mono text-[11px] text-[var(--muted-ink)]">
                    {formatTime(event.created_at)}
                  </span>
                  <span className="font-mono text-[13px]">{event.type}</span>
                </li>
              ))}
            </ol>
          )}
        </section>

        <section className="flex flex-col gap-2">
          <div className="flex items-baseline justify-between gap-3">
            <h2 className="text-sm font-semibold">GET /discrepancies</h2>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              disabled={busy !== null}
              onClick={() => void run("discrepancies", async () => {
                setDiscrepancies(await api.discrepancies());
              })}
            >
              Refresh
            </Button>
          </div>
          <p className="text-[12px] leading-5 text-[var(--muted-ink)]">
            Authorize then Cancel. Recon records cancelled vs authorized and does not change
            payment status.
          </p>
          {discrepancies.length === 0 ? (
            <p className="text-[13px] text-[var(--muted-ink)]">None yet.</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full min-w-xl border-t border-border text-left text-[13px]">
                <thead>
                  <tr className="border-b border-border">
                    <th className="py-2 pr-3 font-medium">Payment</th>
                    <th className="py-2 pr-3 font-medium">Internal</th>
                    <th className="py-2 pr-3 font-medium">Provider</th>
                    <th className="py-2 font-medium">Recorded</th>
                  </tr>
                </thead>
                <tbody>
                  {discrepancies.map((row) => (
                    <tr
                      key={row.id}
                      className={cn(
                        "border-b border-border font-mono text-[12px]",
                        row.payment_id === currentId && "bg-card",
                      )}
                    >
                      <td className="py-2 pr-3">
                        <button
                          type="button"
                          className="hover:underline"
                          onClick={() => writeSession({ ...session, currentId: row.payment_id })}
                        >
                          {row.payment_id}
                        </button>
                      </td>
                      <td className="py-2 pr-3">{row.internal_status}</td>
                      <td className="py-2 pr-3">{row.provider_status}</td>
                      <td className="py-2">{formatTime(row.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section className="flex flex-col gap-2">
          <h2 className="text-sm font-semibold">POST /webhooks/mock</h2>
          <p className="text-[12px] leading-5 text-[var(--muted-ink)]">
            Applies a provider event without calling the mock PSP. Replay last with a different
            type for 409 PROVIDER_EVENT_REUSE.
          </p>
          <div className="flex flex-wrap items-end gap-2">
            <div className="flex min-w-48 flex-1 flex-col gap-1">
              <Label htmlFor="inbound-type">type</Label>
              <select
                id="inbound-type"
                value={inboundType}
                onChange={(e) => setInboundType(e.target.value)}
                className="h-8 rounded-sm border border-input bg-card px-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
              >
                {INBOUND_TYPES.map((type) => (
                  <option key={type} value={type}>
                    {type}
                  </option>
                ))}
              </select>
            </div>
            <Button
              type="button"
              disabled={busy !== null || !payment}
              onClick={() => {
                if (!payment) return;
                const id = `psp_${crypto.randomUUID()}`;
                void run("inbound", async () => {
                  const p = await api.inbound(id, inboundType, payment.id);
                  setLastInboundId(id);
                  setPayment(p);
                  setEvents(await api.events(p.id, true));
                });
              }}
            >
              Send
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={busy !== null || !payment || !lastInboundId}
              onClick={() => {
                if (!payment || !lastInboundId) return;
                void run("inbound-replay", async () => {
                  const p = await api.inbound(lastInboundId, inboundType, payment.id);
                  setPayment(p);
                  setEvents(await api.events(p.id, true));
                });
              }}
            >
              Replay last
            </Button>
          </div>
        </section>
      </main>

      <aside className="flex min-w-0 flex-col gap-3 bg-[var(--mast)] px-4 py-4 lg:sticky lg:top-0 lg:h-dvh lg:overflow-auto">
        <h2 className="text-sm font-semibold">Last HTTP</h2>
        {exchange ? (
          <>
            <p className="font-mono text-[12px] break-all">
              {exchange.method} {exchange.path}
            </p>
            <p
              className={cn(
                "font-mono text-[13px]",
                exchange.status >= 400 || exchange.status === 0
                  ? "text-[var(--fail)]"
                  : "text-[var(--ok)]",
              )}
            >
              {exchange.status || "failed"}
            </p>
            {Object.keys(exchange.requestHeaders).length > 0 ? (
              <pre className="overflow-x-auto font-mono text-[11px] leading-5 text-[var(--muted-ink)]">
                {Object.entries(exchange.requestHeaders)
                  .map(([key, value]) => `${key}: ${value}`)
                  .join("\n")}
              </pre>
            ) : null}
            <h3 className="text-[12px] font-semibold">Request</h3>
            <pre className="overflow-x-auto border border-border bg-card p-2 font-mono text-[11px] leading-5">
              {pretty(exchange.requestBody)}
            </pre>
            <h3 className="text-[12px] font-semibold">Response</h3>
            <pre className="overflow-x-auto border border-border bg-card p-2 font-mono text-[11px] leading-5">
              {pretty(exchange.responseBody)}
            </pre>
          </>
        ) : (
          <p className="text-[13px] leading-5 text-[var(--muted-ink)]">
            Create, authorize, capture, refund, or cancel. The JSON from that call lands here.
          </p>
        )}
      </aside>
    </div>
  );
}
