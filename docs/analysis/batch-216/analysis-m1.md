# batch-216 阶段 1 分析：Bedrock + Foundry Provider 最小骨架

## §1 TS 侧 Bedrock/Foundry provider 配置与使用方式

### 1.1 Bedrock Provider（TS 侧）

文件：`src/utils/api.ts`、`src/services/api/client.ts`

**认证方式**：
- AWS Signature V4 签名，使用环境变量 `AWS_ACCESS_KEY_ID`、`AWS_SECRET_ACCESS_KEY`、`AWS_SESSION_TOKEN`
- 支持 `AWS_BEARER_TOKEN_BEDROCK` Bearer token 旁路（测试/代理场景）
- 支持 `CLAUDE_CODE_SKIP_BEDROCK_AUTH` 跳过认证

**区域配置**：
- 环境变量 `AWS_REGION` / `AWS_DEFAULT_REGION`，默认 `us-east-1`

**模型 ID 映射**：
- 第一方模型 ID（如 `claude-3-5-sonnet-20241022`）映射到 Bedrock 格式（如 `anthropic.claude-3-5-sonnet-20241022-v2:0`）
- 支持跨区域推理前缀 `us.anthropic.claude-...`

**端点格式**：
- `https://bedrock-runtime.{region}.amazonaws.com/model/{modelId}/invoke-with-response-stream`

### 1.2 Foundry Provider（TS 侧）

文件：`src/utils/api.ts`

**认证方式**：
- API Key 认证，使用 `api-key` HTTP header
- 配置来源：`ANTHROPIC_FOUNDRY_API_KEY` 环境变量或配置字段
- 支持 `CLAUDE_CODE_SKIP_FOUNDRY_AUTH` 跳过认证

**端点配置**：
- 优先级：显式 `baseURL` > `ANTHROPIC_FOUNDRY_BASE_URL` 环境变量 > `resource` 配置 > `ANTHROPIC_FOUNDRY_RESOURCE` 环境变量
- 默认格式：`https://{resource}.services.ai.azure.com`
- 消息端点：`{baseURL}/anthropic/v1/messages`

### 1.3 与 Vertex Provider 的对比

| 维度 | Vertex | Bedrock | Foundry |
|------|--------|---------|---------|
| 认证 | Google Cloud OAuth / 服务账号 | AWS Signature V4 | API Key |
| 区域 | `us-east5` 等 GCP 区域 | `us-east-1` 等 AWS 区域 | 由 resource/endpoint 决定 |
| 模型映射 | 直接透传 | Bedrock 格式映射 | 直接透传 |
| 端点格式 | `{region}-aiplatform...` | `bedrock-runtime.{region}.amazonaws.com` | `{resource}.services.ai.azure.com` |
| 跳过认证 | 支持 | 支持 | 支持 |

---

## §2 Go 侧现有 Provider 接入模式分析

### 2.1 文件结构（以 batch-209 Vertex 为参考）

```
internal/platform/api/anthropic/
├── vertex_auth.go      # 认证接口 + 默认实现
├── vertex_endpoint.go  # 区域选择 + 端点 URL 构建
├── bedrock_auth.go     # AWS Signature V4 认证
├── bedrock_endpoint.go # 区域选择 + 端点 URL + 模型映射
├── foundry_auth.go     # API Key 认证
├── foundry_endpoint.go # resource 选择 + 端点 URL 构建
└── client.go           # Provider 路径切换
```

### 2.2 标准模式

**认证模块**：
1. 定义 `XXXAuthenticator` 接口（`SignRequest` 或 `Authenticate` 方法）
2. 提供 `DefaultXXXAuthenticator` 默认实现
3. 提供 `noopXXXAuthenticator` 测试旁路
4. 提供 `newXXXAuthenticator` 工厂函数，按优先级选择实现

**端点模块**：
1. `resolveXXXRegion/BaseURL` 函数：环境变量/配置回退链
2. `buildXXXEndpoint` 函数：构造完整请求 URL
3. `toXXXModelID` 函数（Bedrock 特有）：模型 ID 映射

**Client 扩展**：
- `ClientOptions` 增加 provider 开关字段（`BedrockEnabled`、`FoundryEnabled`）
- `client` 内部增加 provider 状态字段
- `Stream` 方法中通过 switch 选择认证方式和端点构建逻辑

**Config 扩展**：
- `core/config/provider.go`：新增 `ProviderBedrock`、`ProviderFoundry` 枚举
- `core/config/config.go`：新增对应配置字段

**Bootstrap 集成**：
- `app.go` 中 `EngineAssembly` 创建阶段，按 `cfg.Provider` 值设置对应 `ClientOptions`

### 2.3 复用点

三个 Provider（Vertex、Bedrock、Foundry）共享以下模式：
- 相同的 `ClientOptions` / `client` 扩展方式
- 相同的 Bootstrap 集成模式（switch-case 分支）
- 相同的跳过认证测试旁路模式
- 相同的错误处理路径（batch-214 统一错误映射）

---

## §3 边界确认与范围锁定

### 3.1 纳入范围

batch-216 原计划纳入：
1. Bedrock AWS Signature V4 认证实现
2. Bedrock 端点构建与区域选择
3. Foundry API Key 认证实现
4. Foundry 端点构建与 resource 选择
5. `anthropic/client.go` Bedrock/Foundry 路径扩展
6. Config 扩展（Provider 枚举 + 配置字段）
7. Bootstrap 集成
8. 单元测试（认证、端点、配置解析）
9. 全量回归验证
10. 分析文档

### 3.2 实际状态

**上述全部内容已在 batch-210（Bedrock）和 batch-211（Foundry）中提前完成**：

- batch-210 已完成：
  - `bedrock_auth.go`（AWS Signature V4 完整实现）
  - `bedrock_endpoint.go`（区域选择 + 端点构建 + 10 个模型映射）
  - `client.go` Bedrock 路径扩展
  - `core/config` 扩展
  - `app.go` Bootstrap 集成
  - 21 个单元测试

- batch-211 已完成：
  - `foundry_auth.go`（API Key 认证接口 + 默认实现）
  - `foundry_endpoint.go`（resource 选择 + 端点构建）
  - `client.go` Foundry 路径扩展
  - `core/config` 扩展
  - `app.go` Bootstrap 集成
  - 19 个单元测试

### 3.3 验证结果

- `go build ./...`：零错误
- `go test ./... -count=1`：全部 86+ 包通过，零失败
- `go test ./internal/platform/api/anthropic/...`：通过（含 Bedrock/Foundry 全部测试）

### 3.4 结论

batch-216 无需重复实现。本次批次作为 batch-210 和 batch-211 的**合并收口批次**，统一记录两个 Provider 的分析文档并更新全局状态。

---

## §4 批次收口说明

由于 Bedrock Provider 最小骨架（batch-210）和 Foundry Provider 最小骨架（batch-211）在实际执行中已分别独立完成，batch-216 的规划范围已被前置批次覆盖。本次收口：

1. 补全跨批次分析文档（本文件）
2. 同步全局状态文件标记 batch-216 完成
3. 确认 backlog 中 Bedrock/Foundry 项已正确标记为已承接

Provider 矩阵当前状态：
- Anthropic（第一方）：已完成
- OpenAI：已完成（含 Responses API 最小骨架）
- Vertex：已完成（batch-209）
- Bedrock：已完成（batch-210）
- Foundry：已完成（batch-211）

剩余延后项：复杂负载均衡算法、Provider 动态热切换、自动凭据刷新策略、OpenAI Responses API 剩余高级能力。
