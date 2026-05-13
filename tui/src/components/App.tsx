import React, { useState, useCallback, useEffect, useRef } from "react";
import { Box, Text, useApp } from "ink";
import { Chat, type Line } from "./Chat.js";
import { Input } from "./Input.js";
import { PermissionDialog } from "./PermissionDialog.js";
import { StatusLine } from "./StatusLine.js";
import { connectWS, sendInput, sendApproval, type WSMessage } from "../ws-client.js";
import { writeSync } from "fs";

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
  const lastDeltaFlushRef = useRef(0);
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
                // Write directly to terminal for streaming, bypassing React batching.
                writeSync(2, text);
                // Also batch-update React state at a throttled rate for proper rendering.
                const now = Date.now();
                if (now - (lastDeltaFlushRef.current ?? 0) > 50) {
                  lastDeltaFlushRef.current = now;
                  const id = lineIdRef.current++;
                  const content = currentDeltaRef.current;
                  setLines((prev) => {
                    const last = prev[prev.length - 1];
                    if (last?.type === "delta") {
                      return [...prev.slice(0, -1), { id, text: content, type: "delta" }];
                    }
                    return [...prev, { id, text: content, type: "delta" }];
                  });
                }
              }
              break;
            }
            case "thinking": {
              const thought = evt.payload?.thinking ?? evt.payload?.text ?? "";
              setIsRunning(true);
              setIsThinking(true);
              if (thought) {
                addLine(`Thinking: ${thought}`, "thinking");
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
              addLine(
                `📊 in=${inTk ?? "?"} out=${outTk ?? "?"} cache_c=${usage.cache_creation_input_tokens ?? "?"} cache_r=${usage.cache_read_input_tokens ?? "?"} stop=${evt.payload?.stop_reason ?? "?"}`,
                "info"
              );
              setIsRunning(false);
              setIsThinking(false);
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
