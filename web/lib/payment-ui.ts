import type { Status } from "@/lib/api";

export type Action = "authorize" | "capture" | "refund" | "cancel";

export function formatMinor(amount: number, currency: string): string {
  try {
    return new Intl.NumberFormat("en-GB", {
      style: "currency",
      currency,
    }).format(amount / 100);
  } catch {
    return `${amount} ${currency}`;
  }
}

export function formatTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) {
    return iso;
  }
  return d.toISOString().replace("T", " ").replace("Z", " UTC");
}

export function actionsFor(status: Status): Action[] {
  switch (status) {
    case "requires_payment":
      return ["authorize"];
    case "authorized":
      return ["capture", "cancel"];
    case "captured":
      return ["refund"];
    default:
      return [];
  }
}

export function stampClass(status: Status): string {
  switch (status) {
    case "authorized":
      return "border-[var(--stamp)] text-[var(--stamp)]";
    case "captured":
      return "border-[var(--ok)] text-[var(--ok)]";
    case "processing":
      return "border-[var(--hold)] text-[var(--hold)]";
    case "requires_payment":
      return "border-[var(--ink)] text-[var(--ink)]";
    case "refunded":
      return "border-[var(--muted-ink)] text-[var(--muted-ink)]";
    case "failed":
    case "cancelled":
      return "border-[var(--fail)] text-[var(--fail)]";
  }
}

export function ticketRule(status: Status): string {
  switch (status) {
    case "authorized":
      return "border-l-[var(--stamp)]";
    case "captured":
      return "border-l-[var(--ok)]";
    case "processing":
      return "border-l-[var(--hold)]";
    case "failed":
    case "cancelled":
      return "border-l-[var(--fail)]";
    case "refunded":
      return "border-l-[var(--muted-ink)]";
    default:
      return "border-l-[var(--ink)]";
  }
}
