import React from "react";
import { Box, Text } from "ink";

interface StatusLineProps {
  connected: boolean;
  isRunning: boolean;
  isThinking: boolean;
  inputTokens?: number;
  outputTokens?: number;
}

export function StatusLine({
  connected,
  isRunning: running,
  isThinking: thinking,
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
        {running && thinking ? (
          <Text color="yellow">⠋ Thinking...</Text>
        ) : running ? (
          <Text color="cyan">⚡ Running...</Text>
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
