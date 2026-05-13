import React from "react";
import { describe, it, expect } from "bun:test";
import { render } from "ink-testing-library";
import { MarkdownLine, CodeBlock, isCodeBlockStart } from "./Markdown.js";

describe("MarkdownLine", () => {
  it("renders plain text", () => {
    const { lastFrame } = render(<MarkdownLine text="hello world" />);
    expect(lastFrame()).toContain("hello world");
  });

  it("renders inline code", () => {
    const { lastFrame } = render(<MarkdownLine text="run `npm install` to install" />);
    const frame = lastFrame();
    expect(frame).toContain("npm install");
  });

  it("renders bold text", () => {
    const { lastFrame } = render(<MarkdownLine text="this is **important**" />);
    const frame = lastFrame();
    expect(frame).toContain("important");
  });

  it("renders mixed formatting", () => {
    const { lastFrame } = render(<MarkdownLine text="use `git commit` to save **changes**" />);
    const frame = lastFrame();
    expect(frame).toContain("git commit");
    expect(frame).toContain("changes");
  });
});

describe("isCodeBlockStart", () => {
  it("detects code block start", () => {
    expect(isCodeBlockStart("```python")).toBe(true);
    expect(isCodeBlockStart("```")).toBe(true);
    expect(isCodeBlockStart("hello world")).toBe(false);
  });
});

describe("CodeBlock", () => {
  it("renders code with language", () => {
    const { lastFrame } = render(<CodeBlock code='print("hello")' language="python" />);
    const frame = lastFrame();
    expect(frame).toContain("python");
    expect(frame).toContain('print("hello")');
  });

  it("renders code without language", () => {
    const { lastFrame } = render(<CodeBlock code="some code" />);
    expect(lastFrame()).toContain("some code");
  });

  it("renders multi-line code", () => {
    const code = "line1\nline2\nline3";
    const { lastFrame } = render(<CodeBlock code={code} />);
    const frame = lastFrame();
    expect(frame).toContain("line1");
    expect(frame).toContain("line2");
    expect(frame).toContain("line3");
  });
});
