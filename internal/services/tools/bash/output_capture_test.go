package bash

import (
	"testing"
)

func TestParseOutputRedirectionStdout(t *testing.T) {
	info := parseOutputRedirection("echo hello > output.txt")
	if info.Command != "echo hello" {
		t.Fatalf("Command = %q, want %q", info.Command, "echo hello")
	}
	if info.StdoutFile != "output.txt" {
		t.Fatalf("StdoutFile = %q, want %q", info.StdoutFile, "output.txt")
	}
	if info.StderrFile != "" {
		t.Fatalf("StderrFile = %q, want empty", info.StderrFile)
	}
	if info.Append {
		t.Fatal("Append = true, want false")
	}
}

func TestParseOutputRedirectionStdoutAppend(t *testing.T) {
	info := parseOutputRedirection("echo hello >> output.txt")
	if info.Command != "echo hello" {
		t.Fatalf("Command = %q, want %q", info.Command, "echo hello")
	}
	if info.StdoutFile != "output.txt" {
		t.Fatalf("StdoutFile = %q, want %q", info.StdoutFile, "output.txt")
	}
	if !info.Append {
		t.Fatal("Append = false, want true")
	}
}

func TestParseOutputRedirectionStderr(t *testing.T) {
	info := parseOutputRedirection("echo hello 2> error.txt")
	if info.Command != "echo hello" {
		t.Fatalf("Command = %q, want %q", info.Command, "echo hello")
	}
	if info.StderrFile != "error.txt" {
		t.Fatalf("StderrFile = %q, want %q", info.StderrFile, "error.txt")
	}
	if info.StdoutFile != "" {
		t.Fatalf("StdoutFile = %q, want empty", info.StdoutFile)
	}
}

func TestParseOutputRedirectionStderrAppend(t *testing.T) {
	info := parseOutputRedirection("echo hello 2>> error.txt")
	if info.Command != "echo hello" {
		t.Fatalf("Command = %q, want %q", info.Command, "echo hello")
	}
	if info.StderrFile != "error.txt" {
		t.Fatalf("StderrFile = %q, want %q", info.StderrFile, "error.txt")
	}
	if !info.Append {
		t.Fatal("Append = false, want true")
	}
}

func TestParseOutputRedirectionBoth(t *testing.T) {
	info := parseOutputRedirection("echo hello > out.txt 2> err.txt")
	if info.Command != "echo hello" {
		t.Fatalf("Command = %q, want %q", info.Command, "echo hello")
	}
	if info.StdoutFile != "out.txt" {
		t.Fatalf("StdoutFile = %q, want %q", info.StdoutFile, "out.txt")
	}
	if info.StderrFile != "err.txt" {
		t.Fatalf("StderrFile = %q, want %q", info.StderrFile, "err.txt")
	}
}

func TestParseOutputRedirectionQuotedPath(t *testing.T) {
	info := parseOutputRedirection(`echo hello > "my file.txt"`)
	if info.Command != "echo hello" {
		t.Fatalf("Command = %q, want %q", info.Command, "echo hello")
	}
	if info.StdoutFile != "my file.txt" {
		t.Fatalf("StdoutFile = %q, want %q", info.StdoutFile, "my file.txt")
	}
}

func TestParseOutputRedirectionNoRedirect(t *testing.T) {
	info := parseOutputRedirection("echo hello")
	if info.Command != "echo hello" {
		t.Fatalf("Command = %q, want %q", info.Command, "echo hello")
	}
	if info.StdoutFile != "" {
		t.Fatalf("StdoutFile = %q, want empty", info.StdoutFile)
	}
	if info.StderrFile != "" {
		t.Fatalf("StderrFile = %q, want empty", info.StderrFile)
	}
}

func TestParseOutputRedirectionNoSpace(t *testing.T) {
	info := parseOutputRedirection("echo hello>out.txt")
	if info.Command != "echo hello" {
		t.Fatalf("Command = %q, want %q", info.Command, "echo hello")
	}
	if info.StdoutFile != "out.txt" {
		t.Fatalf("StdoutFile = %q, want %q", info.StdoutFile, "out.txt")
	}
}

func TestParseOutputRedirectionPipedCommand(t *testing.T) {
	info := parseOutputRedirection("cat file.txt | grep foo > results.txt")
	if info.Command != "cat file.txt | grep foo" {
		t.Fatalf("Command = %q, want %q", info.Command, "cat file.txt | grep foo")
	}
	if info.StdoutFile != "results.txt" {
		t.Fatalf("StdoutFile = %q, want %q", info.StdoutFile, "results.txt")
	}
}

func TestUnquote(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`"hello"`, "hello"},
		{`'hello'`, "hello"},
		{`hello`, "hello"},
		{`"`, `"`},
		{`""`, ``},
	}
	for _, c := range cases {
		got := unquote(c.in)
		if got != c.want {
			t.Fatalf("unquote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
