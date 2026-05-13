import React from "react";
import { Box, Text } from "ink";

interface StatusLineProps {
  connected: boolean;
  isThinking: boolean;
  inputTokens?: number;
  outputTokens?: number;
}

export function StatusLine({
  connected,
  isThinking,
  inputTokens,
  outputTokens,
}: StatusLineProps) {
  return (
    <Box borderStyle="single" borderColor="gray" paddingX={1}>
      <Text>
        {connected ? (
          <Text color="green">●</Text>
        ) : (
          <Text color="red">●</Text>
        )}
        {" "}
        {isThinking ? (
          <Text color="yellow">⠋ Thinking...</Text>
        ) : (
          <Text dimColor>Idle</Text>
        )}
      </Text>
      {inputTokens != null || outputTokens != null ? (
        <Text dimColor>
          {"  │  "}
          in={inputTokens ?? "?"} out={outputTokens ?? "?"}
        </Text>
      ) : null}
    </Box>
  );
}
