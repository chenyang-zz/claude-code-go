export interface UsageLike {
  input_tokens?: number;
  output_tokens?: number;
  cache_creation_input_tokens?: number;
  cache_read_input_tokens?: number;
}

export interface UsagePayloadLike {
  turn_usage?: UsageLike;
  cumulative_usage?: UsageLike;
  usage?: UsageLike;
}

export interface TokenTotals {
  inputTokens: number;
  outputTokens: number;
}

export function usageForLine(payload: UsagePayloadLike): UsageLike {
  return payload.turn_usage ?? payload.usage ?? {};
}

export function usageForStatus(payload: UsagePayloadLike): UsageLike {
  return payload.cumulative_usage ?? payload.turn_usage ?? payload.usage ?? {};
}

function tokenTotalsFromUsage(usage: UsageLike): TokenTotals {
  return {
    inputTokens: usage.input_tokens ?? 0,
    outputTokens: usage.output_tokens ?? 0,
  };
}

export function statusUsageDelta(payload: UsagePayloadLike, previousTurnCumulative: TokenTotals): {
  delta: TokenTotals;
  nextTurnCumulative: TokenTotals;
} {
  if (payload.cumulative_usage) {
    const nextTurnCumulative = tokenTotalsFromUsage(payload.cumulative_usage);
    return {
      delta: {
        inputTokens: Math.max(0, nextTurnCumulative.inputTokens - previousTurnCumulative.inputTokens),
        outputTokens: Math.max(0, nextTurnCumulative.outputTokens - previousTurnCumulative.outputTokens),
      },
      nextTurnCumulative,
    };
  }

  const delta = tokenTotalsFromUsage(usageForStatus(payload));
  return {
    delta,
    nextTurnCumulative: {
      inputTokens: previousTurnCumulative.inputTokens + delta.inputTokens,
      outputTokens: previousTurnCumulative.outputTokens + delta.outputTokens,
    },
  };
}
