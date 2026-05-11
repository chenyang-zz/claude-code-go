package server

import "time"

// ServerConfig holds the configuration for an embedded direct-connect server.
type ServerConfig struct {
	// Port is the TCP port the server listens on.
	Port int
	// Host is the network interface to bind to (e.g. "localhost").
	Host string
	// AuthToken is the Bearer token required for incoming connections.
	AuthToken string
	// Unix socket path, if set takes precedence over TCP.
	Unix string
	// IdleTimeout is the idle timeout for detached sessions. 0 means never expire.
	IdleTimeout time.Duration
	// MaxSessions is the maximum number of concurrent sessions. 0 means unlimited.
	MaxSessions int
	// Workspace is the default working directory for new sessions.
	Workspace string
}

// SessionState represents the lifecycle state of a direct-connect session.
type SessionState string

const (
	SessionStarting SessionState = "starting"
	SessionRunning  SessionState = "running"
	SessionDetached SessionState = "detached"
	SessionStopping SessionState = "stopping"
	SessionStopped  SessionState = "stopped"
)

// SessionInfo holds runtime metadata for one direct-connect session.
type SessionInfo struct {
	ID        string
	Status    SessionState
	CreatedAt time.Time
	WorkDir   string
	SessionKey string
}

// SessionIndexEntry is the persisted metadata for one session.
type SessionIndexEntry struct {
	SessionID          string
	TranscriptSessionID string
	CWD                string
	PermissionMode     string
	CreatedAt          time.Time
	LastActiveAt       time.Time
}

// SessionIndex maps stable session keys to their metadata.
type SessionIndex map[string]SessionIndexEntry

// ConnectResponse represents the response from creating a direct-connect session.
type ConnectResponse struct {
	SessionID string `json:"session_id"`
	WsURL     string `json:"ws_url"`
	WorkDir   string `json:"work_dir,omitempty"`
}

// DirectConnectConfig holds the connection details for an established session.
type DirectConnectConfig struct {
	ServerURL string
	SessionID string
	WsURL     string
	AuthToken string
}
