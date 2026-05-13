import { describe, expect, it } from "bun:test";
import { statusUsageDelta, usageForLine, usageForStatus } from "./usage.js";

describe("usage selectors", () => {
  it("uses turn usage for usage line and cumulative usage for status", () => {
    const payload = {
      turn_usage: { input_tokens: 10, output_tokens: 20 },
      cumulative_usage: { input_tokens: 100, output_tokens: 200 },
    };

    expect(usageForLine(payload)).toEqual({ input_tokens: 10, output_tokens: 20 });
    expect(usageForStatus(payload)).toEqual({ input_tokens: 100, output_tokens: 200 });
  });

  it("falls back to turn usage when cumulative usage is unavailable", () => {
    const payload = {
      turn_usage: { input_tokens: 10, output_tokens: 20 },
    };

    expect(usageForStatus(payload)).toEqual({ input_tokens: 10, output_tokens: 20 });
  });

  it("computes only the new cumulative usage within a turn", () => {
    const first = statusUsageDelta({
      cumulative_usage: { input_tokens: 100, output_tokens: 20 },
    }, { inputTokens: 0, outputTokens: 0 });
    expect(first.delta).toEqual({ inputTokens: 100, outputTokens: 20 });
    expect(first.nextTurnCumulative).toEqual({ inputTokens: 100, outputTokens: 20 });

    const second = statusUsageDelta({
      cumulative_usage: { input_tokens: 130, output_tokens: 25 },
    }, first.nextTurnCumulative);
    expect(second.delta).toEqual({ inputTokens: 30, outputTokens: 5 });
    expect(second.nextTurnCumulative).toEqual({ inputTokens: 130, outputTokens: 25 });
  });

  it("treats turn usage as a direct delta when cumulative usage is unavailable", () => {
    const result = statusUsageDelta({
      turn_usage: { input_tokens: 10, output_tokens: 20 },
    }, { inputTokens: 100, outputTokens: 200 });

    expect(result.delta).toEqual({ inputTokens: 10, outputTokens: 20 });
    expect(result.nextTurnCumulative).toEqual({ inputTokens: 110, outputTokens: 220 });
  });
});
