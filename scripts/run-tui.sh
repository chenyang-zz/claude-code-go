#!/bin/bash
# TUI 启动脚本：单命令同时启动 Go 后端 + TUI 前端
# Go 后台运行，TUI 独占终端（通过 script 分配 PTY）
# 自动使用临时 session DB，避免迁移冲突

set -e

DIR="$(cd "$(dirname "$0")/.." && pwd)"
TUI_DIR="$DIR/tui"
TUI_LOG_DIR="$TUI_DIR/logs"
mkdir -p "$TUI_LOG_DIR"

# 默认开启前端调试日志；显式设置 TUI_DEBUG=0 可关闭
if [ -z "${TUI_DEBUG+x}" ]; then
  TUI_DEBUG=1
fi
if [ -z "${TUI_DEBUG_LEVEL+x}" ]; then
  TUI_DEBUG_LEVEL=debug
fi
if [ -z "${TUI_DEBUG_MODULE+x}" ]; then
  TUI_DEBUG_MODULE="ws,render,state"
fi

# 编译（如果二进制不存在或源码更新）
cd "$DIR"
go build -o /tmp/cc-tui ./cmd/cc/. 2>/dev/null || {
  echo "编译失败" >&2
  exit 1
}

# 使用临时 DB 避免迁移冲突
SESSION_DB="/tmp/cc-tui-sessions.db"
rm -f "$SESSION_DB"

# 启动 Go 后端（后台），捕获端口
CLAUDE_CODE_SESSION_DB_PATH="$SESSION_DB" /tmp/cc-tui --tui 2>/tmp/cc-tui-port.log &
CC_PID=$!

# 等待端口输出（日志包含 ANSI 转义码，需 strip 后解析）
PORT=""
for i in $(seq 1 10); do
  PORT=$(sed 's/\x1b\[[0-9;]*m//g' /tmp/cc-tui-port.log 2>/dev/null | grep -Eo 'port=[0-9]+' | tail -1 | cut -d= -f2)
  if [ -n "$PORT" ]; then
    break
  fi
  sleep 1
done

if [ -z "$PORT" ]; then
  echo "错误：无法获取 WebSocket 端口" >&2
  kill $CC_PID 2>/dev/null
  exit 1
fi

echo "Go 后端已启动 (PID: $CC_PID)"
echo "WebSocket 端口: $PORT"

# 透传前端调试环境变量到 bun 进程（含 tmux 场景）
FRONTEND_ENV=""
[ -n "${TUI_DEBUG:-}" ] && FRONTEND_ENV="$FRONTEND_ENV TUI_DEBUG='$TUI_DEBUG'"
[ -n "${TUI_DEBUG_LEVEL:-}" ] && FRONTEND_ENV="$FRONTEND_ENV TUI_DEBUG_LEVEL='$TUI_DEBUG_LEVEL'"
[ -n "${TUI_DEBUG_MODULE:-}" ] && FRONTEND_ENV="$FRONTEND_ENV TUI_DEBUG_MODULE='$TUI_DEBUG_MODULE'"
[ -n "${TUI_DEBUG_STDERR:-}" ] && FRONTEND_ENV="$FRONTEND_ENV TUI_DEBUG_STDERR='$TUI_DEBUG_STDERR'"
[ -n "${TUI_DEBUG_FILE:-}" ] && FRONTEND_ENV="$FRONTEND_ENV TUI_DEBUG_FILE='$TUI_DEBUG_FILE'"
[ -n "${TUI_DEBUG_CONTENT:-}" ] && FRONTEND_ENV="$FRONTEND_ENV TUI_DEBUG_CONTENT='$TUI_DEBUG_CONTENT'"
[ -n "${TUI_TYPEWRITER_INTERVAL_MS:-}" ] && FRONTEND_ENV="$FRONTEND_ENV TUI_TYPEWRITER_INTERVAL_MS='$TUI_TYPEWRITER_INTERVAL_MS'"
[ -n "${TUI_TYPEWRITER_DELTA_CHARS_PER_TICK:-}" ] && FRONTEND_ENV="$FRONTEND_ENV TUI_TYPEWRITER_DELTA_CHARS_PER_TICK='$TUI_TYPEWRITER_DELTA_CHARS_PER_TICK'"
[ -n "${TUI_TYPEWRITER_THINKING_CHARS_PER_TICK:-}" ] && FRONTEND_ENV="$FRONTEND_ENV TUI_TYPEWRITER_THINKING_CHARS_PER_TICK='$TUI_TYPEWRITER_THINKING_CHARS_PER_TICK'"

if [ "${TUI_DEBUG:-}" != "0" ]; then
  EFFECTIVE_DEBUG_FILE="${TUI_DEBUG_FILE:-$TUI_LOG_DIR/frontend-debug.log}"
  if [ "${TUI_DEBUG_RESET_ON_START:-0}" = "1" ]; then
    mkdir -p "$(dirname "$EFFECTIVE_DEBUG_FILE")"
    : > "$EFFECTIVE_DEBUG_FILE"
    clear
  fi
  echo "前端调试日志: $EFFECTIVE_DEBUG_FILE"
fi

# 使用 script 创建 PTY 运行 TUI，配合 tmux 分屏显示日志
# TUI_TMUX=0 可禁用 tmux（直接全屏运行 TUI）
if command -v tmux &>/dev/null && [ "${TUI_TMUX:-1}" != "0" ]; then
  # 清理旧 session
  tmux kill-session -t cc-tui 2>/dev/null || true

  # 创建 tmux session：左 TUI / 右日志
  tmux new-session -d -s cc-tui "script -q /dev/null sh -c \"cd '$TUI_DIR' && env$FRONTEND_ENV bun run src/index.tsx --port $PORT\""
  tmux split-window -t cc-tui -h "tail -f /tmp/cc-tui-port.log"

  echo "切换 pane（TUI ↔ 日志）: Ctrl+B → ←/→"
  echo "退出 TUI 后自动关闭分屏"
  echo ""

  tmux attach-session -t cc-tui || true

  # TUI 退出后清理
  kill $CC_PID 2>/dev/null || true
  wait $CC_PID 2>/dev/null || true
  tmux kill-session -t cc-tui 2>/dev/null || true
else
  # 无 tmux：显示日志摘要
  echo ""
  grep -vE '^[[:space:]]*$' /tmp/cc-tui-port.log | tail -3
  echo "Go 日志: tail -f /tmp/cc-tui-port.log"
  echo ""
  script -q /dev/null sh -c "cd '$TUI_DIR' && env$FRONTEND_ENV bun run src/index.tsx --port $PORT"
fi

# TUI 退出后，清理 Go 进程
kill $CC_PID 2>/dev/null
wait $CC_PID 2>/dev/null || true
echo ""
echo "TUI 已退出"
echo "Go 日志: tail -f /tmp/cc-tui-port.log"
