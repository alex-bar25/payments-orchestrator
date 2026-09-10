"use client";

import { useCallback, useEffect, useState, useSyncExternalStore } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api, ApiError, type Discrepancy, type Payment, type PaymentEvent } from "@/lib/api";
import {
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
  { amount: 4999, label: "4999", hint: "authorize, capture, refund" },
  { amount: 1, label: "1", hint: "authorize declined, status failed" },
  { amount: 2, label: "2", hint: "capture declined, stays authorized" },
  { amount: 3, label: "3", hint: "refund declined, stays captured" },
] as const;

const INBOUND_TYPES = [
  "payment.authorized",
  "payment.failed",
  "payment.captured",
  "payment.refunded",
  "payment.cancelled",
] as const;

type Notice = { kind: "ok" | "err"; code?: string; text: string };

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
  const [notice, setNotice] = useState<Notice | null>(null);

  const remember = useCallback(
    (id: string, create?: LastCreate) => {
      writeSession(rememberPayment(session, id, create));
    },
    [session],
  );

  const loadPayment = useCallback(async (id: string) => {
    const [p, ev] = await Promise.all([api.get(id), api.events(id)]);
    setPayment(p);
    setEvents(ev);
    return p;
  }, []);

  const refreshSide = useCallback(async () => {
    setDiscrepancies(await api.discrepancies());
  }, []);

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
        const [p, ev] = await Promise.all([api.get(currentId), api.events(currentId)]);
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
        const rows = await api.discrepancies();
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
      void loadPayment(payment.id).catch(() => undefined);
    }, 2000);
    return () => clearInterval(t);
  }, [payment, loadPayment]);

  async function run(label: string, fn: () => Promise<void>) {
    setBusy(label);
    setNotice(null);
    try {
      await fn();
    } catch (err) {
      if (err instanceof ApiError) {
        setNotice({ kind: "err", code: err.code, text: err.message });
      } else {
        setNotice({ kind: "err", text: "request failed" });
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
      setNotice({
        kind: "ok",
        text: opts?.key ? `idempotent replay ${p.id}` : `created ${p.id}`,
      });
      remember(p.id, { key, amount: nextAmount, currency: nextCurrency });
      setPayment(p);
      setEvents(await api.events(p.id));
    });
  }

  function act(action: Action) {
    if (!payment) return;
    return run(action, async () => {
      const fn = api[action];
      const p = await fn(payment.id, crypto.randomUUID());
      setPayment(p);
      setEvents(await api.events(p.id));
      setNotice({ kind: "ok", text: `${action} → ${p.status}` });
    });
  }

  const legal = payment ? actionsFor(payment.status) : [];

  return (
    <div className="flex min-h-dvh flex-col">
      <header className="flex flex-wrap items-end justify-between gap-3 border-b border-border bg-[var(--mast)] px-5 py-4 lg:px-8">
        <div>
          <p className="font-sans text-xl font-semibold tracking-tight">Clearing desk</p>
          <p className="mt-1 max-w-xl text-[13px] text-[var(--muted-ink)]">
            Mock card lifecycle for review. Integer minor units, one status, append-only events.
            Buttons hide illegal moves; the API still rejects them.
          </p>
        </div>
        <div className="flex items-stretch border border-border bg-card text-[13px]">
          <span className="px-3 py-2 font-mono">merch_demo</span>
          <span className="border-l border-border px-3 py-2">
            API {apiUp === null ? "…" : apiUp ? "ok" : "down"}
          </span>
        </div>
      </header>

      <div className="flex flex-1 flex-col lg:flex-row">
        <aside className="flex w-full flex-col gap-8 border-b border-border px-5 py-6 lg:w-[22rem] lg:shrink-0 lg:border-r lg:border-b-0 lg:px-6">
          <section className="flex flex-col gap-3">
            <h2 className="text-base font-semibold">Try this</h2>
            <p className="text-[13px] leading-5 text-[var(--muted-ink)]">
              Create one of these, then run the actions on the ticket. Amount 1 fails authorize, 2
              fails capture, 3 fails refund.
            </p>
            <ul className="flex flex-col border border-border bg-card">
              {PRESETS.map((preset) => (
                <li key={preset.amount} className="border-b border-border last:border-b-0">
                  <button
                    type="button"
                    disabled={busy !== null}
                    onClick={() => void createWith(preset.amount, currency || "EUR")}
                    className="flex w-full items-baseline justify-between gap-3 px-3 py-2.5 text-left hover:bg-muted disabled:opacity-50"
                  >
                    <span className="font-mono text-sm">{preset.label}</span>
                    <span className="text-[12px] text-[var(--muted-ink)]">{preset.hint}</span>
                  </button>
                </li>
              ))}
            </ul>
          </section>

          <form
            className="flex flex-col gap-3"
            onSubmit={(e) => {
              e.preventDefault();
              const n = Number(amount);
              void createWith(n, currency.trim().toUpperCase() || "EUR");
            }}
          >
            <h2 className="text-base font-semibold">Custom amount</h2>
            <div className="grid grid-cols-[1fr_5rem] gap-2">
              <div className="flex flex-col gap-1">
                <Label htmlFor="amount">Minor units</Label>
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
              Create payment
            </Button>
          </form>

          {lastCreate ? (
            <section className="flex flex-col gap-2">
              <h2 className="text-base font-semibold">Idempotency</h2>
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
                Replay same body
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
                Replay different body
              </Button>
              <p className="text-[12px] leading-5 text-[var(--muted-ink)]">
                Same key + same body returns the original row. Same key + different amount is 409
                IDEMPOTENCY_KEY_REUSE.
              </p>
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
            <Label htmlFor="lookup">Load by id</Label>
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
            <section className="flex flex-col gap-2">
              <h2 className="text-base font-semibold">This tab</h2>
              <ul className="flex flex-col gap-1">
                {ids.map((id) => (
                  <li key={id}>
                    <button
                      type="button"
                      onClick={() => writeSession({ ...session, currentId: id })}
                      className={cn(
                        "w-full truncate border border-transparent px-2 py-1 text-left font-mono text-[12px] hover:border-border",
                        id === currentId && "border-border bg-card",
                      )}
                    >
                      {id}
                    </button>
                  </li>
                ))}
              </ul>
            </section>
          ) : null}
        </aside>

        <main className="flex min-w-0 flex-1 flex-col gap-8 px-5 py-6 lg:px-8">
          {notice ? (
            <p
              role="alert"
              className={cn(
                "border px-3 py-2 font-mono text-[13px]",
                notice.kind === "err"
                  ? "border-[var(--fail)] text-[var(--fail)]"
                  : "border-[var(--ok)] text-[var(--ok)]",
              )}
            >
              {notice.code ? `${notice.code}: ${notice.text}` : notice.text}
            </p>
          ) : null}

          {payment ? (
            <section className={cn("border border-border border-l-4 bg-card p-5", ticketRule(payment.status))}>
              <div className="flex flex-wrap items-start justify-between gap-3">
                <p className="font-mono text-[12px] break-all text-[var(--muted-ink)]">{payment.id}</p>
                <span
                  className={cn(
                    "border px-2 py-0.5 font-mono text-[12px]",
                    stampClass(payment.status),
                  )}
                >
                  {payment.status}
                </span>
              </div>
              <p className="mt-6 font-mono text-[clamp(2.4rem,7vw,4.4rem)] leading-none tracking-tight">
                {formatMinor(payment.amount, payment.currency)}
              </p>
              <p className="mt-2 font-mono text-[13px] text-[var(--muted-ink)]">
                {payment.amount} {payment.currency}
                {payment.provider_payment_id ? `  ${payment.provider_payment_id}` : ""}
              </p>
              <div className="mt-6 flex flex-wrap gap-2">
                {legal.map((action) => (
                  <Button
                    key={action}
                    type="button"
                    variant={action === "cancel" || action === "refund" ? "outline" : "default"}
                    disabled={busy !== null}
                    onClick={() => void act(action)}
                  >
                    {action}
                  </Button>
                ))}
                {legal.length === 0 ? (
                  <p className="text-[13px] text-[var(--muted-ink)]">No further moves from this status.</p>
                ) : null}
              </div>
            </section>
          ) : (
            <section className="border border-dashed border-border px-5 py-10">
              <p className="text-lg font-semibold">No payment on the blotter</p>
              <p className="mt-2 max-w-md text-[14px] leading-6 text-[var(--muted-ink)]">
                Create 4999, then authorize, capture, and refund. Or create 1, 2, or 3 to see the
                mock PSP decline a step.
              </p>
            </section>
          )}

          <section className="flex flex-col gap-3">
            <h2 className="text-base font-semibold">Events</h2>
            {events.length === 0 ? (
              <p className="text-[13px] text-[var(--muted-ink)]">Empty until a payment is loaded.</p>
            ) : (
              <ol className="flex flex-col border-t border-border">
                {events.map((event) => (
                  <li
                    key={event.id}
                    className="grid grid-cols-1 gap-1 border-b border-border py-2.5 sm:grid-cols-[11rem_1fr_auto] sm:items-baseline sm:gap-4"
                  >
                    <span className="font-mono text-[12px] text-[var(--muted-ink)]">
                      {formatTime(event.created_at)}
                    </span>
                    <span className="font-mono text-[13px]">{event.type}</span>
                    <span className="font-mono text-[11px] text-[var(--muted-ink)]">{event.id}</span>
                  </li>
                ))}
              </ol>
            )}
          </section>

          <section className="flex flex-col gap-3">
            <div className="flex items-baseline justify-between gap-3">
              <h2 className="text-base font-semibold">Discrepancies</h2>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                disabled={busy !== null}
                onClick={() => void run("discrepancies", refreshSide)}
              >
                Refresh
              </Button>
            </div>
            <p className="max-w-2xl text-[13px] leading-5 text-[var(--muted-ink)]">
              Authorize, then cancel. Cancel never voids the mock hold, so recon should record
              cancelled vs authorized without changing payment status. Needs the worker.
            </p>
            {discrepancies.length === 0 ? (
              <p className="text-[13px] text-[var(--muted-ink)]">None recorded yet.</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full min-w-[36rem] border-t border-border text-left text-[13px]">
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

          <section className="flex max-w-xl flex-col gap-3">
            <h2 className="text-base font-semibold">Inbound mock</h2>
            <p className="text-[13px] leading-5 text-[var(--muted-ink)]">
              Applies a provider event without another PSP call. Replay last with the same type
              returns the row. Change the type, then replay, for 409 PROVIDER_EVENT_REUSE.
            </p>
            <div className="flex flex-wrap items-end gap-2">
              <div className="flex min-w-[12rem] flex-1 flex-col gap-1">
                <Label htmlFor="inbound-type">Type</Label>
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
                    setEvents(await api.events(p.id));
                    setNotice({ kind: "ok", text: `inbound ${inboundType}` });
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
                    setEvents(await api.events(p.id));
                    setNotice({ kind: "ok", text: "inbound replay" });
                  });
                }}
              >
                Replay last
              </Button>
            </div>
          </section>
        </main>
      </div>
    </div>
  );
}
