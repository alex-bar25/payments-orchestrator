const KEY = "po.session";

export type LastCreate = {
  key: string;
  amount: number;
  currency: string;
};

export type StoredSession = {
  currentId: string | null;
  ids: string[];
  lastCreate: LastCreate | null;
};

const empty: StoredSession = { currentId: null, ids: [], lastCreate: null };
const listeners = new Set<() => void>();
let snapshot: StoredSession = empty;
let rawCache: string | null = null;

function parse(raw: string | null): StoredSession {
  if (!raw) {
    return empty;
  }
  try {
    const parsed = JSON.parse(raw) as Partial<StoredSession>;
    return {
      currentId: parsed.currentId ?? null,
      ids: Array.isArray(parsed.ids) ? parsed.ids.filter((id) => typeof id === "string") : [],
      lastCreate: parsed.lastCreate ?? null,
    };
  } catch {
    return empty;
  }
}

function read(): StoredSession {
  const raw = sessionStorage.getItem(KEY);
  if (raw === rawCache) {
    return snapshot;
  }
  rawCache = raw;
  snapshot = parse(raw);
  return snapshot;
}

export function subscribeSession(onStoreChange: () => void) {
  listeners.add(onStoreChange);
  return () => {
    listeners.delete(onStoreChange);
  };
}

export function getSessionSnapshot() {
  return read();
}

export function getServerSessionSnapshot(): StoredSession {
  return empty;
}

export function writeSession(next: StoredSession) {
  sessionStorage.setItem(KEY, JSON.stringify(next));
  rawCache = null;
  read();
  listeners.forEach((listener) => listener());
}

export function rememberPayment(session: StoredSession, id: string, create?: LastCreate): StoredSession {
  return {
    currentId: id,
    ids: [id, ...session.ids.filter((x) => x !== id)].slice(0, 12),
    lastCreate: create ?? session.lastCreate,
  };
}
