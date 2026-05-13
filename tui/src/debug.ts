import { appendFileSync, mkdirSync } from "fs";
import { dirname, resolve } from "path";

type DebugLevel = "error" | "warn" | "info" | "debug" | "trace";
type DebugModule = "ws" | "render" | "state" | "perf";
type DebugContentMode = "off" | "preview" | "full";

const levelPriority: Record<DebugLevel, number> = {
  error: 0,
  warn: 1,
  info: 2,
  debug: 3,
  trace: 4,
};

function parseEnabled(value: string | undefined): boolean {
  if (!value) return false;
  return value === "1" || value.toLowerCase() === "true";
}

function parseLevel(value: string | undefined): DebugLevel {
  if (!value) return "info";
  const normalized = value.toLowerCase();
  if (normalized === "error" || normalized === "warn" || normalized === "info" || normalized === "debug" || normalized === "trace") {
    return normalized;
  }
  return "info";
}

function parseModules(value: string | undefined): Set<DebugModule> {
  if (!value || value.trim() === "") return new Set<DebugModule>(["ws", "render"]);
  const allowed = new Set<DebugModule>(["ws", "render", "state", "perf"]);
  const parsed = new Set<DebugModule>();
  for (const raw of value.split(",")) {
    const item = raw.trim() as DebugModule;
    if (allowed.has(item)) parsed.add(item);
  }
  return parsed.size > 0 ? parsed : new Set<DebugModule>(["ws", "render"]);
}

function trimText(text: string, max = 80): string {
  const compact = text.replace(/\s+/g, " ").trim();
  if (compact.length <= max) return compact;
  return `${compact.slice(0, max)}...`;
}

function redact(value: unknown): unknown {
  if (typeof value === "string") {
    const masked = value
      .replace(/Bearer\s+[A-Za-z0-9._-]+/gi, "Bearer [REDACTED]")
      .replace(/sk-[A-Za-z0-9_-]{12,}/g, "sk-[REDACTED]");
    if (contentMode === "full") {
      return masked;
    }
    return trimText(masked);
  }
  if (Array.isArray(value)) return value.map((item) => redact(item));
  if (value && typeof value === "object") {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      out[k] = redact(v);
    }
    return out;
  }
  return value;
}

const enabled = parseEnabled(process.env.TUI_DEBUG);
const minLevel = parseLevel(process.env.TUI_DEBUG_LEVEL);
const modules = parseModules(process.env.TUI_DEBUG_MODULE);
const stderrEnabled = parseEnabled(process.env.TUI_DEBUG_STDERR);
const logPath = resolve(process.cwd(), process.env.TUI_DEBUG_FILE || "logs/frontend-debug.log");
const contentMode: DebugContentMode = (() => {
  const raw = (process.env.TUI_DEBUG_CONTENT || "preview").toLowerCase();
  if (raw === "off" || raw === "preview" || raw === "full") return raw;
  return "preview";
})();

if (enabled) {
  mkdirSync(dirname(logPath), { recursive: true });
}

let seq = 0;

export interface DebugContext {
  turnId?: string;
  [key: string]: unknown;
}

export function debugLog(level: DebugLevel, module: DebugModule, event: string, data?: Record<string, unknown>, ctx?: DebugContext): void {
  if (!enabled) return;
  if (!modules.has(module)) return;
  if (levelPriority[level] > levelPriority[minLevel]) return;

  const entry = {
    ts: new Date().toISOString(),
    level,
    module,
    event,
    turnId: ctx?.turnId ?? "",
    seq: ++seq,
    data: redact(data ?? {}),
  };
  const line = `${JSON.stringify(entry)}\n`;
  appendFileSync(logPath, line);
  if (stderrEnabled && (level === "error" || level === "warn")) {
    // eslint-disable-next-line no-console
    console.error(line.trimEnd());
  }
}

export function isDebugEnabled(): boolean {
  return enabled;
}

export function debugLogPath(): string {
  return logPath;
}

export function debugContentMode(): DebugContentMode {
  return contentMode;
}
