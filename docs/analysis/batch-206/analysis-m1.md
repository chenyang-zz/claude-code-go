# batch-206 M1 分析：变量替换全链路源码对照与边界确认

## §1 TS 侧变量替换全链路

### 1.1 substitutePluginVariables

- **位置**：`src/utils/plugins/pluginOptionsStorage.ts:326-344`
- **签名**：`substitutePluginVariables(value: string, plugin: { path: string; source?: string }): string`
- **行为**：
  - 替换 `${CLAUDE_PLUGIN_ROOT}` → `plugin.path`（Windows 标准化为 `/`）
  - 替换 `${CLAUDE_PLUGIN_DATA}` → `getPluginDataDir(plugin.source)`（仅当 `plugin.source` 存在）
  - 使用函数形式 `.replace()` 避免 `$` 模式解释问题
- **使用点**：
  | 文件 | 行号 | 场景 |
  |------|------|------|
  | `loadPluginCommands.ts` | 241-258 | `allowed-tools` frontmatter 字段替换 |
  | `loadPluginCommands.ts` | 339-343 | `getPromptForCommand` 内容替换 |
  | `mcpPluginIntegration.ts` | 477 | `resolvePluginMcpEnvironment` 中替换 command/args/env |
  | `lspPluginIntegration.ts` | 239 | `resolvePluginLspEnvironment` 中替换 command/args/env |

### 1.2 substituteArguments

- **位置**：`src/utils/argumentSubstitution.ts:94-145`
- **签名**：`substituteArguments(content, args, appendIfNoPlaceholder, argumentNames)`
- **支持的占位符**：
  - `$ARGUMENTS` → 完整参数字符串
  - `$ARGUMENTS[n]` → 第 n 个参数（0-based）
  - `$n` → 简写索引参数
  - `$name` → 命名参数（基于 frontmatter `arguments` 字段映射到位置）
- **`appendIfNoPlaceholder`**：如果没有占位符且 args 非空，追加 `\n\nARGUMENTS: {args}`
- **`parseArguments`**：使用 shell-quote 解析参数字符串

### 1.3 getPluginDataDir

- **位置**：`src/utils/plugins/pluginDirectories.ts:119-123`
- **路径**：`{pluginsDir}/data/{sanitized-plugin-id}/`
- **`pluginsDir`**：`~/.claude/plugins/` 或 `CLAUDE_CODE_PLUGIN_CACHE_DIR` 覆盖
- **`sanitizePluginId`**：`replace(/[^a-zA-Z0-9\-_]/g, '-')`
- **行为**：同步 `mkdirSync(dir, { recursive: true })`，懒调用

### 1.4 Hook Runner 环境变量注入

- **位置**：`src/utils/hooks.ts:881-909`
- **注入变量**：
  | 变量 | 来源 | 条件 |
  |------|------|------|
  | `CLAUDE_PROJECT_DIR` | `getProjectRoot()` | 始终 |
  | `CLAUDE_PLUGIN_ROOT` | `pluginRoot` (toHookPath) | `pluginRoot` 存在 |
  | `CLAUDE_PLUGIN_DATA` | `getPluginDataDir(pluginId)` (toHookPath) | `pluginId` 存在 |
  | `CLAUDE_PLUGIN_OPTION_{key}` | `loadPluginOptions(pluginId)` | `pluginOpts` 存在 |
- **命令字符串替换**：内联 `command.replace(/\$\{CLAUDE_PLUGIN_ROOT\}/g, ...)`，不调用 `substitutePluginVariables`，因为需区分 bash（POSIX 路径）和 PowerShell（原生路径）
- **注意**：`skillRoot` 也映射到 `CLAUDE_PLUGIN_ROOT`（skills 使用同名变量保持兼容）

### 1.5 ${CLAUDE_SKILL_DIR}

- **位置**：`src/utils/plugins/loadPluginCommands.ts:360-369`
- **条件**：仅当 `config.isSkillMode` 为 true 时替换
- **值**：`dirname(file.filePath)`，Windows 标准化

### 1.6 本批排除的变量

| 变量 | 排除原因 |
|------|----------|
| `${user_config.X}` | 需要 secureStorage + settings.pluginConfigs |
| `${CLAUDE_SESSION_ID}` | 需要 session ID 基础设施 |
| `${VAR}` 通用环境变量 | Go 侧 `os.ExpandEnv` 可覆盖基本需求 |
| `executeShellCommandsInPrompt` | 需要 engine/REPL 核心扩展 |

---

## §2 Go 侧变量替换现状与缺口

### 2.1 CommandAdapter (`internal/platform/plugin/command_adapter.go`)

| 能力 | 状态 | 说明 |
|------|------|------|
| `${1}`, `${2}` 位置参数 | ✅ 已有 | `substituteSimpleArgs` 实现 |
| `${CLAUDE_PLUGIN_ROOT}` | ❌ 缺失 | 未替换 |
| `${CLAUDE_PLUGIN_DATA}` | ❌ 缺失 | 未替换 |
| `${CLAUDE_SKILL_DIR}` | ❌ 缺失 | 未替换 |
| `$ARGUMENTS` | ❌ 缺失 | 未实现 |
| `$ARGUMENTS[n]` | ❌ 缺失 | 未实现 |
| `$n` | ❌ 缺失 | 未实现 |
| `$name` 命名参数 | ❌ 缺失 | 未实现 |
| `AllowedTools` 消费 | ❌ 缺失 | 字段存在但未使用 |
| `ArgumentHint` 消费 | ❌ 缺失 | 字段存在但未使用 |

### 2.2 HookRunner (`internal/runtime/hooks/runner.go`)

| 能力 | 状态 | 说明 |
|------|------|------|
| 命令字符串变量替换 | ❌ 缺失 | `cmdHook.Command` 直接执行，无替换 |
| `CLAUDE_PROJECT_DIR` 注入 | ❌ 缺失 | 未注入 |
| `CLAUDE_PLUGIN_ROOT` 注入 | ❌ 缺失 | 未注入 |
| `CLAUDE_PLUGIN_DATA` 注入 | ❌ 缺失 | 未注入 |
| `CLAUDE_PLUGIN_OPTION_*` 注入 | ❌ 缺失 | 本批不实现 |
| pluginRoot/pluginId/skillRoot 传入 | ❌ 缺失 | `HookCommand` 无此信息 |

### 2.3 McpRegistrar (`internal/platform/plugin/mcp_registrar.go`)

| 能力 | 状态 | 说明 |
|------|------|------|
| `Command` 变量替换 | ❌ 缺失 | `toClientServerConfig` 直接透传 |
| `Args` 变量替换 | ❌ 缺失 | 直接透传 |
| `Env` 变量替换 | ❌ 缺失 | 直接透传 |
| `CLAUDE_PLUGIN_ROOT` 注入 | ❌ 缺失 | 未注入到 Env |
| `CLAUDE_PLUGIN_DATA` 注入 | ❌ 缺失 | 未注入到 Env |

### 2.4 LspRegistrar (`internal/platform/plugin/lsp_registrar.go`)

| 能力 | 状态 | 说明 |
|------|------|------|
| `Command` 变量替换 | ❌ 缺失 | `toLspServerConfig` 直接透传 |
| `Args` 变量替换 | ❌ 缺失 | 直接透传 |
| `Env` 变量替换 | ❌ 缺失 | 直接透传 |
| `CLAUDE_PLUGIN_ROOT` 注入 | ❌ 缺失 | 未注入到 Env |
| `CLAUDE_PLUGIN_DATA` 注入 | ❌ 缺失 | 未注入到 Env |

### 2.5 AgentRegistrar (`internal/platform/plugin/agent_registrar.go`)

| 能力 | 状态 | 说明 |
|------|------|------|
| `RawContent` 变量替换 | ❌ 缺失 | 直接透传为 `SystemPrompt` |
| **本批处理** | ⚠️ 延后 | Agent system prompt 替换在 engine 侧消费，registrar 侧只做透传 |

### 2.6 类型缺口

| 类型 | 字段 | 说明 |
|------|------|------|
| `LoadedPlugin` | `Source` 字段缺失 | TS 侧 `plugin.source` = `name@marketplace`，用于 `getPluginDataDir` 的 key |
| `PluginCommand` | `PluginSource` 字段缺失 | 需要 `source` 来做 `${CLAUDE_PLUGIN_DATA}` 替换 |
| `PluginCommand` | `ArgumentNames` 字段缺失 | 需要存储 frontmatter `arguments` 字段以支持命名参数 |

---

## §3 边界确认

### 3.1 本批实现的变量集合

| 变量 | 使用场景 | 实现文件 |
|------|----------|----------|
| `${CLAUDE_PLUGIN_ROOT}` | CommandAdapter, HookRunner, MCP, LSP | `variables.go` |
| `${CLAUDE_PLUGIN_DATA}` | CommandAdapter, HookRunner, MCP, LSP | `variables.go` + `directories.go` |
| `${CLAUDE_SKILL_DIR}` | CommandAdapter (skill mode only) | `variables.go` |
| `$ARGUMENTS` | CommandAdapter | `variables.go` |
| `$ARGUMENTS[n]` | CommandAdapter | `variables.go` |
| `$n` | CommandAdapter | `variables.go` |
| `$name` (named) | CommandAdapter | `variables.go` |

### 3.2 数据目录规范

```
{pluginsDir}/data/{sanitized-id}/
pluginsDir = ~/.claude/plugins/  (或 CLAUDE_CODE_PLUGIN_CACHE_DIR 覆盖)
sanitized-id = replace(/[^a-zA-Z0-9\-_]/g, '-')
```

- 使用 `os.MkdirAll` 递归创建
- 懒创建：仅在变量替换需要时调用
- Go 实现位置：`internal/platform/plugin/directories.go`

### 3.3 Hook 环境变量矩阵

| 变量 | 值 | 条件 |
|------|-----|------|
| `CLAUDE_PROJECT_DIR` | `projectRoot` (运行时传入) | 始终注入 |
| `CLAUDE_PLUGIN_ROOT` | `pluginRoot` 或 `skillRoot` | root 存在时 |
| `CLAUDE_PLUGIN_DATA` | `GetPluginDataDir(pluginSource)` | source 存在时 |

### 3.4 CommandAdapter 运行时字段消费边界

| 字段 | 本批消费方式 | 说明 |
|------|-------------|------|
| `AllowedTools` | 解析为 `[]string` | 当前仅解析存储，engine 侧完整消费延后 |
| `ArgumentHint` | 集成到 `Metadata().Usage` | `Usage: "/name [argHint]"` |
| `Model` | ❌ 延后 | 需要 engine 侧支持 |
| `Effort` | ❌ 延后 | 需要 engine 侧支持 |
| `DisableModelInvocation` | ❌ 延后 | 需要 engine 侧支持 |
| `Shell` | ❌ 延后 | 需要 engine/REPL 侧支持 |

### 3.5 架构决策

1. **变量替换引擎位置**：`internal/platform/plugin/variables.go` — 纯函数，无外部依赖
2. **数据目录位置**：`internal/platform/plugin/directories.go` — 依赖 `os` 和路径工具
3. **Hook runner 改造**：`HookCommand` 不添加字段，通过调用方传入 `pluginRoot/pluginSource/skillRoot` 参数
4. **MCP/LSP 改造**：在 `toClientServerConfig`/`toLspServerConfig` 前增加变量替换层
5. **Windows 路径**：统一使用 `filepath.ToSlash` 做标准化（Go 侧 bash 执行用 POSIX 路径）
