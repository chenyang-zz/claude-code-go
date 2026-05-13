import React from "react";
import { render } from "ink";
import { App } from "./components/App.js";

// Parse --port argument
const portArg = process.argv.findIndex((a) => a === "--port");
const port = portArg >= 0 ? parseInt(process.argv[portArg + 1], 10) : 0;

if (!port || port <= 0 || port > 65535) {
  console.error("Usage: tui --port <port>");
  process.exit(1);
}

render(<App port={port} />);
