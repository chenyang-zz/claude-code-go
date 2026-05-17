import React from "react";
import { describe, it, expect } from "bun:test";
import { render } from "ink-testing-library";
import { StatusLine } from "./StatusLine.js";

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

describe("StatusLine", () => {
  it("shows Idle when not running", () => {
    const { lastFrame } = render(
      <StatusLine connected={true} isRunning={false} isThinking={false} />
    );
    const frame = lastFrame();
    expect(frame).toContain("Idle");
    expect(frame).not.toContain("Thinking");
    expect(frame).not.toContain("Running");
  });

  it("shows Thinking when running and thinking", () => {
    const { lastFrame } = render(
      <StatusLine connected={true} isRunning={true} isThinking={true} />
    );
    expect(lastFrame()).toContain("Thinking");
  });

  it("animates Thinking dots", async () => {
    const { lastFrame, unmount } = render(
      <StatusLine connected={true} isRunning={true} isThinking={true} />
    );
    expect(lastFrame()).toContain("Thinking.");

    await wait(380);

    expect(lastFrame()).toContain("Thinking..");
    unmount();
  });

  it("shows Running when running but not thinking", () => {
    const { lastFrame } = render(
      <StatusLine connected={true} isRunning={true} isThinking={false} />
    );
    expect(lastFrame()).toContain("Running");
  });

  it("shows connected status without Disconnected text", () => {
    const { lastFrame } = render(
      <StatusLine connected={true} isRunning={false} isThinking={false} />
    );
    expect(lastFrame()).toContain("Idle");
    expect(lastFrame()).not.toContain("Disconnected");
  });

  it("shows token counts when provided", () => {
    const { lastFrame } = render(
      <StatusLine
        connected={true}
        isRunning={false}
        isThinking={false}
        inputTokens={10}
        outputTokens={20}
      />
    );
    const frame = lastFrame();
    expect(frame).toContain("in=10");
    expect(frame).toContain("out=20");
  });
});
