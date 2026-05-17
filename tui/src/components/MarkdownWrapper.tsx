import React from "react";
import { Text } from "ink";
import { setOptions, parse } from "marked";
import TerminalRenderer from "marked-terminal";

/**
 * Renders Markdown text using marked + marked-terminal.
 * Wraps ink-markdown's logic with proper ESM imports (ink-markdown uses
 * require("ink") which fails with Ink 7's ESM-only module).
 */
export default function Markdown({ children, ...options }: { children: string; [key: string]: any }) {
  setOptions({ renderer: new (TerminalRenderer as any)(options) as any });
  const rendered = String((parse as any)(children) ?? "").trim();
  const output = rendered.length > 0 ? rendered : children;
  return React.createElement(Text, null, output);
}
