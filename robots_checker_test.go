package main

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRobotsChecker_FallsBackToAllowOnBadString(t *testing.T) {
	t.Parallel()
	rc := &RobotsChecker{}
	err := rc.LoadRobots(string([]byte{0xff, 0xfe}))
	assert.NoError(t, err)
	assert.True(t, rc.IsAllowed("/some/path", "Googlebot"))
}

func TestRobotsChecker_FailsUserAgentNotAllowed(t *testing.T) {
	t.Parallel()
	rc := &RobotsChecker{}
	err := rc.LoadRobots("User-agent: *\nDisallow: /")
	require.NoError(t, err)
	assert.False(t, rc.IsAllowed("/some/path", "Google"))
}

func TestRobotsChecker_DisallowsPath(t *testing.T) {
	t.Parallel()
	rc := &RobotsChecker{}
	err := rc.LoadRobots("User-Agent: *\r\nDisallow: /account\r\n")
	require.NoError(t, err)
	assert.False(t, rc.IsAllowed("/account", "Test"))
	assert.False(t, rc.IsAllowed("/account/sub", "Test"))
}

func TestNewRobotsChecker_LoadsRobots(t *testing.T) {
	t.Parallel()
	robotsTxt := "User-agent: *\nDisallow: /private\nAllow: /public"
	rc, err := NewRobotsChecker(robotsTxt)
	require.NoError(t, err)
	assert.NotNil(t, rc)
	assert.True(t, rc.IsAllowed("/public", "TestBot"))
	assert.False(t, rc.IsAllowed("/private", "TestBot"))
	assert.True(t, rc.IsAllowed("/public/allowed", "TestBot"))
	assert.False(t, rc.IsAllowed("/private/forbidden", "TestBot"))
}
