package mailbox

import (
	"sync"
	"time"
)

// InboxPoller periodically polls a teammate's inbox for new messages.
// It is a simplified Go equivalent of the TS useInboxPoller hook, without
// React/AppState dependencies, permission routing, or UI concerns.
type InboxPoller struct {
	agentName string
	teamName  string
	homeDir   string
	interval  time.Duration
	onMessage func([]Message)

	mu      sync.Mutex
	ticker  *time.Ticker
	stopCh  chan struct{}
	running bool
}

// inboxMessageCallback is the signature for the callback invoked when new
// unread messages are found.
type inboxMessageCallback func(messages []Message)

// NewInboxPoller creates a new InboxPoller for the given agent and team.
// The onMessage callback is invoked on each poll cycle when unread messages
// are found. A nil callback is treated as a no-op.
func NewInboxPoller(agentName, teamName, homeDir string, interval time.Duration, onMessage inboxMessageCallback) *InboxPoller {
	if interval <= 0 {
		interval = 1 * time.Second
	}
	if onMessage == nil {
		onMessage = func(_ []Message) {}
	}
	return &InboxPoller{
		agentName: agentName,
		teamName:  teamName,
		homeDir:   homeDir,
		interval:  interval,
		onMessage: onMessage,
	}
}

// Start begins polling. If the poller is already running, this is a no-op.
func (p *InboxPoller) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return
	}
	p.running = true
	p.ticker = time.NewTicker(p.interval)
	p.stopCh = make(chan struct{})

	go func() {
		for {
			select {
			case <-p.ticker.C:
				p.poll()
			case <-p.stopCh:
				return
			}
		}
	}()
}

// Stop stops polling. If the poller is not running, this is a no-op.
func (p *InboxPoller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return
	}
	p.ticker.Stop()
	close(p.stopCh)
	p.running = false
}

// IsRunning reports whether the poller is currently active.
func (p *InboxPoller) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// poll executes one poll cycle: reads unread messages and invokes the callback.
func (p *InboxPoller) poll() {
	messages, err := ReadUnreadMessages(p.agentName, p.teamName, p.homeDir)
	if err != nil || len(messages) == 0 {
		return
	}

	// Invoke callback with unread messages
	p.onMessage(messages)

	// Mark messages as read after successful delivery
	_ = MarkMessagesAsRead(p.agentName, p.teamName, p.homeDir)
}
