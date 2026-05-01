# batch-213 阶段 1 源码对照与边界确认

## M1-1: TS 侧 circuit breaker / resilience 机制源码分析

### 结论

TS 侧**不存在** circuit breaker（熔断器）模式。Provider 级别的韧性完全依赖静态配置和同一 provider 内的 retry/fallback。

### 关键发现

1. **`src/services/api/withRetry.ts`** — TS 侧唯一的 retry 机制
   - 实现同 provider 内的指数退避重试（最大 10 次，`BASE_DELAY_MS=500` × 2^n + 25% 抖动，maxDelay=32s）
   - `FallbackTriggeredError` 仅在同一 provider 内触发模型切换（如 Opus → Sonnet），**不是跨 provider fallback**
   - 连续 3 次 529 错误（`MAX_529_RETRIES=3`）后触发 fallback model
   - 没有 provider 状态跟踪、没有熔断逻辑

2. **`src/utils/model/providers.ts`** — Provider 选择
   - 通过环境变量静态选择：`CLAUDE_CODE_USE_BEDROCK` / `USE_VERTEX` / `USE_FOUNDRY`
   - 运行时无 provider 切换能力
   - 无健康探测、无熔断器

3. **无 circuit breaker 相关代码**
   - 搜索 `circuit breaker`、`resilience pattern`、`breaker state`、`half-open`、`closed`、`open` 等关键词，无相关实现
   - 仅有 `pluginBlocklist.ts`、`bashProvider.ts` 等文件名称中包含 "block" 或 "provider" 但不涉及熔断语义

### 推断

Go 侧的 circuit breaker 是**新增能力**，无 TS 侧源码可直接对照。设计应参考业界经典模式（Martin Fowler / Microsoft resilience patterns），并与现有 Go 侧 retry/fallback/health 基础设施协同。

---

## M1-2: Go 侧现有 retry/fallback/health 集成点评估

### 已有基础设施（batch-67 + batch-96 + batch-212）

1. **`runtime/engine/retry.go`**
   - `isRetriableError(err)` — 错误分类（`model.RetryableError` 接口 + 关键词匹配）
   - `tryFallback(ctx, req, primaryErr)` — 两阶段 fallback：
     1. 先尝试 `FallbackModel` 在同一 `Client` 上
     2. 再遍历 `FallbackClients` 跨 provider
   - `shouldFallbackAfterAttempts(attempt)` — fallback 触发阈值控制
   - `RetryPolicy` — 指数退避（默认 3 次、500ms 基础 × 2^n + 25% 抖动、最大 30s）

2. **`runtime/engine/engine.go`** — `Runtime` struct 关键字段
   - `Client model.Client` — 主 provider client
   - `FallbackModel string` — 同 Client fallback model
   - `FallbackClients []model.Client` — 跨 provider fallback clients（batch-212 新增）
   - `FallbackAfterAttempts int` — fallback 触发阈值
   - `RetryPolicy RetryPolicy` — 退避策略

3. **`core/model/provider_health.go`** — HealthChecker（batch-212）
   - `ProviderHealth` 接口 — 主动探测
   - `HealthChecker` — 并发探测注册表
   - 与 circuit breaker 独立但可协同

### Circuit Breaker 集成点

| 集成位置 | 动作 | 文件 |
|---------|------|------|
| model.Client 包装 | 装饰器模式包装 Stream，调用前检查熔断状态，失败后上报 | `runtime/engine/circuit_breaker_client.go` |
| tryFallback 遍历 | 跳过已熔断的 FallbackClient | `runtime/engine/retry.go` |
| shouldFallbackAfterAttempts | 主 Client 熔断时提前触发 fallback | `runtime/engine/retry.go` |
| Runtime struct | 增加 CircuitBreakerRegistry | `runtime/engine/engine.go` |
| `/status` 命令 | 展示各 provider 熔断状态 | `services/commands/status.go` |

---

## M1-3: 边界确认与范围锁定

### Circuit Breaker 与 HealthChecker 的边界

- **HealthChecker**：主动探测，由 `/status` 命令或外部调用触发，回答 "provider 现在健康吗"
- **CircuitBreaker**：被动响应，由请求失败事件驱动，回答 "这个 provider 最近失败太多次，应该暂时屏蔽"
- 两者**独立**，不互相依赖。HealthChecker 不感知熔断状态，CircuitBreaker 不感知健康探测结果。

### 状态机参数默认值

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `FailureThreshold` | 5 | 连续 5 次失败触发熔断（Open） |
| `RecoveryTimeout` | 30s | 30 秒后从 Open 进入 Half-Open |
| `HalfOpenMaxRequests` | 1 | Half-Open 态只允许 1 个请求试探 |
| `HalfOpenSuccessThreshold` | 1 | 试探成功 1 次即恢复 Closed |

> 参考：TS 侧 `MAX_529_RETRIES=3` 触发 fallback，Go 侧 `DefaultRetryPolicy().MaxAttempts=3`。熔断阈值设为 5 略高于 retry 次数，确保 retry 耗尽后才触发熔断。

### 与 FallbackClients 的协同

1. **主 Client 熔断时**：`tryFallback` 应直接跳过主 Client，进入 fallback 流程
2. **FallbackClient 遍历**：每个 client 被尝试前检查其熔断状态，已熔断则跳过
3. **所有 client 都熔断时**：返回错误，不无限循环
4. **熔断状态恢复**：Half-Open 试探成功后，该 client 重新可用

---

## 验证结论

### M2-1: Circuit breaker 状态机与核心类型

- 产出：`internal/core/model/circuit_breaker.go`
- 验证方式：`go test ./internal/core/model/... -run TestCircuitBreaker -count=1 -v`
- 结果：7 个测试全部通过（默认值、Closed→Open、Open→HalfOpen→Closed、HalfOpen 失败重新打开、成功重置失败计数、HalfOpen 最大请求限制、并发安全）
- 状态：已验证

### M2-2: Provider client 熔断包装器

- 产出：`internal/runtime/engine/circuit_breaker_client.go`
- 验证方式：`go test ./internal/runtime/engine/... -run TestCircuitBreakerClient -count=1 -v`
- 结果：7 个测试全部通过（允许执行、记录成功、记录可重试失败、跳过不可重试失败、熔断时拒绝、Unwrap、Name）
- 状态：已验证

### M3-1 + M3-2: Engine retry 循环熔断检查注入 + Fallback 联动

- 产出：`internal/runtime/engine/retry.go` 修改
- 验证方式：`go test ./internal/runtime/engine/... -run TestTryFallback -count=1 -v`
- 结果：原有 7 个 fallback 测试零回归 + 新增 4 个集成测试全部通过（熔断错误触发 fallback、跳过已熔断 fallback client、主 client 熔断跳过 FallbackModel、所有 fallback client 都熔断时返回 nil）
- 状态：已验证

### M3-3: `/status` 命令熔断状态摘要

- 产出：`internal/services/commands/status.go` 修改
- 验证方式：`go build ./...` 零错误
- 状态：源码推断（StatusCommand 的 CircuitBreakers 字段由调用方填充，无运行时集成测试覆盖）

### M4-1: 全量回归

- 验证方式：`go build ./...` + `go test ./... -count=1`
- 结果：全量 86+ 包全部编译通过，零回归
- 状态：已验证

### 批次总结

- 新增文件：`internal/core/model/circuit_breaker.go`、`internal/runtime/engine/circuit_breaker_client.go`、`internal/core/model/circuit_breaker_test.go`、`internal/runtime/engine/circuit_breaker_client_test.go`、`internal/runtime/engine/circuit_breaker_integration_test.go`
- 修改文件：`internal/runtime/engine/retry.go`、`internal/services/commands/status.go`
- 新增测试：18 个（core/model 7 个 + engine client 7 个 + engine integration 4 个）
- 全量测试：86+ 包零回归

### 不纳入本批的能力

- 自适应阈值调整（基于历史成功率动态变更 threshold）
- 半开态概率放行（只放行部分请求试探）
- 多维度熔断（按模型、按区域、按错误类型细分）
- Provider 动态注册/注销
- 与外部监控系统的集成
- 浏览器/IDE 可视化展示
