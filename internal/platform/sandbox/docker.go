package sandbox

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// DockerSandbox provides Docker container-based sandbox execution.
// It wraps commands in a disposable container with resource limits.
type DockerSandbox struct {
	image    string
	memory   string
	cpuCount int64
	timeout  time.Duration
	network  bool
}

// NewDockerSandbox creates a Docker sandbox executor with the given options.
func NewDockerSandbox(image string, memory string, cpuCount int64, timeout time.Duration, network bool) *DockerSandbox {
	if image == "" {
		image = "ubuntu:22.04"
	}
	if memory == "" {
		memory = "512m"
	}
	if cpuCount <= 0 {
		cpuCount = 2
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &DockerSandbox{
		image:    image,
		memory:   memory,
		cpuCount: cpuCount,
		timeout:  timeout,
		network:  network,
	}
}

// Execute runs a command inside a Docker container and returns the output.
func (d *DockerSandbox) Execute(ctx context.Context, command string, workDir string) (stdout string, stderr string, exitCode int, err error) {
	// Build docker run arguments
	args := []string{
		"run", "--rm",
		"-i",
		"--memory", d.memory,
		"--cpus", fmt.Sprintf("%d", d.cpuCount),
	}

	// Network policy
	if !d.network {
		args = append(args, "--network", "none")
	}

	// Mount working directory if specified
	if workDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s", workDir, workDir))
		args = append(args, "-w", workDir)
	}

	// Wrap command with timeout inside the container when timeout is set
	timeoutSec := int(d.timeout.Seconds())
	if timeoutSec > 0 {
		// Use timeout command inside the container
		args = append(args, d.image, "timeout", fmt.Sprintf("%d", timeoutSec), "sh", "-c", command)
	} else {
		args = append(args, d.image, "sh", "-c", command)
	}

	// Execute docker run
	cmd := exec.CommandContext(ctx, "docker", args...)

	var stdoutBuf strings.Builder
	var stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			err = nil
		}
	} else {
		exitCode = 0
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode, err
}

// Cleanup removes stopped containers (no-op for --rm containers).
func (d *DockerSandbox) Cleanup() error {
	return nil
}

// IsDockerAvailable checks if docker is installed and running.
func IsDockerAvailable() bool {
	cmd := exec.Command("docker", "info", "--format", "{{.ServerVersion}}")
	return cmd.Run() == nil
}
