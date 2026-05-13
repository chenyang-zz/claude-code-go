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
    <Box>
      <Text bold>❯ </Text>
      <Text>{value}</Text>
      {!disabled && <Text dimColor>█</Text>}
    </Box>
  );
}
