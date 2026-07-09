package pluginperm_test

import (
	"testing"

	"github.com/jahrulnr/gosite/pkg/pluginperm"
	"github.com/stretchr/testify/assert"
)

func TestSubsetOfManifest(t *testing.T) {
	manifest := []string{"websites:read", "nginx:read", "nginx:manage"}
	assert.True(t, pluginperm.SubsetOfManifest([]string{"websites:read"}, manifest))
	assert.False(t, pluginperm.SubsetOfManifest([]string{"docker:manage"}, manifest))
}

func TestIntersect(t *testing.T) {
	got := pluginperm.Intersect(
		[]string{"websites:read", "docker:manage"},
		[]string{"websites:read", "nginx:read"},
	)
	assert.Equal(t, []string{"websites:read"}, got)
}

func TestValid(t *testing.T) {
	assert.True(t, pluginperm.Valid("system:read"))
	assert.False(t, pluginperm.Valid("unknown:scope"))
}
