package fs

import "os"

// CWDResolver provides a stable working-directory fallback for tool invocations.
type CWDResolver struct {
	// original stores the process working directory captured during bootstrap.
	original string
}

// NewCWDResolver captures the current process working directory as the stable fallback.
func NewCWDResolver() (*CWDResolver, error) {
	original, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return &CWDResolver{original: original}, nil
}

// NewCWDResolverWithOriginal builds a resolver from an already known original working directory.
func NewCWDResolverWithOriginal(original string) *CWDResolver {
	return &CWDResolver{original: original}
}

// Original returns the resolver's bootstrap working directory fallback.
func (r *CWDResolver) Original() string {
	if r == nil {
		return ""
	}

	return r.original
}

// GetCwd returns the call-scoped working directory when present, otherwise the bootstrap fallback.
func (r *CWDResolver) GetCwd(override string) string {
	if override != "" {
		return override
	}

	return r.Original()
}
