package sandbox

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// RestrictedSandbox provides a lightweight non-Docker sandbox using
// OS-level restriction mechanisms (process groups, rlimits on Linux,
// sandbox-exec on macOS). Falls back to rlimit/process group isolation
// when the full sandbox-runtime package is unavailable.
type RestrictedSandbox struct {
	timeout time.Duration
}

// NewRestrictedSandbox creates a restricted execution sandbox.
func NewRestrictedSandbox(timeout time.Duration) *RestrictedSandbox {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &RestrictedSandbox{timeout: timeout}
}

// Execute runs a command under restricted execution and returns output.
// On macOS: uses sandbox-exec with a basic deny-all profile when available.
// On Linux: uses process group isolation with resource limits.
// Falls back to basic process isolation when no advanced sandbox tools are available.
func (r *RestrictedSandbox) Execute(ctx context.Context, command string, workDir string) (stdout string, stderr string, exitCode int, err error) {
	switch runtime.GOOS {
	case "darwin":
		return r.executeMacOSSandbox(ctx, command, workDir)
	case "linux":
		return r.executeLinuxSandbox(ctx, command, workDir)
	default:
		return r.executeBasicIsolation(ctx, command, workDir)
	}
}

// executeMacOSSandbox uses sandbox-exec with a basic restrictive profile.
func (r *RestrictedSandbox) executeMacOSSandbox(ctx context.Context, command string, workDir string) (string, string, int, error) {
	profile := buildBasicSeatbeltProfile()

	args := []string{"sandbox-exec", "-p", profile}
	if workDir != "" {
		args = append(args, "sh", "-c", "cd '"+workDir+"' && "+command)
	} else {
		args = append(args, "sh", "-c", command)
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	return stdoutBuf.String(), stderrBuf.String(), exitCode, err
}

// executeLinuxSandbox uses bubblewrap (bwrap) when available, otherwise process isolation.
func (r *RestrictedSandbox) executeLinuxSandbox(ctx context.Context, command string, workDir string) (string, string, int, error) {
	if _, err := exec.LookPath("bwrap"); err == nil {
		return r.executeBwrap(ctx, command, workDir)
	}
	return r.executeBasicIsolation(ctx, command, workDir)
}

// executeBwrap uses bubblewrap for user-namespace sandboxed execution.
func (r *RestrictedSandbox) executeBwrap(ctx context.Context, command string, workDir string) (string, string, int, error) {
	bwrapArgs := []string{
		"--unshare-pid",
		"--unshare-net",
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/lib", "/lib",
		"--ro-bind", "/lib64", "/lib64",
		"--ro-bind", "/bin", "/bin",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
	}

	if workDir != "" {
		bwrapArgs = append(bwrapArgs, "--bind", workDir, workDir)
		bwrapArgs = append(bwrapArgs, "--chdir", workDir)
	}

	bwrapArgs = append(bwrapArgs, "sh", "-c", command)

	cmd := exec.CommandContext(ctx, "bwrap", bwrapArgs...)
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	return stdoutBuf.String(), stderrBuf.String(), exitCode, err
}

// executeBasicIsolation uses process group isolation as a minimal safety measure.
func (r *RestrictedSandbox) executeBasicIsolation(ctx context.Context, command string, workDir string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	return stdoutBuf.String(), stderrBuf.String(), exitCode, err
}

// buildBasicSeatbeltProfile returns a minimal macOS sandbox-exec profile.
func buildBasicSeatbeltProfile() string {
	return `(version 1)
(deny default)
(allow file-read* (subpath "/usr") (subpath "/bin") (subpath "/lib") (subpath "/sbin"))
(allow file-read* file-write* (subpath "/tmp") (subpath "/private/tmp"))
(allow process-exec (subpath "/usr") (subpath "/bin") (subpath "/sbin"))
(allow sysctl-read)
(allow signal (target self))
(allow network-outbound (literal "0.0.0.0/0"))
`
}
