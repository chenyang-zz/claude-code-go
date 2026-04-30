# batch-207 阶段 1 分析：Plugin 热重载源码对照与边界确认

## §1 TS 侧 Plugin 文件监听与热重载源码分析

### 1.1 关键发现：TS 侧无直接 Plugin 目录 Watcher

当前 TypeScript 代码中**不存在**对 `~/.claude/plugins/` 目录的 chokidar 文件监听。Plugin 系统的"热重载"通过以下间接机制实现：

| 机制 | 触发源 | 行为 |
|------|--------|------|
| Settings Watcher | `~/.claude/settings.json` 变化 | `enabledPlugins` 等字段变化时设置 `needsRefresh=true` |
| Skill Watcher | `~/.claude/skills/` 等目录变化 | **独立系统**，只清除 skill/command 缓存，**不触发 plugin reload** |
| `/reload-plugins` 命令 | 用户手动执行 | 直接调用 `refreshActivePlugins()` |
| `needsRefresh` 通知 | UI 显示通知引导用户 | `useManagePlugins.ts:293-303` |

### 1.2 核心刷新原语：`refreshActivePlugins()`

**文件**：`src/utils/plugins/refresh.ts:72-191`

**执行流程**（全量刷新）：

```
1. clearAllCaches()                    — 清除所有 plugin 相关缓存
2. clearPluginCacheExclusions()        — 清除孤儿插件排除缓存
3. loadAllPlugins()                    — 重新发现+加载所有 plugins（含网络请求）
4. getPluginCommands()                 — 加载 commands
5. getAgentDefinitionsWithOverrides()  — 加载 agents
6. load MCP servers (per plugin)       — 加载 MCP
7. load LSP servers (per plugin)       — 加载 LSP
8. setAppState({ plugins, agentDefinitions, mcp, ... }) — 写入运行时状态
9. reinitializeLspServerManager()      — 重新初始化 LSP manager
10. loadPluginHooks()                  — 加载 hooks（容错：失败不阻断）
```

**三层模型**（代码注释明确）：
- Layer 1: intent (settings) — `enabledPlugins` 等配置
- Layer 2: materialization — `~/.claude/plugins/` 实际目录内容
- Layer 3: active components (AppState) — 运行时活跃的 commands/agents/hooks/MCP/LSP

`refreshActivePlugins()` 是 Layer-3 的刷新原语。

### 1.3 Debounce 策略

- **Plugin 系统本身无独立 debounce**。调用方直接顺序执行。
- **Skill Watcher** 使用 `RELOAD_DEBOUNCE_MS = 300ms`（`skillChangeDetector.ts:42`），`setTimeout` + `clearTimeout` 重置模式。
- **Settings Watcher** 使用 `awaitWriteFinish: { stabilityThreshold: 1000, pollInterval: 500 }`，并配合 `DELETION_GRACE_MS = 1700ms` 处理"删除-重建"模式。
- `needsRefresh` 标志作为去重手段——多次设置时若已为 true 则跳过。

### 1.4 失败处理语义

- **部分成功，不回滚**：各组件独立 try-catch，错误推入 `errors` 数组
- **Hooks 容错**：`loadPluginHooks()` 失败仅增加 `error_count`，不影响其他数据（`refresh.ts:152-161`）
- **后台安装降级**：自动 refresh 失败 → 降级为设置 `needsRefresh=true`，用户可手动重试
- **错误展示**：`/reload-plugins` 返回消息包含 error count，提示运行 `/doctor`

### 1.5 热重载与 `/reload-plugins` 命令的关系

| 维度 | 说明 |
|------|------|
| `/reload-plugins` | 用户手动触发 Layer-3 refresh 的入口 |
| 自动热重载 | **不存在**对 plugin 目录的自动文件监听热重载 |
| NeedsRefresh | disk 状态变化时设置 `needsRefresh=true`，UI 通知引导手动 reload |
| Headless 模式 | `refreshPluginState()` 自动调用（无需用户交互） |
| Hook 自动重载 | Policy settings 变化时 hooks **自动**热重载（独立路径） |

**设计意图**（PR 5c）：所有 Layer-3 交换都通过 `/reload-plugins` 走 `refreshActivePlugins()`，保持单一一致的心智模型。之前的 auto-refresh 存在 stale-cache bug。

### 1.6 对 Go 侧迁移的启示

1. **Go 侧需要新增 plugin 目录 fsnotify 监听**（TS 侧没有这一层）
2. **全量刷新策略与 TS 侧一致**：`clearAllCaches()` → `loadAllPlugins()` → 各组件重新加载
3. **Debounce 窗口 300ms** 与 skill watcher 对齐
4. **失败不回滚语义**保持一致：各组件独立加载，错误聚合
5. **不需要 NeedsRefresh 通知机制**：CLI 场景下可直接自动 reload（没有 UI 通知层）
6. **不需要 Settings Watcher**：Go 侧 settings 变更机制不同

---

## §2 Go 侧 Watcher 基础设施与子系统 Unregister 现状评估

### 2.1 Skill Watcher 可复用程度

**文件**：`internal/services/tools/skill/watcher.go`（244 行）

| 组件 | 复用度 | 说明 |
|------|--------|------|
| Debounce 逻辑（timer + mutex） | **高** | `scheduleReload()` 的 timer 重置模式通用，可直接复用 |
| `fsnotify.Watcher` 生命周期 | **中** | `Start`/`Stop`/`loop` 模式可用，但需修复 `stopCh` 复用问题 |
| `.git` 路径过滤 | **高** | `isGitPath()` 纯函数，完全通用 |
| 硬编码回调 `emitSkillsLoaded()` | **不可复用** | 需改为注入式回调 |
| 全局单例 `DefaultWatcher` | **不可复用** | 需支持多实例或同时管理多组目录 |
| 只递归一层子目录 | **需调整** | plugin 目录结构更深，需递归监听所有子目录 |
| 无临时文件过滤 | **需补充** | 增加 `.tmp`、`.swp`、`~`、`.DS_Store` 等黑名单 |

**结论**：建议**新建独立** `internal/platform/plugin/watcher.go`，拷贝核心 debounce/fsnotify 模式，但独立演进 plugin 特有的过滤规则和回调机制。避免改动现有 skill watcher 引入回归风险。

### 2.2 子系统 Unregister 能力盘点

#### 2.2.1 Plugin Registry（Plugin 级）

**文件**：`internal/platform/plugin/registry.go`

- **已有能力**：`Unregister(name string) error` 方法已存在，可从 `InMemoryPluginRegistry` 中移除 plugin 记录
- **缺口**：Plugin 级 unregister 已有，但**能力级 unregister 缺失**（agent/MCP/LSP/commands/hooks 的独立移除）

#### 2.2.2 Agent Registry

**文件**：`internal/core/agent/registry.go`

- **已有能力**：`RegisterAgent(definition AgentDefinition) error`
- **缺口**：**无 `Unregister` 方法**。`InMemoryRegistry` 只有 `agents map[string]AgentDefinition`，没有移除接口
- **影响**：热重载时无法清理旧 plugin 注册的 agents

#### 2.2.3 MCP Registry

**文件**：`internal/platform/mcp/registry/server_registry.go`

- **已有能力**：`RegisterServer(name string, config ServerConfig) error`
- **缺口**：**无 `RemoveServer` 方法**。无法停止/移除已注册的 MCP server
- **影响**：热重载时旧 plugin 的 MCP servers 持续运行，造成资源泄漏和冲突

#### 2.2.4 LSP Manager

**文件**：`internal/platform/lsp/manager.go`

- **已有能力**：`RegisterServer(name string, config ServerConfig) error`
- **缺口**：**无 `StopServer`/`RemoveServer` 方法**。无法停止/移除已注册的 LSP server
- **影响**：热重载时旧 plugin 的 LSP servers 持续运行，造成资源泄漏

#### 2.2.5 Command Registry

**文件**：`internal/services/commands/registry.go`

- **已有能力**：`Register(name string, handler CommandHandler) error` + `Unregister(name string) error`
- **状态**：**已有 unregister 能力**

#### 2.2.6 Hooks

**文件**：`internal/runtime/hooks/manager.go`

- **已有能力**：`RegisterMatchers(matchers []CommandMatcher)` + `ClearMatchers()`
- **状态**：**已有 clear 能力**（`ClearMatchers` 可全量清空），但无单条移除

### 2.3 Unregister 缺口总结

| 子系统 | 已有能力 | 缺失能力 | 优先级 |
|--------|----------|----------|--------|
| Plugin Registry | `Unregister(name)` | — | 已有 |
| Command Registry | `Unregister(name)` | — | 已有 |
| Hooks | `ClearMatchers()` | 单条移除（非必须） | 已有（Clear 够用） |
| **Agent Registry** | `RegisterAgent` | **`Unregister(name)`** | **P0** |
| **MCP Registry** | `RegisterServer` | **`RemoveServer(name)`** | **P0** |
| **LSP Manager** | `RegisterServer` | **`StopServer(name)`/`RemoveServer(name)`** | **P0** |

---

## §3 热重载边界确认

### 3.1 全量刷新策略

**决策**：采用"全部卸载 → 全部重新加载 → 全部重新注册"策略（与 TS 侧 `refreshActivePlugins()` 一致）。

**理由**：
1. CLI 场景下插件数量少（通常 <10 个），全量刷新开销可接受
2. 避免维护精细的 capability-to-plugin 映射关系
3. 与 TS 侧语义一致，降低心智负担
4. 实现简单，正确性易验证

**执行管线**：

```
PluginReloader.Reload()
  ├─ 1. mu.Lock()（防止并发 reload）
  ├─ 2. registrar.UnregisterAll(baseHooks)  ← 全量注销
  │     ├─ commands.UnregisterAll()
  │     ├─ agentRegistry.UnregisterAll()
  │     ├─ mcpRegistry.RemoveAllServers()
  │     ├─ lspManager.StopAllServers()
  │     ├─ hooks.ClearMatchers()
  │     └─ pluginRegistry.UnregisterAll()
  ├─ 3. loader.RefreshActivePlugins()        ← 重新加载
  ├─ 4. registrar.RegisterAll(result, baseHooks) ← 重新注册
  └─ 5. mu.Unlock()
```

### 3.2 防抖参数

| 参数 | 值 | 来源 |
|------|-----|------|
| Debounce 窗口 | **300ms** | 与 skill watcher (`defaultDebounceInterval = 300ms`) 对齐 |
| 实现方式 | `time.AfterFunc` + timer 重置 | 复用 skill watcher 模式 |
| 事件聚合 | 窗口内所有事件合并为一次 reload | 不区分事件类型 |

### 3.3 并发安全

| 场景 | 策略 |
|------|------|
| 并发 reload | **reload 锁**：`PluginReloader` 持有 `sync.Mutex`，禁止并发执行 reload |
| reload 期间新事件 | **丢弃**：若 reload 进行中收到新 fsnotify 事件，设置 `pendingReload=true`，reload 完成后自动触发下一次 |
| watcher 启动/停止 | 与 reload 锁独立，但 `Stop()` 时停止 debounce timer |

### 3.4 失败处理

| 场景 | 策略 |
|------|------|
| Unregister 失败 | **记录错误，继续执行**。unregister 是清理操作，失败不阻断 reload |
| RefreshActivePlugins 失败 | **中断 reload**，保持当前注册状态（unregister 已执行，但 register 未完成） |
| RegisterAll 失败 | **部分注册**，错误记录到日志。已注册的部分保持运行 |
| Hooks 加载失败 | **容错**：单独 try-catch，不影响其他组件 |

**不回滚策略**：与 TS 侧一致，采用 best-effort 语义。失败时可能处于"部分卸载+部分注册"的中间状态，但不主动回滚。用户可再次触发 reload 或重启恢复。

### 3.5 监听范围

| 目录 | 级别 | 说明 |
|------|------|------|
| `~/.claude/plugins/` | 用户级 | 全局插件目录 |
| `{projectPath}/.claude/plugins/` | 项目级 | 项目本地插件 |
| **监听深度** | 递归所有子目录 | plugin 目录结构包含多级子目录 |
| **过滤规则** | `.git`、临时文件 | 排除 `.git/`、`.tmp`、`.swp`、`~`、`.DS_Store` |
| **关注事件** | Create/Write/Remove/Rename | 与 skill watcher 一致 |

### 3.6 与 `/reload-plugins` 命令的集成

- `/reload-plugins` 命令内部复用 `PluginReloader.Reload()` 管线
- 手动 reload 和自动 watcher 触发共用同一执行管线
- 消除手动/自动两条路径的语义差异

### 3.7 明确不做的事（本批边界）

1. **定向热重载**：不实现单插件级精细刷新
2. **Settings Watcher**：不监听 settings 文件变化
3. **NeedsRefresh 通知**：CLI 场景直接自动 reload，不需要通知层
4. **Hook 独立热重载**：Hooks 随全量刷新一起重载，不单独处理 policy settings 变化
5. **Deletion Grace**：不实现 1700ms 的删除宽限期（简化实现，后续如有需要可补充）
