import React, { useRef, useEffect } from "react";
import { Box, Text } from "ink";
import { MarkdownLine } from "./Markdown.js";
import { type WSMessage } from "../ws-client.js";

export interface Line {
  id: number;
  text: string;
  type: "delta" | "thinking" | "tool" | "error" | "info" | "line";
}

export function Chat({ lines, isThinking }: { lines: Line[]; isThinking: boolean }) {
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    // Auto-scroll to bottom when new content arrives
  }, [lines]);

  return (
    <Box flexDirection="column" flexGrow={1}>
      {lines.map((line) => (
        <Box key={line.id}>
          {line.type === "thinking" && (
            <Text color="yellow">
              {isThinking ? "⠋" : "✓"} {line.text}
            </Text>
          )}
          {line.type === "tool" && (
            <Text color="cyan">{line.text}</Text>
          )}
          {line.type === "error" && (
            <Text color="red">{line.text}</Text>
          )}
          {line.type === "info" && (
            <Text color="gray">{line.text}</Text>
          )}
          {line.type === "delta" && (
            <MarkdownLine text={line.text} />
          )}
          {line.type === "line" && (
            <MarkdownLine text={line.text} />
          )}
        </Box>
      ))}
      {isThinking && (
        <Box>
          <Text color="yellow">⠋ Thinking...</Text>
        </Box>
      )}
    </Box>
  );
}
