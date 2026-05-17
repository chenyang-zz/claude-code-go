import { useCallback, useEffect, useMemo, useRef } from "react";
import { debugLog } from "../debug.js";
import type { TypewriterKind } from "../types/tui.js";

type DrainReason = "usage" | "reset" | "unmount";

interface TypewriterSpeed {
  charsPerTick: number;
  intervalMs: number;
}

interface TypewriterState {
  kind: TypewriterKind;
  targetText: string;
  visibleText: string;
  timer: ReturnType<typeof setTimeout> | null;
  charsPerTick: number;
  intervalMs: number;
}

interface TypewriterOptions {
  onRender: (kind: TypewriterKind, visibleText: string) => void;
  getTurnId: () => string;
  onDrainComplete?: (reason: DrainReason) => void;
}

function parsePositiveInt(value: string | undefined, fallback: number): number {
  if (!value) return fallback;
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

function speedFor(kind: TypewriterKind): TypewriterSpeed {
  const intervalMs = parsePositiveInt(process.env.TUI_TYPEWRITER_INTERVAL_MS, 16);
  if (kind === "delta") {
    return {
      charsPerTick: parsePositiveInt(process.env.TUI_TYPEWRITER_DELTA_CHARS_PER_TICK, 6),
      intervalMs,
    };
  }
  return {
    charsPerTick: parsePositiveInt(process.env.TUI_TYPEWRITER_THINKING_CHARS_PER_TICK, 8),
    intervalMs,
  };
}

function createWriter(kind: TypewriterKind): TypewriterState {
  const speed = speedFor(kind);
  return {
    kind,
    targetText: "",
    visibleText: "",
    timer: null,
    charsPerTick: speed.charsPerTick,
    intervalMs: speed.intervalMs,
  };
}

export function useTypewriter({ onRender, getTurnId, onDrainComplete }: TypewriterOptions) {
  const deltaWriterRef = useRef<TypewriterState>(createWriter("delta"));
  const thinkingWriterRef = useRef<TypewriterState>(createWriter("thinking"));
  const drainRequestRef = useRef<{ requested: boolean; reason: DrainReason }>({
    requested: false,
    reason: "usage",
  });

  const snapshotWriter = useCallback((writer: TypewriterState) => ({
    kind: writer.kind,
    targetLen: writer.targetText.length,
    visibleLen: writer.visibleText.length,
    remainingLen: Math.max(0, writer.targetText.length - writer.visibleText.length),
    charsPerTick: writer.charsPerTick,
    intervalMs: writer.intervalMs,
  }), []);

  const renderWriter = useCallback((writer: TypewriterState) => {
    if (!writer.visibleText) return;
    onRender(writer.kind, writer.visibleText);
  }, [onRender]);

  const isWriterDrained = useCallback((writer: TypewriterState) => (
    writer.visibleText === writer.targetText
  ), []);

  const isAllDrained = useCallback(() => (
    isWriterDrained(deltaWriterRef.current) && isWriterDrained(thinkingWriterRef.current)
  ), [isWriterDrained]);

  const completeDrainIfReady = useCallback(() => {
    if (!drainRequestRef.current.requested || !isAllDrained()) return;

    const reason = drainRequestRef.current.reason;
    drainRequestRef.current.requested = false;
    debugLog("debug", "render", "typewriter.drain.complete", {
      reason,
      delta: snapshotWriter(deltaWriterRef.current),
      thinking: snapshotWriter(thinkingWriterRef.current),
    }, { turnId: getTurnId() });
    onDrainComplete?.(reason);
  }, [getTurnId, isAllDrained, onDrainComplete, snapshotWriter]);

  const tickWriter = useCallback((writer: TypewriterState) => {
    writer.timer = null;
    if (writer.visibleText === writer.targetText) return;

    const targetChars = Array.from(writer.targetText);
    const visibleChars = Array.from(writer.visibleText);
    writer.visibleText = targetChars.slice(0, visibleChars.length + writer.charsPerTick).join("");
    renderWriter(writer);
    debugLog("trace", "render", "typewriter.tick", snapshotWriter(writer), { turnId: getTurnId() });

    if (writer.visibleText !== writer.targetText) {
      writer.timer = setTimeout(() => tickWriter(writer), writer.intervalMs);
      return;
    }

    completeDrainIfReady();
  }, [completeDrainIfReady, getTurnId, renderWriter, snapshotWriter]);

  const startWriter = useCallback((writer: TypewriterState) => {
    if (writer.timer || writer.visibleText === writer.targetText) return;
    debugLog("debug", "render", "typewriter.start", snapshotWriter(writer), { turnId: getTurnId() });
    writer.timer = setTimeout(() => tickWriter(writer), writer.intervalMs);
  }, [getTurnId, snapshotWriter, tickWriter]);

  const append = useCallback((kind: TypewriterKind, text: string) => {
    const writer = kind === "delta" ? deltaWriterRef.current : thinkingWriterRef.current;
    writer.targetText += text;
    startWriter(writer);
  }, [startWriter]);

  const setTarget = useCallback((kind: TypewriterKind, text: string) => {
    const writer = kind === "delta" ? deltaWriterRef.current : thinkingWriterRef.current;
    writer.targetText = text;
    if (!text.startsWith(writer.visibleText)) {
      writer.visibleText = "";
    }
    startWriter(writer);
  }, [startWriter]);

  const flush = useCallback((kind: TypewriterKind) => {
    const writer = kind === "delta" ? deltaWriterRef.current : thinkingWriterRef.current;
    if (writer.timer) {
      clearTimeout(writer.timer);
      writer.timer = null;
    }
    if (writer.visibleText !== writer.targetText) {
      writer.visibleText = writer.targetText;
      renderWriter(writer);
    }
    debugLog("debug", "render", "typewriter.flush", snapshotWriter(writer), { turnId: getTurnId() });
    completeDrainIfReady();
  }, [completeDrainIfReady, getTurnId, renderWriter, snapshotWriter]);

  const reset = useCallback((kind: TypewriterKind) => {
    const writer = kind === "delta" ? deltaWriterRef.current : thinkingWriterRef.current;
    if (writer.timer) {
      clearTimeout(writer.timer);
      writer.timer = null;
    }
    writer.targetText = "";
    writer.visibleText = "";
    debugLog("debug", "render", "typewriter.reset", snapshotWriter(writer), { turnId: getTurnId() });
  }, [getTurnId, snapshotWriter]);

  const resetAll = useCallback(() => {
    drainRequestRef.current.requested = false;
    drainRequestRef.current.reason = "reset";
    reset("delta");
    reset("thinking");
  }, [reset]);

  const requestDrain = useCallback((reason: DrainReason) => {
    drainRequestRef.current = { requested: true, reason };
    debugLog("debug", "render", "typewriter.drain.request", {
      reason,
      delta: snapshotWriter(deltaWriterRef.current),
      thinking: snapshotWriter(thinkingWriterRef.current),
    }, { turnId: getTurnId() });
    completeDrainIfReady();
  }, [completeDrainIfReady, getTurnId, snapshotWriter]);

  const getSnapshot = useCallback((kind: TypewriterKind) => {
    const writer = kind === "delta" ? deltaWriterRef.current : thinkingWriterRef.current;
    return {
      targetText: writer.targetText,
      visibleText: writer.visibleText,
      ...snapshotWriter(writer),
    };
  }, [snapshotWriter]);

  useEffect(() => () => {
    drainRequestRef.current = { requested: true, reason: "unmount" };
    resetAll();
  }, [resetAll]);

  return useMemo(() => ({
    append,
    setTarget,
    flush,
    reset,
    resetAll,
    requestDrain,
    isAllDrained,
    getSnapshot,
  }), [append, flush, getSnapshot, isAllDrained, requestDrain, reset, resetAll, setTarget]);
}
