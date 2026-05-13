import React from "react";
import { Box, Text } from "ink";
import { ThinkingIndicator } from "./ThinkingIndicator.js";

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
  const isActivelyThinking = running && thinking;
  const stateColor = !connected ? "red" : isActivelyThinking ? "yellow" : running ? "cyan" : "green";
  const stateText = running ? "Running..." : "Idle";
  const stateIcon = !connected ? "●" : running ? "⚡" : "●";

  return (
    <Box
      borderStyle="round"
      borderColor={stateColor}
      justifyContent="space-between"
      paddingX={1}
      width="100%"
    >
      <Box>
        {isActivelyThinking ? (
          <ThinkingIndicator color={stateColor} />
        ) : (
          <>
            <Text color={stateColor}>{stateIcon}</Text>
            <Text dimColor> Status </Text>
            <Text color={stateColor}>{stateText}</Text>
          </>
        )}
      </Box>
      <Box>
        {inputTokens != null || outputTokens != null ? (
          <Text dimColor>
            tokens  in=<Text color="cyan">{inputTokens ?? "?"}</Text>  out=<Text color="cyan">{outputTokens ?? "?"}</Text>
          </Text>
        ) : (
          <Text dimColor>tokens  in=?  out=?</Text>
        )}
      </Box>
    </Box>
  );
}
