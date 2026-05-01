# batch-215 阶段 1 分析：TS/Go retry + fallback + circuit breaker 协调逻辑对照

## §1 TS 侧 `withRetry.ts` retry + fallback + circuit breaker 协调逻辑

文件：`src/services/api/withRetry.ts`

### 1.1 Retry 循环结构

- **默认 maxRetries**：`DEFAULT_MAX_RETRIES = 10`，可通过 `CLAUDE_CODE_MAX_RETRIES` 环境变量覆盖。
- **循环方式**：`for (let attempt = 1; attempt <= maxRetries + 1; attempt++)`
- **Backoff 计算**：`BASE_DELAY_MS = 500`，指数退避上限 32s，带 25% jitter。
- **Retry-After 尊重**：优先使用 HTTP `retry-after` header。

### 1.2 Fallback 触发时机

TS 侧的 fallback 有三条路径：

1. **连续 529 超载 fallback**（第 326-365 行）：
   - 跟踪 `consecutive529Errors` 计数器
   - 当连续 529 达到 `MAX_529_RETRIES = 3` 时
   - 若配置了 `fallbackModel`，抛出 `FallbackTriggeredError`
   - 注意：仅对非 custom Opus 模型或 `FALLBACK_FOR_ALL_PRIMARY_MODELS` 环境变量开启时生效

2. **Fast mode 429/529 fallback**（第 267-305 行）：
   - 当 fast mode 激活时遇到 429/529
   - 短 retry-after（< 20s）：等待后重试，保持 fast mode
   - 长 retry-after 或未知：触发 cooldown，切换到标准速度模型
   - 如果是 overage 不可用：永久禁用 fast mode

3. **Max retries 耗尽**：
   - 抛出 `CannotRetryError`
   - TS 侧没有显式的"最后尝试 fallback"逻辑，fallback 由调用方处理

### 1.3 `shouldRetry` 决策逻辑

`shouldRetry(error)` 决定是否对某个 `APIError` 进行重试：

| 条件 | 行为 |
|------|------|
| Mock rate limit | 不 retry |
| Persistent mode + 429/529 | 总是 retry |
| CCR remote mode + 401/403 | 总是 retry |
| Overloaded error (type:overloaded_error) | Retry |
| Max tokens overflow | Retry（并调整参数） |
| x-should-retry: true | Retry（非订阅用户或企业用户） |
| x-should-retry: false | 不 retry（Ant 员工对 5xx 除外） |
| APIConnectionError | Retry |
| 408 / 409 | Retry |
| 429 | Retry（非 ClaudeAI 订阅用户） |
| 401 / 403 token revoked | Retry（刷新凭证） |
| 5xx | Retry |

### 1.4 Circuit Breaker

TS 侧**没有显式的 circuit breaker 机制**。熔断逻辑在 batch-214 中仅在 Go 侧实现。

### 1.5 关键差异点（对比 Go 侧）

- TS 侧 retry 次数默认 10 次，Go 侧默认 3 次
- TS 侧有 529 连续计数 fallback，Go 侧没有对应机制
- TS 侧有 fast mode / persistent retry 特殊逻辑，Go 侧没有
- TS 侧 401/403 会触发凭证刷新，Go 侧 batch-214 已部分实现（auth error 返回 ProviderErrorAuthError）

---

## §2 Go 侧 `streamAndConsume` retry 循环现状分析

文件：`claude-code-go/internal/runtime/engine/engine.go:1233-1343`

### 2.1 循环结构

```go
for attempt := 0; attempt <= retries; attempt++ {
    modelStream, connErr := e.Client.Stream(ctx, req)
    if connErr != nil {
        // 连接错误路径
    }
    result, streamErr := e.consumeModelStream(...)
    if streamErr != nil {
        // 流错误路径
    }
}
// retries 耗尽 — try fallback
```

### 2.2 连接错误路径（connErr）

1. `lastErr = connErr`
2. `if !isRetriableError(connErr) { break }`
3. `if e.shouldFallbackAfterAttempts(attempt + 1) { tryFallback }`
4. `if attempt < retries { backoff + retry }`

### 2.3 流错误路径（streamErr）

1. `if !isRetriableError(streamErr) { return error }`
2. `lastErr = streamErr`
3. `if e.shouldFallbackAfterAttempts(attempt + 1) { tryFallback }`
4. `if attempt < retries { backoff + retry }`

### 2.4 `isRetriableError` 现状

文件：`claude-code-go/internal/runtime/engine/retry.go:63-98`

当前逻辑：
1. 检查 `model.RetryableError` 接口 → 调用 `IsRetryable()`
2. 网络错误
3. HTTP 状态码：529/500/502/503/504/429/408
4. 关键词：overloaded, rate_limit, timeout
5. OpenAI 特定消息

**问题**：`CircuitBreakerOpenError` 没有实现 `RetryableError` 接口，其错误消息 `"circuit breaker open for provider %s"` 也不匹配任何 HTTP 状态码或关键词。因此 `isRetriableError` 对 breaker open 返回 **false**。

### 2.5 `tryFallback` 现状

文件：`claude-code-go/internal/runtime/engine/retry.go:140-189`

```go
func (e *Runtime) tryFallback(ctx context.Context, req model.Request, primaryErr error) *fallbackResult {
    var cbErr *model.CircuitBreakerOpenError
    if !isRetriableError(primaryErr) && !errors.As(primaryErr, &cbErr) {
        return nil
    }
    // 1. 尝试同 provider fallback model
    // 2. 尝试跨 provider fallback clients
}
```

`tryFallback` 已经单独处理了 `CircuitBreakerOpenError`（通过 `errors.As`），所以 breaker open 时**最终能 fallback**。

### 2.6 当前 breaker open / quota exceeded 的代码路径

#### CircuitBreakerOpenError 路径

1. `Client.Stream` 返回 `CircuitBreakerOpenError`
2. `isRetriableError` 返回 false
3. 连接错误路径：break 出循环
4. 循环结束后：`tryFallback` 检查到 `CircuitBreakerOpenError`，执行 fallback
5. **结论**：能 fallback，但不是通过 retry 循环内部的 `shouldFallbackAfterAttempts`，而是 break 后直接 fallback。

#### ProviderErrorQuotaExceeded 路径

1. `Client.Stream` 返回 `ProviderError{Kind: ProviderErrorQuotaExceeded}`
2. `isRetriableError` 中 `errors.As` 匹配到 `RetryableError` → 调用 `IsRetryable()` 返回 **false**
3. 继续检查：网络错误？否。HTTP 状态码？可能（如果消息含 429）。关键词？可能。
4. 如果消息不含 429/关键词，`isRetriableError` 返回 false
5. 连接错误路径：break 出循环 → `tryFallback`（`isRetriableError` 返回 false 且不是 `CircuitBreakerOpenError`，所以 `tryFallback` 返回 nil）→ 返回错误
6. **结论**：quota exceeded **不能 fallback**，直接报错。

### 2.7 缺口总结

| 缺口 | 当前行为 | 期望行为 |
|------|----------|----------|
| Breaker open | break 后直接 fallback | 立即 fallback（一次 Stream 调用都不浪费） |
| Quota exceeded | break 后直接报错 | 立即触发跨 provider fallback |
| isRetriableError 语义 | 不识别 CircuitBreakerOpenError | 识别为 retriable（使 engine 统一处理） |
| Retry 事件 | 仅有 Attempt/MaxAttempts/BackoffMs/Error | 增加 ErrorKind 字段，便于可观测性 |

---

## §3 边界确认与范围锁定

### 3.1 纳入范围

1. `isRetriableError` 识别 `CircuitBreakerOpenError` 返回 true
2. `streamAndConsume` 遇到 `CircuitBreakerOpenError` 时立即 fallback（不进入 retry 循环）
3. `streamAndConsume` 遇到 `ProviderErrorQuotaExceeded` 时立即 fallback（不 retry）
4. `RetryAttemptedPayload` 增加 `ErrorKind` 字段
5. `/status` 增加 retry/fallback 统计摘要
6. 对应测试（6+ 新增测试）

### 3.2 排除范围

- 不修改 circuit breaker 本身的阈值或半开态逻辑（batch-214 已完成）
- 不修改 provider error mapper 的分类逻辑（batch-214 已完成）
- 不引入 529 连续计数 fallback（TS 侧特性，本批次不跟进）
- 不引入 fast mode / persistent retry 逻辑（TS 侧特性，本批次不跟进）
- 不修改凭证刷新策略

### 3.3 关键文件变更清单

| 文件 | 变更类型 |
|------|----------|
| `internal/runtime/engine/retry.go` | `isRetriableError` 增加 CircuitBreakerOpenError 识别；新增 `isImmediateFallbackError`、`extractErrorKind`；`tryFallback` 支持 quota exceeded |
| `internal/runtime/engine/engine.go` | `streamAndConsume` 增加 breaker open / quota exceeded 立即 fallback；`runFallback` 增加 fallback 统计；retry 事件填充 `ErrorKind` |
| `internal/core/event/types.go` | `RetryAttemptedPayload` 增加 `ErrorKind` 字段 |
| `internal/core/model/runtime_stats.go` | 新增 `RuntimeStats` 结构体（线程安全计数器） |
| `internal/core/model/circuit_breaker.go` | `CircuitBreaker` 增加 `tripCount` 和 `TripCount()` 方法 |
| `internal/services/commands/status.go` | 增加 `StatsCollector` 字段和 `runtimeStatsStatus` 展示 |
| `internal/app/bootstrap/app.go` | `EngineAssembly` 增加 `StatsCollector`；`DefaultEngineFactory` 创建并注入 StatsCollector；`newCommandRegistry` 接收 StatsCollector |
| `internal/runtime/engine/*_test.go` | 新增 6 个测试 |

---

## §4 实现记录

### 4.1 M2-1：`isRetriableError` 识别 `CircuitBreakerOpenError`

在 `retry.go:74-79` 增加：
```go
var cbErr *model.CircuitBreakerOpenError
if errors.As(err, &cbErr) {
    return true
}
```

这样 `CircuitBreakerOpenError` 被统一识别为 retriable，engine 可以统一处理。

### 4.2 M2-2：breaker open 时立即触发 fallback

在 `engine.go:1255-1261` 增加 `isImmediateFallbackError` 检查：
```go
if isImmediateFallbackError(connErr) {
    if fb := e.tryFallback(ctx, req, lastErr); fb != nil {
        return e.runFallback(ctx, req, fb, streamingExec, out)
    }
    return streamResult{}, fmt.Errorf("stream failed, immediate fallback not available: %w", lastErr)
}
```

连接错误和流错误路径都增加了此检查。当 breaker open 时，不进入 retry 循环，立即尝试 fallback。

### 4.3 M2-3：quota exceeded 时立即触发 fallback

`isImmediateFallbackError`（`retry.go:240-252`）同时检查 quota exceeded：
```go
var pErr *model.ProviderError
if errors.As(err, &pErr) && pErr.Kind == model.ProviderErrorQuotaExceeded {
    return true
}
```

同时修改 `tryFallback`（`retry.go:148`）允许 quota exceeded 触发 fallback：
```go
isQuotaExceeded := errors.As(primaryErr, &pErr) && pErr.Kind == model.ProviderErrorQuotaExceeded
if !isRetriableError(primaryErr) && !errors.As(primaryErr, &cbErr) && !isQuotaExceeded {
    return nil
}
```

### 4.4 M2-4：Retry 事件可观测性增强

`RetryAttemptedPayload` 增加 `ErrorKind string` 字段（`types.go:70`）。

新增 `extractErrorKind` 辅助函数（`retry.go:207-228`），从 `ProviderError`、`CircuitBreakerOpenError`、网络错误、HTTP 状态码中提取分类字符串。

`streamAndConsume` 在发送 `TypeRetryAttempted` 事件时填充 `ErrorKind`。

### 4.5 M2-5：新增测试

| 测试 | 覆盖内容 |
|------|----------|
| `TestIsRetriableError_CircuitBreakerOpenError` | `isRetriableError` 对 breaker open 返回 true |
| `TestIsImmediateFallbackError` | breaker open / quota exceeded 返回 true；其他返回 false |
| `TestExtractErrorKind` | 各类错误的 kind 提取正确性 |
| `TestRuntimeRun_CircuitBreakerOpenImmediateFallback` | breaker open 立即 fallback，不 retry |
| `TestRuntimeRun_QuotaExceededImmediateFallback` | quota exceeded 立即 fallback，不 retry |
| `TestRuntimeRunRetryEmitsEvents_WithErrorKind` | retry 事件包含 ErrorKind |

### 4.6 M3-2：`/status` retry/fallback 统计增强

新增 `RuntimeStats`（`core/model/runtime_stats.go`）：使用 `sync/atomic` 实现线程安全的 retry/fallback/cb trip 计数器。

`CircuitBreaker` 增加 `tripCount` 字段（`core/model/circuit_breaker.go`），在 `RecordFailure` 触发 Open 时递增。

`StatusCommand` 增加 `StatsCollector *model.RuntimeStats` 字段，在 `Execute` 中调用 `runtimeStatsStatus` 展示：
```
- Runtime resilience: retries=N, fallbacks=M, cb_trips=K
```

`DefaultEngineFactory` 创建 `StatsCollector`，注入 engine，并通过 `EngineAssembly` 返回。`newCommandRegistry` 接收 StatsCollector 并注入 StatusCommand。

### 4.7 M4-1：验证结果

- `go build ./...`：零错误
- `go test ./internal/runtime/engine/... -count=1`：全部通过（含 6 个新增测试，零回归）
- `go test ./internal/app/bootstrap/... -count=1`：通过
- `go test ./... -count=1`：86+ 包中除 `internal/services/tools/grep`（环境缺少 `rg` 可执行文件）外全部通过
