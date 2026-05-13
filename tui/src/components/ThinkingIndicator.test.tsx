import React from "react";
import { describe, expect, it } from "bun:test";
import { render } from "ink-testing-library";
import { ThinkingIndicator } from "./ThinkingIndicator.js";

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

describe("ThinkingIndicator", () => {
  it("animates spinner and dots", async () => {
    const { lastFrame, unmount } = render(
      <ThinkingIndicator spinnerIntervalMs={20} dotIntervalMs={40} />
    );

    expect(lastFrame()).toContain("⠋ Thinking.");

    await wait(25);
    expect(lastFrame()).toContain("⠙ Thinking.");

    await wait(25);
    expect(lastFrame()).toContain("Thinking..");

    unmount();
  });
});
