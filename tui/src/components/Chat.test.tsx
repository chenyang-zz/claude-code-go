import React from "react";
import { describe, it, expect } from "bun:test";
import { render } from "ink-testing-library";
import { Chat, type Line } from "./Chat.js";

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

describe("Chat", () => {
  it("renders info lines", () => {
    const lines: Line[] = [{ id: 1, text: "hello", type: "info" }];
    const { lastFrame } = render(<Chat lines={lines} isThinking={false} />);
    expect(lastFrame()).toContain("hello");
  });

  it("renders error lines", () => {
    const lines: Line[] = [{ id: 1, text: "oops", type: "error" }];
    const { lastFrame } = render(<Chat lines={lines} isThinking={false} />);
    expect(lastFrame()).toContain("oops");
  });

  it("renders tool lines in cyan", () => {
    const lines: Line[] = [{ id: 1, text: "Bash", type: "tool" }];
    const { lastFrame } = render(<Chat lines={lines} isThinking={false} />);
    expect(lastFrame()).toContain("Bash");
  });

  it("shows Thinking indicator", () => {
    const { lastFrame } = render(<Chat lines={[]} isThinking={true} />);
    expect(lastFrame()).toContain("Thinking");
  });

  it("animates the bottom Thinking indicator", async () => {
    const { lastFrame, unmount } = render(<Chat lines={[]} isThinking={true} />);
    expect(lastFrame()).toContain("⠋ Thinking.");

    await wait(130);

    expect(lastFrame()).toContain("⠙ Thinking.");
    unmount();
  });

  it("does not show Thinking indicator when not thinking", () => {
    const { lastFrame } = render(<Chat lines={[]} isThinking={false} />);
    expect(lastFrame()).not.toContain("Thinking");
  });
});
