import React, { useEffect, useState } from "react";
import { Text } from "ink";

const spinnerFrames = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];

interface ThinkingIndicatorProps {
  color?: string;
  dotIntervalMs?: number;
  label?: string;
  spinnerIntervalMs?: number;
}

export function ThinkingIndicator({
  color = "yellow",
  dotIntervalMs = 350,
  label = "Thinking",
  spinnerIntervalMs = 120,
}: ThinkingIndicatorProps) {
  const [dotCount, setDotCount] = useState(1);
  const [spinnerIndex, setSpinnerIndex] = useState(0);

  useEffect(() => {
    const timer = setInterval(() => {
      setDotCount((count) => (count >= 3 ? 1 : count + 1));
    }, dotIntervalMs);

    return () => clearInterval(timer);
  }, [dotIntervalMs]);

  useEffect(() => {
    const timer = setInterval(() => {
      setSpinnerIndex((index) => (index + 1) % spinnerFrames.length);
    }, spinnerIntervalMs);

    return () => clearInterval(timer);
  }, [spinnerIntervalMs]);

  return (
    <Text color={color}>
      {spinnerFrames[spinnerIndex]} {label}{".".repeat(dotCount)}
    </Text>
  );
}
