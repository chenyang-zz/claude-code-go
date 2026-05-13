import React, { useState, useCallback, useEffect, useRef } from "react";
import { Box, Text, useApp } from "ink";
import { Chat, type Line } from "./Chat.js";
import { Input } from "./Input.js";
import { connectWS, sendInput, type WSMessage } from "../ws-client.js";

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
  const [connected, setConnected] = useState(false);
  const [inputDisabled, setInputDisabled] = useState(true);
  const lineIdRef = useRef(2);
  const currentDeltaRef = useRef("");
  const wsRef = useRef<WebSocket | null>(null);

  const addLine = useCallback((text: string, type: Line["type"]) => {
    const id = lineIdRef.current++;
    setLines((prev) => [...prev, { id, text, type }]);
  }, []);

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
                currentDeltaRef.current += text;
                // Replace last delta line if it exists
                const id = lineIdRef.current;
                lineIdRef.current = id + 1;
                setLines((prev) => {
                  const last = prev[prev.length - 1];
                  if (last?.type === "delta") {
                    return [...prev.slice(0, -1), { id, text: currentDeltaRef.current, type: "delta" }];
                  }
                  return [...prev, { id, text: currentDeltaRef.current, type: "delta" }];
                });
              }
              break;
            }
            case "thinking": {
              const thought = evt.payload?.thinking ?? evt.payload?.text ?? "";
              setIsThinking(true);
              if (thought) {
                addLine(`Thinking: ${thought}`, "thinking");
              }
              break;
            }
            case "tool.call.started": {
              const name = evt.payload?.name ?? "unknown";
              addLine(`⚡ ${name}`, "tool");
              break;
            }
            case "tool.call.finished": {
              const name = evt.payload?.name ?? "unknown";
              const isErr = evt.payload?.is_error;
              addLine(`${isErr ? "✗" : "✓"} ${name}${isErr ? " (error)" : ""}`, isErr ? "error" : "tool");
              setIsThinking(false);
              break;
            }
            case "error": {
              addLine(`✗ ${evt.payload?.message ?? evt.payload?.error ?? "Error"}`, "error");
              setIsThinking(false);
              break;
            }
            case "usage": {
              const usage = evt.payload?.turn_usage ?? evt.payload?.usage ?? {};
              addLine(
                `📊 in=${usage.input_tokens ?? "?"} out=${usage.output_tokens ?? "?"} cache_c=${usage.cache_creation_input_tokens ?? "?"} cache_r=${usage.cache_read_input_tokens ?? "?"} stop=${evt.payload?.stop_reason ?? "?"}`,
                "info"
              );
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
      </Box>
      <Box marginTop={1}>
        <Input onSubmit={handleSubmit} disabled={inputDisabled} />
      </Box>
    </Box>
  );
}
