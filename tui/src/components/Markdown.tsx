import React from "react";
import { Box, Text } from "ink";

/**
 * Renders a single line of text with basic Markdown formatting:
 * - Inline code: `text`
 * - Bold: **text**
 * - Code blocks are handled as pre-rendered blocks by the caller
 */
export function MarkdownLine({ text }: { text: string }) {
  // Split by inline code patterns: `code`
  const parts: Array<{ type: "text" | "code" | "bold"; content: string }> = [];
  let remaining = text;

  while (remaining.length > 0) {
    // Check for inline code
    const codeMatch = remaining.match(/^([^`]*)`([^`]+)`(.*)/s);
    if (codeMatch) {
      // Text before code
      addBoldParts(parts, codeMatch[1]);
      parts.push({ type: "code", content: codeMatch[2] });
      remaining = codeMatch[3];
      continue;
    }

    // No more formatting
    addBoldParts(parts, remaining);
    break;
  }

  return (
    <Text>
      {parts.map((part, i) => {
        switch (part.type) {
          case "code":
            return (
              <Text key={i} color="cyanBright" backgroundColor="blackBright">
                {` ${part.content} `}
              </Text>
            );
          case "bold":
            return (
              <Text key={i} bold>
                {part.content}
              </Text>
            );
          default:
            return <Text key={i}>{part.content}</Text>;
        }
      })}
    </Text>
  );
}

function addBoldParts(
  parts: Array<{ type: "text" | "code" | "bold"; content: string }>,
  text: string,
) {
  // Split by bold pattern: **text**
  const boldRegex = /\*\*(.+?)\*\*/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = boldRegex.exec(text)) !== null) {
    if (match.index > lastIndex) {
      parts.push({ type: "text", content: text.slice(lastIndex, match.index) });
    }
    parts.push({ type: "bold", content: match[1] });
    lastIndex = match.index + match[0].length;
  }

  if (lastIndex < text.length) {
    parts.push({ type: "text", content: text.slice(lastIndex) });
  }
}

/**
 * Detects a code block opening fence in a line.
 */
export function isCodeBlockStart(line: string): boolean {
  return /^```/.test(line.trim());
}

/**
 * Renders a code block with a border box.
 */
export function CodeBlock({ code, language }: { code: string; language?: string }) {
  return (
    <Box
      borderStyle="round"
      borderColor="gray"
      flexDirection="column"
      paddingX={1}
      marginY={1}
    >
      {language ? (
        <Box>
          <Text dimColor>{language}</Text>
        </Box>
      ) : null}
      <Box flexDirection="column">
        {code.split("\n").map((line, i) => (
          <Text key={i}>{line}</Text>
        ))}
      </Box>
    </Box>
  );
}
