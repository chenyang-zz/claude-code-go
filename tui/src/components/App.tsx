import React, { useState, useCallback, useEffect, useRef } from "react";
import { Box, Text, useApp } from "ink";
import { Chat, type Line } from "./Chat.js";
import { Input } from "./Input.js";
import { PermissionDialog } from "./PermissionDialog.js";
import { StatusLine } from "./StatusLine.js";
import { connectWS, sendInput, sendApproval, type WSMessage } from "../ws-client.js";

interface AppProps {
  port: number;
}

export function App({ port }: AppProps) {
  const { exit } = useApp();
  const [lines, setLines] = useState<Line[]>([
    { id: 0, text: "Claude Code TUI", type: "info" },
    { id: 1, text: `Connected on port ${port}`, type: "info" },
  ]);
  const [isThinking, setIsThinking] = useState(false);
  const [isRunning, setIsRunning] = useState(false);
  const [connected, setConnected] = useState(false);
  const [inputDisabled, setInputDisabled] = useState(true);
  const lineIdRef = useRef(2);
  const currentDeltaRef = useRef("");
  const deltaQueueRef = useRef<string[]>([]);
  const deltaProcessingRef = useRef(false);
  const wsRef = useRef<WebSocket | null>(null);

  // Permission dialog state
  const [dialogVisible, setDialogVisible] = useState(false);
  const [dialogTitle, setDialogTitle] = useState("");
  const [dialogBody, setDialogBody] = useState("");

  // Status line state
  const [inputTokens, setInputTokens] = useState<number | undefined>();
  const [outputTokens, setOutputTokens] = useState<number | undefined>();

  const addLine = useCallback((text: string, type: Line["type"]) => {
    const id = lineIdRef.current++;
    setLines((prev) => [...prev, { id, text, type }]);
  }, []);

  const handleApprove = useCallback(() => {
    if (wsRef.current && connected) {
      sendApproval(wsRef.current, true);
    }
    setDialogVisible(false);
    setDialogTitle("");
    setDialogBody("");
    setInputDisabled(false);
  }, [connected]);

  const handleDeny = useCallback(() => {
    if (wsRef.current && connected) {
      sendApproval(wsRef.current, false);
    }
    setDialogVisible(false);
    setDialogTitle("");
    setDialogBody("");
    setInputDisabled(false);
  }, [connected]);

  useEffect(() => {
    const ws = connectWS(port, (msg: WSMessage) => {
      switch (msg.type) {
        case "event": {
          const evt = msg.payload;
          if (!evt) break;

          switch (evt.type) {
            case "message.delta": {
              const text = evt.payload?.text ?? "";
              if (text) {
                setIsRunning(true);
                setIsThinking(false);
                currentDeltaRef.current += text;
                // Queue the delta for async processing to allow React
                // to render between messages (prevent batching).
                deltaQueueRef.current.push(text);
                if (!deltaProcessingRef.current) {
                  deltaProcessingRef.current = true;
                  processDeltaQueue();
                }
              }
              break;
            }
            case "thinking": {
              const thought = evt.payload?.thinking ?? evt.payload?.text ?? "";
              setIsRunning(true);
              setIsThinking(true);
              if (thought) {
                const id = lineIdRef.current++;
                setLines((prev) => {
                  const last = prev[prev.length - 1];
                  if (last?.type === "thinking") {
                    return [...prev.slice(0, -1), { id, text: `Thinking: ${thought}`, type: "thinking" }];
                  }
                  return [...prev, { id, text: `Thinking: ${thought}`, type: "thinking" }];
                });
              }
              break;
            }
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
            case "error": {
              addLine(`✗ ${evt.payload?.message ?? evt.payload?.error ?? "Error"}`, "error");
              setIsThinking(false);
              break;
            }
            case "usage": {
              const usage = evt.payload?.turn_usage ?? evt.payload?.usage ?? {};
              const inTk = usage.input_tokens;
              const outTk = usage.output_tokens;
              if (inTk != null) setInputTokens(inTk);
              if (outTk != null) setOutputTokens(outTk);
              // Finalize delta content: strip raw ``` markers and add code blocks
              finalizeResponse();
              addLine(
                `📊 in=${inTk ?? "?"} out=${outTk ?? "?"} cache_c=${usage.cache_creation_input_tokens ?? "?"} cache_r=${usage.cache_read_input_tokens ?? "?"} stop=${evt.payload?.stop_reason ?? "?"}`,
                "info"
              );
              setIsRunning(false);
              setIsThinking(false);
              currentDeltaRef.current = "";
              break;
            }
            case "retry.attempted": {
              addLine(`⟳ Attempt ${evt.payload?.attempt}/${evt.payload?.max_attempts}: ${evt.payload?.error}`, "info");
              break;
            }
            case "model.fallback": {
              addLine(`⇄ Model fallback: ${evt.payload?.original_model} → ${evt.payload?.fallback_model}`, "info");
              break;
            }
            case "compact.done": {
              addLine(`📦 Compact: ${evt.payload?.pre_token_count} → ${evt.payload?.post_token_count} tokens`, "info");
              break;
            }
            case "tool.progress": {
              // Optional: show progress
              break;
            }
          }
          break;
        }
        case "line": {
          const text = msg.payload?.text ?? "";
          if (text) {
            addLine(text, "line");
          }
          break;
        }
        case "approval_prompt": {
          const title = msg.payload?.title ?? "Approval Required";
          const body = msg.payload?.body ?? "";
          setDialogTitle(title);
          setDialogBody(body);
          setDialogVisible(true);
          setInputDisabled(true);
          break;
        }
      }
    });

    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      setInputDisabled(false);
      setLines((prev) => [
        ...prev,
        { id: lineIdRef.current++, text: "✓ Connected to engine", type: "info" },
      ]);
    };

    ws.onclose = () => {
      setConnected(false);
      setInputDisabled(true);
      setDialogVisible(false);
      setLines((prev) => [
        ...prev,
        { id: lineIdRef.current++, text: "✗ Disconnected", type: "error" },
      ]);
    };

    return () => {
      ws.close();
    };
  }, [port, addLine]);

  const handleSubmit = useCallback(
    (text: string) => {
      if (wsRef.current && connected) {
        currentDeltaRef.current = "";
        addLine(`❯ ${text}`, "line");
        sendInput(wsRef.current, text);
        setIsRunning(true);
        setIsThinking(true);
      }
    },
    [connected, addLine],
  );

  function processDeltaQueue() {
    const text = deltaQueueRef.current.shift();
    if (!text) {
      deltaProcessingRef.current = false;
      return;
    }
    const id = lineIdRef.current++;
    const content = currentDeltaRef.current;
    setLines((prev) => {
      const last = prev[prev.length - 1];
      if (last?.type === "delta") {
        return [...prev.slice(0, -1), { id, text: content, type: "delta" }];
      }
      return [...prev, { id, text: content, type: "delta" }];
    });
    // Process next delta after React has had a chance to render.
    setTimeout(processDeltaQueue, 0);
  }

  function finalizeResponse() {
    const full = currentDeltaRef.current;
    if (!full) return;

    // Replace the last delta line(s) with code-block-separated segments
    const segments: Array<{ text: string; type: "delta" | "code"; lang?: string }> = [];
        const blockRegex = /```(\w*)\n([\s\S]*?)```/g;;
    let lastIdx = 0;
    let match;
    while ((match = blockRegex.exec(full)) !== null) {
      if (match.index > lastIdx) {
        segments.push({ text: full.slice(lastIdx, match.index), type: "delta" });
      }
      segments.push({ text: match[2], type: "code", lang: match[1] || undefined });
      lastIdx = match.index + match[0].length;
    }
    if (lastIdx < full.length) {
      segments.push({ text: full.slice(lastIdx), type: "delta" });
    }

    if (segments.length === 1 && segments[0].type === "delta") {
      // No code blocks found; just update the last delta line with the plain text
      const id = lineIdRef.current++;
      setLines((prev) => {
        const last = prev[prev.length - 1];
        if (last?.type === "delta") {
          return [...prev.slice(0, -1), { id, text: segments[0].text, type: "delta" }];
        }
        return [...prev, { id, text: segments[0].text, type: "delta" }];
      });
      return;
    }

    // One or more code blocks found: replace trailing delta lines with segments
    setLines((prev) => {
      let i = prev.length - 1;
      while (i >= 0 && prev[i].type === "delta") i--;
      const base = prev.slice(0, i + 1);
      for (const seg of segments) {
        base.push({
          id: lineIdRef.current++,
          text: seg.text,
          type: seg.type === "code" ? "code" : "delta",
          codeLanguage: seg.lang,
        });
      }
      return base;
    });
  }

  return (
    <Box flexDirection="column" height="100%">
      <Box flexGrow={1} flexDirection="column">
        <Box marginBottom={1}>
          <Text bold>
            Claude Code {connected ? <Text color="green">● Connected</Text> : <Text color="red">● Disconnected</Text>}
          </Text>
        </Box>
        <Chat lines={lines} isThinking={isThinking} />
        <PermissionDialog
          visible={dialogVisible}
          title={dialogTitle}
          body={dialogBody}
          onApprove={handleApprove}
          onDeny={handleDeny}
        />
      </Box>
      <Box flexDirection="column">
        <StatusLine
          connected={connected}
          isRunning={isRunning}
          isThinking={isThinking && !dialogVisible}
          inputTokens={inputTokens}
          outputTokens={outputTokens}
        />
        <Box marginTop={1}>
          <Input onSubmit={handleSubmit} disabled={inputDisabled || dialogVisible} />
        </Box>
      </Box>
    </Box>
  );
}
