import React, { useState, useCallback } from "react";
import { Box, Text, useInput } from "ink";

export function Input({
  onSubmit,
  disabled,
}: {
  onSubmit: (text: string) => void;
  disabled: boolean;
}) {
  const [value, setValue] = useState("");
  const borderColor = disabled ? "gray" : value ? "cyan" : "blue";
  const labelColor = disabled ? "gray" : "cyan";
  const helperText = disabled ? "Waiting for current turn" : "Enter to send";

  useInput(
    (input, key) => {
      if (disabled) return;

      if (key.return) {
        const trimmed = value.trim();
        if (trimmed) {
          onSubmit(trimmed);
          setValue("");
        }
        return;
      }

      if (key.backspace || key.delete) {
        setValue((v) => v.slice(0, -1));
        return;
      }

      // Regular character input（支持中文等多字符输入法提交）
      if (input && !key.ctrl && !key.meta) {
        setValue((v) => v + input);
      }
    },
  );

  return (
    <Box
      borderStyle="round"
      borderColor={borderColor}
      flexDirection="column"
      paddingX={1}
      width="100%"
    >
      <Box>
        <Text bold color={labelColor}>✦ Prompt</Text>
        <Text dimColor>  {helperText}</Text>
      </Box>
      <Box>
        <Text bold color={disabled ? "gray" : "green"}>❯ </Text>
        {value ? (
          <Text>{value}</Text>
        ) : (
          <Text dimColor>Ask Claude anything...</Text>
        )}
        {!disabled && <Text color="cyan">▌</Text>}
      </Box>
    </Box>
  );
}
