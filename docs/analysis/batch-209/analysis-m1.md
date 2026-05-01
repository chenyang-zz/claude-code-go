# batch-209 分析文档：Vertex Provider 最小骨架

## §1 TS 侧 Vertex Provider 源码分析（已完成）

### 1.1 TS 侧 Provider 架构总览

TS 侧支持的 API provider（`src/utils/model/providers.ts`）：

| Provider | 环境变量 | 说明 |
|----------|----------|------|
| `firstParty` | 默认 | Anthropic 官方 API |
| `bedrock` | `CLAUDE_CODE_USE_BEDROCK` | AWS Bedrock 上的 Claude |
| `vertex` | `CLAUDE_CODE_USE_VERTEX` | Google Cloud Vertex AI 上的 Claude |
| `foundry` | `CLAUDE_CODE_USE_FOUNDRY` | Azure AI Foundry 上的 Claude |

**关键发现**：TS 侧不存在独立的 Gemini API provider。所有 3P provider（bedrock/vertex/foundry）本质都是通过各自云平台的认证方式访问 Claude 模型，消息格式与 Anthropic Messages API 相同。

### 1.2 TS 侧 Vertex 实现源码（`src/services/api/client.ts:221-297`）

```typescript
// 1. GCP 凭据刷新
if (!isEnvTruthy(process.env.CLAUDE_CODE_SKIP_VERTEX_AUTH)) {
  await refreshGcpCredentialsIfNeeded()
}

// 2. 动态导入 AnthropicVertex SDK + google-auth-library
const [{ AnthropicVertex }, { GoogleAuth }] = await Promise.all([
  import('@anthropic-ai/vertex-sdk'),
  import('google-auth-library'),
])

// 3. 构建 GoogleAuth 实例
const googleAuth = isEnvTruthy(process.env.CLAUDE_CODE_SKIP_VERTEX_AUTH)
  ? ({ getClient: () => ({ getRequestHeaders: () => ({}) }) } as unknown as GoogleAuth)
  : new GoogleAuth({
      scopes: ['https://www.googleapis.com/auth/cloud-platform'],
      // 仅在无 project env var 且无 keyfile 时使用 ANTHROPIC_VERTEX_PROJECT_ID
      ...(hasProjectEnvVar || hasKeyFile ? {} : {
        projectId: process.env.ANTHROPIC_VERTEX_PROJECT_ID,
      }),
    })

// 4. 构建 Vertex 参数
const vertexArgs = {
  ...ARGS,  // 复用通用参数（headers、maxRetries、timeout 等）
  region: getVertexRegionForModel(model),
  googleAuth,
  ...(isDebugToStdErr() && { logger: createStderrLogger() }),
}

return new AnthropicVertex(vertexArgs) as unknown as Anthropic
```

### 1.3 TS 侧区域选择逻辑（`src/utils/envUtils.ts:103-183`）

```typescript
// 默认 region：CLOUD_ML_REGION || 'us-east5'
export function getDefaultVertexRegion(): string {
  return process.env.CLOUD_ML_REGION || 'us-east5'
}

// 模型前缀 → env var 覆盖映射
const VERTEX_REGION_OVERRIDES = [
  ['claude-haiku-4-5', 'VERTEX_REGION_CLAUDE_HAIKU_4_5'],
  ['claude-3-5-haiku', 'VERTEX_REGION_CLAUDE_3_5_HAIKU'],
  ['claude-3-5-sonnet', 'VERTEX_REGION_CLAUDE_3_5_SONNET'],
  ['claude-3-7-sonnet', 'VERTEX_REGION_CLAUDE_3_7_SONNET'],
  ['claude-opus-4-1', 'VERTEX_REGION_CLAUDE_4_1_OPUS'],
  ['claude-opus-4', 'VERTEX_REGION_CLAUDE_4_0_OPUS'],
  ['claude-sonnet-4-6', 'VERTEX_REGION_CLAUDE_4_6_SONNET'],
  ['claude-sonnet-4-5', 'VERTEX_REGION_CLAUDE_4_5_SONNET'],
  ['claude-sonnet-4', 'VERTEX_REGION_CLAUDE_4_0_SONNET'],
]

export function getVertexRegionForModel(model: string | undefined): string {
  if (model) {
    const match = VERTEX_REGION_OVERRIDES.find(([prefix]) => model.startsWith(prefix))
    if (match) {
      return process.env[match[1]] || getDefaultVertexRegion()
    }
  }
  return getDefaultVertexRegion()
}
```

### 1.4 TS 侧模型 ID 映射（`src/utils/model/configs.ts`）

Vertex 模型 ID 格式：`claude-3-7-sonnet@20250219`（`model-id@YYYYmmdd`），与 Anthropic 官方模型一一对应。

### 1.5 环境变量清单

| 环境变量 | 用途 | 必填 |
|----------|------|------|
| `CLAUDE_CODE_USE_VERTEX` | 启用 Vertex provider | 是 |
| `ANTHROPIC_VERTEX_PROJECT_ID` | GCP project ID fallback | 否（有替代发现机制） |
| `VERTEX_REGION_CLAUDE_*` | 模型级 region 覆盖 | 否 |
| `CLOUD_ML_REGION` | 全局默认 region | 否（默认 `us-east5`） |
| `CLAUDE_CODE_SKIP_VERTEX_AUTH` | 跳过认证（测试/代理） | 否 |
| `GCLOUD_PROJECT` / `GOOGLE_CLOUD_PROJECT` | GCP project 发现 | 否 |
| `GOOGLE_APPLICATION_CREDENTIALS` | GCP 服务账号凭据 | 否 |

## §2 Go 侧 Provider 架构现状评估

### 2.1 Provider 常量定义（`internal/core/config/provider.go`）

当前定义：
- `ProviderAnthropic = "anthropic"`
- `ProviderOpenAICompatible = "openai-compatible"`
- `ProviderGLM = "glm"`

**缺口**：缺少 `ProviderVertex`。

### 2.2 Bootstrap 装配（`internal/app/bootstrap/app.go:848-958`）

`DefaultEngineFactory` 使用 `switch NormalizeProvider(cfg.Provider)` 分流：
- `ProviderAnthropic` → `anthropic.NewClient(anthropic.Config{...})`
- `ProviderOpenAICompatible` / `ProviderGLM` → `openai.NewClient(openai.Config{...})`

**缺口**：缺少 Vertex 分支。

### 2.3 anthropic.Client 结构（`internal/platform/api/anthropic/client.go`）

```go
type Config struct {
    APIKey       string
    AuthToken    string
    BaseURL      string
    HTTPClient   *http.Client
    IsFirstParty bool
}
```

`Stream` 方法特征：
- 请求路径：`POST {baseURL}/v1/messages`
- 认证头：`x-api-key` 或 `authorization: Bearer`
- 版本头：`anthropic-version: 2023-06-01`
- 请求体：通过 `buildMessagesRequest` 构建

### 2.4 复用策略确认

Vertex 底层使用 Anthropic Messages API 协议，因此 Go 侧实现策略：

1. **复用 `anthropic.Client`** — 消息格式、mapper、stream 解析完全相同
2. **扩展 `anthropic.Config`** — 添加 Vertex 专用字段
3. **新增 `vertex_auth.go`** — Google Cloud OAuth token 获取接口
4. **新增 `vertex_endpoint.go`** — 端点 URL 构建 + 区域选择
5. **修改 `Stream` 方法** — 当 Vertex 模式时切换 base URL 和认证方式

**不需要**独立 `internal/platform/api/vertex/` 包，因为：
- 消息格式与 Anthropic 完全相同
- 流式解析与 Anthropic 完全相同
- 差异仅在认证层和端点 URL

## §3 边界确认与范围锁定

### 3.1 纳入范围

| 能力 | 理由 |
|------|------|
| Google Cloud OAuth token 获取 | 核心认证差异，必须有 |
| `ANTHROPIC_VERTEX_PROJECT_ID` fallback | TS 侧有完整逻辑，最小实现需包含 |
| 区域选择（模型前缀 → env var → 默认） | TS 侧有完整逻辑，最小实现需包含 |
| Vertex AI 端点 URL 构建 | 核心差异，必须有 |
| `ProviderVertex` 常量 + Bootstrap 分支 | 运行时接入，必须有 |
| 模型 ID 映射（vertex 格式） | 配置层接入，必须有 |

### 3.2 排除范围

| 能力 | 理由 |
|------|------|
| GCP 凭据自动刷新定时器 | TS 侧 `refreshGcpCredentialsIfNeeded()` 逻辑复杂，本批最小实现跳过；依赖 Google Cloud 默认凭据自动刷新 |
| Vertex SDK 实例缓存 | TS 侧 TODO 注释提到的 GoogleAuth 缓存优化，属于性能优化而非功能刚需 |
| Bedrock / Foundry provider | 独立 SDK，独立批次 |
| 跨 provider 负载均衡 | 需多 provider 稳定后实施 |
| `isFirstParty` beta header 适配 | Vertex 不是 first-party，task-budgets 等 beta header 在 Vertex 路径中自然跳过 |
