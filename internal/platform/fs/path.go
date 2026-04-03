package fs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var errPathContainsNullBytes = errors.New("path contains null bytes")

// ExpandPath normalizes user input into an absolute path using the provided base directory.
func ExpandPath(path string, baseDir string) (string, error) {
	actualBaseDir := baseDir
	if actualBaseDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}

		actualBaseDir = cwd
	}

	if strings.ContainsRune(path, '\x00') || strings.ContainsRune(actualBaseDir, '\x00') {
		return "", errPathContainsNullBytes
	}

	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return filepath.Clean(actualBaseDir), nil
	}

	if trimmedPath == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		return filepath.Clean(homeDir), nil
	}

	if strings.HasPrefix(trimmedPath, "~/") || strings.HasPrefix(trimmedPath, "~\\") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		return filepath.Clean(filepath.Join(homeDir, trimmedPath[2:])), nil
	}

	if filepath.IsAbs(trimmedPath) {
		return filepath.Clean(trimmedPath), nil
	}

	return filepath.Clean(filepath.Join(actualBaseDir, trimmedPath)), nil
}

// ToRelativePath shortens an absolute path when it is still inside the caller-visible working directory.
func ToRelativePath(absolutePath string, workingDir string) string {
	if absolutePath == "" {
		return ""
	}

	actualWorkingDir := workingDir
	if actualWorkingDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return absolutePath
		}

		actualWorkingDir = cwd
	}

	relativePath, err := filepath.Rel(actualWorkingDir, absolutePath)
	if err != nil {
		return absolutePath
	}

	parentPrefix := ".." + string(filepath.Separator)
	if relativePath == ".." || strings.HasPrefix(relativePath, parentPrefix) {
		return absolutePath
	}

	return relativePath
}
