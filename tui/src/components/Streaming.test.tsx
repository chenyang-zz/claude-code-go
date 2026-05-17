import { describe, it, expect } from "bun:test";

/**
 * Verify that setTimeout(0) creates separate macrotasks.
 * If this test fails, React 19 batches setTimeout updates.
 */
describe("Streaming isolation", () => {
  it("setTimeout(0) creates sequential execution", async () => {
    const order: number[] = [];
    await new Promise<void>((resolve) => {
      setTimeout(() => {
        order.push(1);
        setTimeout(() => {
          order.push(2);
          setTimeout(() => {
            order.push(3);
            resolve();
          }, 0);
        }, 0);
      }, 0);
    });
    expect(order).toEqual([1, 2, 3]);
  });

  it("setTimeout preserves accumulator state", async () => {
    const results: string[] = [];
    const acc = { text: "" };
    const chunks = ["Hel", "lo ", "world"];

    await new Promise<void>((resolve) => {
      function process(i: number) {
        if (i >= chunks.length) {
          resolve();
          return;
        }
        acc.text += chunks[i];
        results.push(acc.text);
        setTimeout(() => process(i + 1), 0);
      }
      process(0);
    });

    expect(results).toEqual(["Hel", "Hello ", "Hello world"]);
  });
});
