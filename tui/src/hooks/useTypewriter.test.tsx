import React, { useEffect } from "react";
import { describe, expect, it } from "bun:test";
import { render } from "ink-testing-library";
import { useTypewriter } from "./useTypewriter.js";
import type { TypewriterKind } from "../types/tui.js";

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

describe("useTypewriter", () => {
  it("drains queued text before reporting completion", async () => {
    const renders: string[] = [];
    let drainCompleted = false;

    function Harness() {
      const writer = useTypewriter({
        getTurnId: () => "turn-test",
        onDrainComplete: () => {
          drainCompleted = true;
        },
        onRender: (kind: TypewriterKind, visibleText: string) => {
          if (kind === "delta") renders.push(visibleText);
        },
      });

      useEffect(() => {
        writer.append("delta", "abcdefghijklmnopqr");
        writer.requestDrain("usage");
      }, [writer]);

      return null;
    }

    const app = render(<Harness />);
    expect(drainCompleted).toBe(false);

    await wait(70);

    expect(renders).toEqual(["abcdef", "abcdefghijkl", "abcdefghijklmnopqr"]);
    expect(drainCompleted).toBe(true);
    app.unmount();
  });
});
