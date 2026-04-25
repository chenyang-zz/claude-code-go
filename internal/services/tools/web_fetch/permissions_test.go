package web_fetch

import (
	"testing"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	"github.com/stretchr/testify/assert"
)

func TestPermissionChecker_PreapprovedHost(t *testing.T) {
	c := NewPermissionChecker(nil, nil, nil)
	eval := c.Check("https://docs.python.org/3/library")
	assert.Equal(t, corepermission.DecisionAllow, eval.Decision)
}

func TestPermissionChecker_DenyRule(t *testing.T) {
	c := NewPermissionChecker(nil, []string{"example.com"}, nil)
	eval := c.Check("https://example.com/page")
	assert.Equal(t, corepermission.DecisionDeny, eval.Decision)
}

func TestPermissionChecker_AskRule(t *testing.T) {
	c := NewPermissionChecker(nil, nil, []string{"example.com"})
	eval := c.Check("https://example.com/page")
	assert.Equal(t, corepermission.DecisionAsk, eval.Decision)
}

func TestPermissionChecker_AllowRule(t *testing.T) {
	c := NewPermissionChecker([]string{"example.com"}, nil, nil)
	eval := c.Check("https://example.com/page")
	assert.Equal(t, corepermission.DecisionAllow, eval.Decision)
}

func TestPermissionChecker_DefaultAsk(t *testing.T) {
	c := NewPermissionChecker(nil, nil, nil)
	eval := c.Check("https://unknown.com/page")
	assert.Equal(t, corepermission.DecisionAsk, eval.Decision)
}

func TestPermissionChecker_DenyOverridesAllow(t *testing.T) {
	c := NewPermissionChecker([]string{"example.com"}, []string{"example.com"}, nil)
	eval := c.Check("https://example.com/page")
	assert.Equal(t, corepermission.DecisionDeny, eval.Decision)
}

func TestPermissionChecker_PreapprovedOverridesDeny(t *testing.T) {
	c := NewPermissionChecker(nil, []string{"docs.python.org"}, nil)
	eval := c.Check("https://docs.python.org/3/library")
	assert.Equal(t, corepermission.DecisionAllow, eval.Decision)
}

func TestPermissionChecker_GitHubPathPrefix(t *testing.T) {
	c := NewPermissionChecker(nil, nil, nil)
	// github.com is only preapproved for /anthropics paths
	eval := c.Check("https://github.com/anthropics/claude-code")
	assert.Equal(t, corepermission.DecisionAllow, eval.Decision)

	eval = c.Check("https://github.com/other/repo")
	assert.Equal(t, corepermission.DecisionAsk, eval.Decision)
}

func TestPermissionChecker_DomainFormatRule(t *testing.T) {
	c := NewPermissionChecker([]string{"domain:example.com"}, nil, nil)
	eval := c.Check("https://example.com/page")
	assert.Equal(t, corepermission.DecisionAllow, eval.Decision)
}

func TestPermissionChecker_WildcardRule(t *testing.T) {
	c := NewPermissionChecker([]string{"*"}, nil, nil)
	eval := c.Check("https://anything.com/page")
	assert.Equal(t, corepermission.DecisionAllow, eval.Decision)
}
