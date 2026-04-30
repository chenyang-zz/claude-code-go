package lsp

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if len(m.servers) != 0 {
		t.Error("new manager should have no servers")
	}
	if len(m.extMap) != 0 {
		t.Error("new manager should have no extension mappings")
	}
}

func TestRegisterServer(t *testing.T) {
	m := NewManager()

	err := m.RegisterServer("gopls", ServerConfig{
		Command: "gopls",
		Args:    []string{"-mode=stdio"},
	}, []string{".go"})
	if err != nil {
		t.Fatalf("RegisterServer error: %v", err)
	}

	if len(m.servers) != 1 {
		t.Errorf("expected 1 server, got %d", len(m.servers))
	}
	if len(m.extMap) != 1 {
		t.Errorf("expected 1 extension mapping, got %d", len(m.extMap))
	}

	entry, ok := m.servers["gopls"]
	if !ok {
		t.Fatal("server gopls not found")
	}
	if entry.State != ServerStateStopped {
		t.Errorf("expected stopped state, got %s", entry.State)
	}
}

func TestRegisterServer_Duplicate(t *testing.T) {
	m := NewManager()
	_ = m.RegisterServer("gopls", ServerConfig{Command: "gopls"}, []string{".go"})

	err := m.RegisterServer("gopls", ServerConfig{Command: "gopls2"}, []string{".go"})
	if err == nil {
		t.Error("expected error for duplicate server registration")
	}
}

func TestRegisterServer_AutoDotPrefix(t *testing.T) {
	m := NewManager()
	_ = m.RegisterServer("test", ServerConfig{Command: "test"}, []string{"go"})

	if name, ok := m.extMap[".go"]; !ok || name != "test" {
		t.Errorf("expected .go → test, got %v", m.extMap)
	}
}

func TestGetServerForFile(t *testing.T) {
	m := NewManager()
	_ = m.RegisterServer("gopls", ServerConfig{Command: "gopls"}, []string{".go"})
	_ = m.RegisterServer("tsserver", ServerConfig{Command: "typescript-language-server", Args: []string{"--stdio"}}, []string{".ts"})

	entry := m.getServerForFile("/path/to/file.go")
	if entry == nil {
		t.Fatal("expected server for .go file")
	}
	if entry.Name != "gopls" {
		t.Errorf("expected gopls, got %s", entry.Name)
	}

	entry = m.getServerForFile("/path/to/file.ts")
	if entry == nil {
		t.Fatal("expected server for .ts file")
	}
	if entry.Name != "tsserver" {
		t.Errorf("expected tsserver, got %s", entry.Name)
	}

	entry = m.getServerForFile("/path/to/file.py")
	if entry != nil {
		t.Error("expected no server for .py file")
	}
}

func TestIsFileOpen(t *testing.T) {
	m := NewManager()

	if m.IsFileOpen("/test/file.go") {
		t.Error("file should not be open initially")
	}

	// Simulate tracking a file as open.
	m.openFiles["file:///test/file.go"] = "gopls"

	if !m.IsFileOpen("/test/file.go") {
		t.Error("file should be tracked as open")
	}
}

func TestShutdown(t *testing.T) {
	m := NewManager()
	_ = m.RegisterServer("gopls", ServerConfig{Command: "gopls"}, []string{".go"})
	_ = m.RegisterServer("tsserver", ServerConfig{Command: "tsserver"}, []string{".ts"})

	m.openFiles["file:///test/file.go"] = "gopls"

	err := m.Shutdown()
	if err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}

	if len(m.servers) != 0 {
		t.Error("servers should be cleared after shutdown")
	}
	if len(m.extMap) != 0 {
		t.Error("extension map should be cleared after shutdown")
	}
	if len(m.openFiles) != 0 {
		t.Error("open files should be cleared after shutdown")
	}
}

func TestExtToLanguageID(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{".go", "go"},
		{".ts", "typescript"},
		{".tsx", "typescript"},
		{".js", "javascript"},
		{".py", "python"},
		{".rs", "rust"},
		{".java", "java"},
		{".cpp", "cpp"},
		{".rb", "ruby"},
		{".html", "html"},
		{".json", "json"},
		{".unknown", "plaintext"},
		{"", "plaintext"},
	}

	for _, tc := range tests {
		result := extToLanguageID(tc.ext)
		if result != tc.expected {
			t.Errorf("extToLanguageID(%q) = %q, want %q", tc.ext, result, tc.expected)
		}
	}
}

func TestServerStateString(t *testing.T) {
	tests := []struct {
		state    ServerState
		expected string
	}{
		{ServerStateStopped, "stopped"},
		{ServerStateStarting, "starting"},
		{ServerStateRunning, "running"},
		{ServerStateError, "error"},
		{ServerState(99), "unknown"},
	}

	for _, tc := range tests {
		result := tc.state.String()
		if result != tc.expected {
			t.Errorf("ServerState(%d).String() = %q, want %q", tc.state, result, tc.expected)
		}
	}
}

// ─── Global singleton tests ─────────────────────────────────────────────────

func TestGetManager_InitiallyNil(t *testing.T) {
	SetManager(nil)
	if mgr := GetManager(); mgr != nil {
		t.Error("expected GetManager to return nil before initialization")
	}
}

func TestSetManager_ThenGetManager(t *testing.T) {
	mgr := NewManager()
	SetManager(mgr)

	got := GetManager()
	if got != mgr {
		t.Error("GetManager did not return the manager set by SetManager")
	}

	SetManager(nil)
}

func TestSetManager_NilClearsState(t *testing.T) {
	mgr := NewManager()
	SetManager(mgr)
	SetManager(nil)

	if mgr := GetManager(); mgr != nil {
		t.Error("expected GetManager to return nil after SetManager(nil)")
	}
}

func TestInitialize_Idempotent(t *testing.T) {
	SetManager(nil)

	if err := Initialize(); err != nil {
		t.Fatalf("first Initialize() failed: %v", err)
	}

	first := GetManager()
	if first == nil {
		t.Fatal("expected non-nil manager after Initialize")
	}

	if err := Initialize(); err != nil {
		t.Fatalf("second Initialize() failed: %v", err)
	}

	second := GetManager()
	if first != second {
		t.Error("second Initialize should return same manager instance")
	}

	Shutdown()
}

func TestShutdown_ClearsSingleton(t *testing.T) {
	SetManager(nil)

	if err := Initialize(); err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}
	if GetManager() == nil {
		t.Fatal("expected non-nil manager after Initialize")
	}

	if err := Shutdown(); err != nil {
		t.Fatalf("Shutdown() failed: %v", err)
	}

	if mgr := GetManager(); mgr != nil {
		t.Error("expected nil manager after Shutdown")
	}
}

func TestShutdown_NoOpWhenNotInitialized(t *testing.T) {
	SetManager(nil)

	if err := Shutdown(); err != nil {
		t.Errorf("Shutdown should be no-op when not initialized, got error: %v", err)
	}
}

func TestGetInitializationStatus_AfterSetManager(t *testing.T) {
	SetManager(nil)

	mgr := NewManager()
	SetManager(mgr)

	state, err := GetInitializationStatus()
	if state != InitStateSuccess {
		t.Errorf("expected InitStateSuccess after SetManager, got %s", state)
	}
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}

	SetManager(nil)
}

func TestGetInitializationStatus_AfterShutdown(t *testing.T) {
	SetManager(nil)
	Initialize()
	Shutdown()

	state, _ := GetInitializationStatus()
	if state != InitStateNotStarted {
		t.Errorf("expected InitStateNotStarted after Shutdown, got %s", state)
	}
}

func TestGetInitializationStatus_InitiallyNotStarted(t *testing.T) {
	SetManager(nil)

	state, _ := GetInitializationStatus()
	if state != InitStateNotStarted {
		t.Errorf("expected InitStateNotStarted initially, got %s", state)
	}
}

func TestInitialize_AfterShutdown_Works(t *testing.T) {
	SetManager(nil)

	if err := Initialize(); err != nil {
		t.Fatalf("first Initialize() failed: %v", err)
	}
	if err := Shutdown(); err != nil {
		t.Fatalf("Shutdown() failed: %v", err)
	}

	if err := Initialize(); err != nil {
		t.Fatalf("re-Initialize() after shutdown failed: %v", err)
	}
	if GetManager() == nil {
		t.Error("expected non-nil manager after re-initialize")
	}

	Shutdown()
}

func TestInitializationState_String(t *testing.T) {
	tests := []struct {
		state    InitializationState
		expected string
	}{
		{InitStateNotStarted, "not-started"},
		{InitStatePending, "pending"},
		{InitStateSuccess, "success"},
		{InitStateFailed, "failed"},
		{InitializationState(99), "unknown"},
	}
	for _, tc := range tests {
		if got := tc.state.String(); got != tc.expected {
			t.Errorf("InitializationState(%d).String() = %q, want %q", tc.state, got, tc.expected)
		}
	}
}

// ─── ChangeFile / SaveFile tests ────────────────────────────────────────────

func TestManager_ChangeFile_NoServerForExtension(t *testing.T) {
	mgr := NewManager()
	err := mgr.ChangeFile("/nonexistent/test.xyz", "content")
	if err != nil {
		t.Errorf("ChangeFile with no server should be no-op, got error: %v", err)
	}
}

func TestManager_SaveFile_NoServerForExtension(t *testing.T) {
	mgr := NewManager()
	err := mgr.SaveFile("/nonexistent/test.xyz")
	if err != nil {
		t.Errorf("SaveFile with no server should be no-op, got error: %v", err)
	}
}

func TestManager_ChangeFile_ServerNotRunning(t *testing.T) {
	mgr := NewManager()
	err := mgr.RegisterServer("test", ServerConfig{Command: "dummy"}, []string{".go"})
	if err != nil {
		t.Fatalf("RegisterServer failed: %v", err)
	}

	err = mgr.ChangeFile("/test/file.go", "content")
	if err != nil {
		t.Errorf("ChangeFile with stopped server should be no-op, got error: %v", err)
	}
}

func TestManager_SaveFile_ServerNotRunning(t *testing.T) {
	mgr := NewManager()
	err := mgr.RegisterServer("test", ServerConfig{Command: "dummy"}, []string{".go"})
	if err != nil {
		t.Fatalf("RegisterServer failed: %v", err)
	}

	err = mgr.SaveFile("/test/file.go")
	if err != nil {
		t.Errorf("SaveFile with stopped server should be no-op, got error: %v", err)
	}
}
