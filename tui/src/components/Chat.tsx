import React from "react";
import { Box, Text } from "ink";
import Markdown from "./MarkdownWrapper.js";
import { CodeBlock } from "./Markdown.js";

export interface Line {
  id: number;
  text: string;
  type: "delta" | "thinking" | "tool" | "error" | "info" | "line" | "code";
  codeLanguage?: string;
}

export function Chat({ lines, isThinking }: { lines: Line[]; isThinking: boolean }) {

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
            <Markdown>{line.text}</Markdown>
          )}
          {line.type === "line" && (
            <Markdown>{line.text}</Markdown>
          )}
          {line.type === "code" && (
            <CodeBlock code={line.text} language={line.codeLanguage} />
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
