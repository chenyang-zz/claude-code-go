package query

import (
	"github.com/google/uuid"
)

// QueryDeps collects I/O dependencies for query(). Passing a deps override
// into QueryParams lets tests inject fakes directly instead of mocking per-module.
type QueryDeps struct {
	// UUID returns a new random UUID string.
	UUID func() string
}

// ProductionDeps returns the production implementation of QueryDeps.
func ProductionDeps() QueryDeps {
	return QueryDeps{
		UUID: func() string { return uuid.New().String() },
	}
}
