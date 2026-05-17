import React from "react";
import { describe, expect, it } from "bun:test";
import { render } from "ink-testing-library";
import { Input } from "./Input.js";

describe("Input", () => {
  it("renders prompt placeholder and send hint", () => {
    const { lastFrame } = render(<Input onSubmit={() => {}} disabled={false} />);
    const frame = lastFrame();
    expect(frame).toContain("Prompt");
    expect(frame).toContain("Enter to send");
    expect(frame).toContain("Ask Claude anything...");
  });

  it("renders disabled helper text", () => {
    const { lastFrame } = render(<Input onSubmit={() => {}} disabled={true} />);
    const frame = lastFrame();
    expect(frame).toContain("Waiting for current turn");
    expect(frame).not.toContain("Enter to send");
  });
});
