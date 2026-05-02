package promptsuggestion

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// SpeculationState 表示推测系统的状态
type SpeculationState string

const (
	SpeculationStateIdle   SpeculationState = "idle"
	SpeculationStateActive SpeculationState = "active"
)

// ActiveSpeculationState 记录活跃推测的元数据
type ActiveSpeculationState struct {
	ID        string
	StartedAt time.Time
	Cancel    context.CancelFunc
}

// Speculator 管理推测状态机
type Speculator struct {
	mu     sync.Mutex
	state  SpeculationState
	active *ActiveSpeculationState
}

// NewSpeculator 创建 Speculator 实例
func NewSpeculator() *Speculator {
	return &Speculator{
		state: SpeculationStateIdle,
	}
}

// StartParams 是 Start 的参数
type StartParams struct {
	SuggestionText string
	Messages       []message.Message
}


// Start 启动推测（简化版）
// 流程：
// 1. 若 IsSpeculationEnabled() == false → 返回（状态保持 idle）
// 2. 若当前 active，先 Abort()
// 3. 创建 child context + cancel
// 4. 状态切换为 active，记录 ActiveSpeculationState
// 5. 启动 goroutine 执行占位逻辑（time.Sleep 100ms 模拟，或仅记录日志）
// 注意：不真正 fork agent（runner 占位），不创建 overlay 文件
func (s *Speculator) Start(ctx context.Context, params StartParams) {
	if !IsSpeculationEnabled() {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == SpeculationStateActive {
		s.abortLocked()
	}

	childCtx, cancel := context.WithCancel(ctx)

	activeState := &ActiveSpeculationState{
		ID:        "speculation-" + time.Now().Format(time.RFC3339Nano),
		StartedAt: time.Now(),
		Cancel:    cancel,
	}
	s.state = SpeculationStateActive
	s.active = activeState

	go func() {
		logger.DebugCF("speculation", "speculation started", map[string]any{
			"id": activeState.ID,
		})
		select {
		case <-childCtx.Done():
			logger.DebugCF("speculation", "speculation cancelled", map[string]any{
				"id": activeState.ID,
			})
		case <-time.After(100 * time.Millisecond):
			logger.DebugCF("speculation", "speculation completed (placeholder)", map[string]any{
				"id": activeState.ID,
			})
		}
	}()
}

// Abort 中止当前推测并切回 idle
func (s *Speculator) Abort() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.abortLocked()
}

// abortLocked 在已持有锁的情况下中止当前推测
func (s *Speculator) abortLocked() {
	if s.active != nil && s.active.Cancel != nil {
		s.active.Cancel()
	}
	s.state = SpeculationStateIdle
	s.active = nil
}

// State 返回当前状态（线程安全）
func (s *Speculator) State() SpeculationState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// prepareMessagesForInjection 消息预处理
// 过滤掉：
// - thinking / redacted_thinking content parts
// - 纯空白文本消息
// 保留其他所有内容
func prepareMessagesForInjection(messages []message.Message) []message.Message {
	result := make([]message.Message, 0, len(messages))

	for _, msg := range messages {
		filtered := make([]message.ContentPart, 0, len(msg.Content))
		for _, part := range msg.Content {
			if part.Type == "thinking" || part.Type == "redacted_thinking" {
				continue
			}
			filtered = append(filtered, part)
		}

		// 如果过滤后没有 content parts，跳过该消息
		if len(filtered) == 0 {
			continue
		}

		// 如果过滤后只剩下空白文本，跳过该消息
		allWhitespace := true
		for _, part := range filtered {
			if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
				allWhitespace = false
				break
			}
			if part.Type != "text" {
				allWhitespace = false
				break
			}
		}
		if allWhitespace {
			continue
		}

		result = append(result, message.Message{
			Role:    msg.Role,
			Content: filtered,
		})
	}

	return result
}
