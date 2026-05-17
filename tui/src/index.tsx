import React from "react";
import { render } from "ink";
import { App } from "./components/App.js";

// Parse --port argument (supports both --port PORT and --port=PORT)
const portArgIdx = process.argv.findIndex((a) => a === "--port" || a.startsWith("--port="));
let port = 0;
if (portArgIdx >= 0) {
  const raw = process.argv[portArgIdx];
  if (raw.startsWith("--port=")) {
    port = parseInt(raw.slice("--port=".length), 10);
  } else {
    port = parseInt(process.argv[portArgIdx + 1], 10);
  }
}

if (!port || port <= 0 || port > 65535) {
  console.error("Usage: tui --port <port>");
  process.exit(1);
}

render(<App port={port} />);
