import { useCallback, useRef, useState } from "react";
import type { Line, LineType } from "../types/tui.js";

export function useChatLines(port: number) {
  const nextLineIdRef = useRef(2);
  const [lines, setLines] = useState<Line[]>([
    { id: 0, text: "Claude Code TUI", type: "info" },
    { id: 1, text: `Connected on port ${port}`, type: "info" },
  ]);

  const addLine = useCallback((text: string, type: LineType) => {
    const id = nextLineIdRef.current++;
    setLines((prev) => [...prev, { id, text, type }]);
  }, []);

  const upsertLastLineOfType = useCallback((text: string, type: LineType) => {
    const id = nextLineIdRef.current++;
    setLines((prev) => {
      const last = prev[prev.length - 1];
      if (last?.type === type) {
        return [...prev.slice(0, -1), { id, text, type }];
      }
      return [...prev, { id, text, type }];
    });
  }, []);

  return {
    lines,
    addLine,
    upsertLastLineOfType,
  };
}
