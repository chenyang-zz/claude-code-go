import { useCallback, useEffect, useRef, useState } from "react";
import { debugContentMode, debugLog, debugLogPath, isDebugEnabled } from "../debug.js";
import { connectWS, sendApproval, sendInput, type WSMessage } from "../ws-client.js";
import type { PermissionDialogState, TokenState, TypewriterKind } from "../types/tui.js";
import { useChatLines } from "./useChatLines.js";
import { useTypewriter } from "./useTypewriter.js";
import { statusUsageDelta, usageForLine, usageForStatus, type TokenTotals } from "./usage.js";

export function useTuiSession(port: number) {
  const { lines, addLine, upsertLastLineOfType } = useChatLines(port);
  const [isThinking, setIsThinking] = useState(false);
  const [isRunning, setIsRunning] = useState(false);
  const [connected, setConnected] = useState(false);
  const [inputDisabled, setInputDisabled] = useState(true);
  const [permission, setPermission] = useState<PermissionDialogState>({
    visible: false,
    title: "",
    body: "",
  });
  const [tokens, setTokens] = useState<TokenState>({});
  const turnClosedRef = useRef(false);
  const activeTurnIdRef = useRef("");
  const turnSeqRef = useRef(0);
  const wsRef = useRef<WebSocket | null>(null);
  const debugContentRef = useRef(debugContentMode());
  const pendingUsageRef = useRef<{ payload: any; receivedAt: number } | null>(null);
  const sessionTokenTotalRef = useRef<TokenTotals>({ inputTokens: 0, outputTokens: 0 });
  const turnCumulativeRef = useRef<TokenTotals>({ inputTokens: 0, outputTokens: 0 });

  const getTurnId = useCallback(() => activeTurnIdRef.current, []);

  const textForDebug = useCallback((text: string) => {
    if (debugContentRef.current === "off") return undefined;
    if (debugContentRef.current === "full") return text;
    const normalized = text.replace(/\s+/g, " ").trim();
    return normalized.length > 120 ? `${normalized.slice(0, 120)}...` : normalized;
  }, []);

  const renderUsage = useCallback((payload: any, receivedAt: number) => {
    const lineUsage = usageForLine(payload);
    const statusUsage = usageForStatus(payload);
    const statusDelta = statusUsageDelta(payload, turnCumulativeRef.current);
    turnCumulativeRef.current = statusDelta.nextTurnCumulative;
    sessionTokenTotalRef.current = {
      inputTokens: sessionTokenTotalRef.current.inputTokens + statusDelta.delta.inputTokens,
      outputTokens: sessionTokenTotalRef.current.outputTokens + statusDelta.delta.outputTokens,
    };
    const inputTokens = lineUsage.input_tokens;
    const outputTokens = lineUsage.output_tokens;
    const statusInputTokens = statusUsage.input_tokens;
    const statusOutputTokens = statusUsage.output_tokens;
    setTokens({
      inputTokens: sessionTokenTotalRef.current.inputTokens,
      outputTokens: sessionTokenTotalRef.current.outputTokens,
    });
    addLine(
      `📊 in=${inputTokens ?? "?"} out=${outputTokens ?? "?"} cache_c=${lineUsage.cache_creation_input_tokens ?? "?"} cache_r=${lineUsage.cache_read_input_tokens ?? "?"} stop=${payload.stop_reason ?? "?"}`,
      "info",
    );
    setIsRunning(false);
    setIsThinking(false);
    turnClosedRef.current = true;
    debugLog("info", "state", "usage.render", {
      inputTokens: inputTokens ?? null,
      outputTokens: outputTokens ?? null,
      statusInputTokens: statusInputTokens ?? null,
      statusOutputTokens: statusOutputTokens ?? null,
      statusDeltaInputTokens: statusDelta.delta.inputTokens,
      statusDeltaOutputTokens: statusDelta.delta.outputTokens,
      sessionInputTokens: sessionTokenTotalRef.current.inputTokens,
      sessionOutputTokens: sessionTokenTotalRef.current.outputTokens,
      stopReason: payload.stop_reason ?? "",
      pendingMs: Date.now() - receivedAt,
    }, { turnId: activeTurnIdRef.current });
  }, [addLine]);

  const handleTypewriterDrainComplete = useCallback((reason: string) => {
    if (reason !== "usage") return;

    const pendingUsage = pendingUsageRef.current;
    if (!pendingUsage) {
      debugLog("warn", "state", "usage.drain.missing", {}, { turnId: activeTurnIdRef.current });
      return;
    }

    pendingUsageRef.current = null;
    renderUsage(pendingUsage.payload, pendingUsage.receivedAt);
  }, [renderUsage]);

  const handleTypewriterRender = useCallback((kind: TypewriterKind, visibleText: string) => {
    const text = kind === "thinking" ? `Thinking: ${visibleText}` : visibleText;
    upsertLastLineOfType(text, kind);
  }, [upsertLastLineOfType]);

  const typewriter = useTypewriter({
    onRender: handleTypewriterRender,
    getTurnId,
    onDrainComplete: handleTypewriterDrainComplete,
  });

  const resetPermission = useCallback(() => {
    setPermission({ visible: false, title: "", body: "" });
  }, []);

  const handleApprove = useCallback(() => {
    if (wsRef.current && connected) {
      sendApproval(wsRef.current, true);
    }
    resetPermission();
    setInputDisabled(false);
  }, [connected, resetPermission]);

  const handleDeny = useCallback(() => {
    if (wsRef.current && connected) {
      sendApproval(wsRef.current, false);
    }
    resetPermission();
    setInputDisabled(false);
  }, [connected, resetPermission]);

  const handleDelta = useCallback((text: string) => {
    if (!text) return;
    turnClosedRef.current = false;
    setIsRunning(true);
    setIsThinking(false);
    typewriter.flush("thinking");
    typewriter.append("delta", text);
    const snapshot = typewriter.getSnapshot("delta");
    debugLog("trace", "render", "typewriter.target.append", {
      kind: "delta",
      chunkLen: text.length,
      chunk: textForDebug(text),
      targetLen: snapshot.targetText.length,
      visibleLen: snapshot.visibleText.length,
    }, { turnId: activeTurnIdRef.current });
  }, [textForDebug, typewriter]);

  const handleThinking = useCallback((thought: string) => {
    if (pendingUsageRef.current) {
      debugLog("debug", "render", "thinking.drop.after.usage.pending", {}, { turnId: activeTurnIdRef.current });
      return;
    }
    if (turnClosedRef.current) {
      debugLog("debug", "render", "thinking.drop.after.turn.closed", {}, { turnId: activeTurnIdRef.current });
      return;
    }
    if (typewriter.getSnapshot("delta").targetText !== "") {
      debugLog("debug", "render", "thinking.drop.after.delta.started", {}, { turnId: activeTurnIdRef.current });
      return;
    }
    setIsRunning(true);
    setIsThinking(true);
    if (!thought) return;
    typewriter.setTarget("thinking", thought);
    debugLog("trace", "render", "thinking.render", {
      thoughtLen: thought.length,
      thought: textForDebug(thought),
      mode: "typewriter",
    }, { turnId: activeTurnIdRef.current });
  }, [textForDebug, typewriter]);

  const handleUsage = useCallback((payload: any) => {
    setIsThinking(false);
    pendingUsageRef.current = { payload, receivedAt: Date.now() };
    const usage = usageForLine(payload);
    const statusUsage = usageForStatus(payload);
    debugLog("info", "state", "usage.pending", {
      inputTokens: usage.input_tokens ?? null,
      outputTokens: usage.output_tokens ?? null,
      statusInputTokens: statusUsage.input_tokens ?? null,
      statusOutputTokens: statusUsage.output_tokens ?? null,
      stopReason: payload.stop_reason ?? "",
      delta: typewriter.getSnapshot("delta"),
      thinking: typewriter.getSnapshot("thinking"),
    }, { turnId: activeTurnIdRef.current });
    typewriter.requestDrain("usage");
  }, [typewriter]);

  const handleLineMessage = useCallback((text: string) => {
    if (!text) return;

    const snapshot = typewriter.getSnapshot("delta");
    if (snapshot.targetText.trim() === "" && snapshot.visibleText.trim() === "") {
      addLine(text, "line");
      debugLog("debug", "render", "line.fallback", { textLen: text.length, text: textForDebug(text) }, { turnId: activeTurnIdRef.current });
      return;
    }

    debugLog("debug", "render", "line.skipped", {
      textLen: text.length,
      text: textForDebug(text),
      targetLen: snapshot.targetText.length,
      visibleLen: snapshot.visibleText.length,
    }, { turnId: activeTurnIdRef.current });
  }, [addLine, textForDebug, typewriter]);

  const handleRuntimeEvent = useCallback((evt: any) => {
    if (!evt) return;

    switch (evt.type) {
      case "message.delta":
        handleDelta(evt.payload?.text ?? "");
        break;
      case "thinking":
        handleThinking(evt.payload?.thinking ?? evt.payload?.text ?? "");
        break;
      case "tool.call.started": {
        const name = evt.payload?.name ?? "unknown";
        setIsRunning(true);
        setIsThinking(false);
        addLine(`⚡ ${name}`, "tool");
        break;
      }
      case "tool.call.finished": {
        const name = evt.payload?.name ?? "unknown";
        const isErr = evt.payload?.is_error;
        addLine(`${isErr ? "✗" : "✓"} ${name}${isErr ? " (error)" : ""}`, isErr ? "error" : "tool");
        break;
      }
      case "error":
        addLine(`✗ ${evt.payload?.message ?? evt.payload?.error ?? "Error"}`, "error");
        setIsThinking(false);
        break;
      case "usage":
        handleUsage(evt.payload ?? {});
        break;
      case "retry.attempted":
        addLine(`⟳ Attempt ${evt.payload?.attempt}/${evt.payload?.max_attempts}: ${evt.payload?.error}`, "info");
        break;
      case "model.fallback":
        addLine(`⇄ Model fallback: ${evt.payload?.original_model} → ${evt.payload?.fallback_model}`, "info");
        break;
      case "compact.done":
        addLine(`📦 Compact: ${evt.payload?.pre_token_count} → ${evt.payload?.post_token_count} tokens`, "info");
        break;
      case "tool.progress":
        break;
    }
  }, [addLine, handleDelta, handleThinking, handleUsage]);

  const handleWSMessage = useCallback((msg: WSMessage) => {
    debugLog("debug", "ws", "recv", {
      msgType: msg.type,
      evtType: msg.payload?.type ?? "",
    }, { turnId: activeTurnIdRef.current });

    switch (msg.type) {
      case "event":
        handleRuntimeEvent(msg.payload);
        break;
      case "line":
        handleLineMessage(msg.payload?.text ?? "");
        break;
      case "approval_prompt":
        setPermission({
          visible: true,
          title: msg.payload?.title ?? "Approval Required",
          body: msg.payload?.body ?? "",
        });
        setInputDisabled(true);
        break;
    }
  }, [handleLineMessage, handleRuntimeEvent]);

  useEffect(() => {
    if (isDebugEnabled()) {
      debugLog("info", "state", "app.init", { port, logPath: debugLogPath(), contentMode: debugContentRef.current });
    }

    const ws = connectWS(port, handleWSMessage);
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      setInputDisabled(false);
      debugLog("info", "ws", "open", { port });
      addLine("✓ Connected to engine", "info");
    };

    ws.onclose = () => {
      setConnected(false);
      setInputDisabled(true);
      resetPermission();
      debugLog("warn", "ws", "close", {});
      addLine("✗ Disconnected", "error");
    };

    return () => {
      typewriter.resetAll();
      ws.close();
    };
  }, [addLine, handleWSMessage, port, resetPermission, typewriter]);

  const submit = useCallback((text: string) => {
    if (!wsRef.current || !connected) return;

    turnSeqRef.current += 1;
    activeTurnIdRef.current = `turn-${turnSeqRef.current}`;
    debugLog("info", "state", "turn.start", {
      promptLen: text.length,
    }, { turnId: activeTurnIdRef.current });
    turnClosedRef.current = false;
    pendingUsageRef.current = null;
    turnCumulativeRef.current = { inputTokens: 0, outputTokens: 0 };
    typewriter.resetAll();
    addLine(`❯ ${text}`, "line");
    sendInput(wsRef.current, text);
    setIsRunning(true);
    setIsThinking(true);
  }, [addLine, connected, typewriter]);

  return {
    lines,
    connected,
    isRunning,
    isThinking,
    inputDisabled,
    permission,
    tokens,
    submit,
    handleApprove,
    handleDeny,
  };
}
