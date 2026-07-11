package filesystem_test

import (
	"testing"

	"github.com/jahrulnr/gosite/internal/infra/filesystem"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_AllowsWWW(t *testing.T) {
	v := filesystem.NewValidator("/www", "/storage", "/tmp")
	require.NoError(t, v.Validate("/www/site/index.html"))
}

func TestValidate_AllowsStorage(t *testing.T) {
	v := filesystem.NewValidator("/www", "/storage", "/tmp")
	require.NoError(t, v.Validate("/storage/webconfig/site.conf"))
}

func TestValidate_AllowsTmp(t *testing.T) {
	v := filesystem.NewValidator("/www", "/storage", "/tmp")
	require.NoError(t, v.Validate("/tmp/upload.txt"))
}

func TestValidate_RejectsTraversal(t *testing.T) {
	v := filesystem.NewValidator("/www", "/storage", "/tmp")
	err := v.Validate("/www/../etc/passwd")
	require.Error(t, err)
	appErr := apperror.From(err)
	assert.Equal(t, apperror.CodePathTraversal, appErr.Code)
}

func TestValidate_AllowsOutsideRoots(t *testing.T) {
	v := filesystem.NewValidator("/www", "/storage", "/tmp")
	require.NoError(t, v.Validate("/etc/passwd"))
}

func TestValidate_RejectsEmpty(t *testing.T) {
	v := filesystem.NewValidator("/www")
	err := v.Validate("")
	require.Error(t, err)
}

func TestValidate_RejectsRelative(t *testing.T) {
	v := filesystem.NewValidator("/www")
	err := v.Validate("relative/path")
	require.Error(t, err)
}

func TestResolve_AllowsExactRoot(t *testing.T) {
	v := filesystem.NewValidator("/www")
	path, err := v.Resolve("/www")
	require.NoError(t, err)
	assert.Equal(t, "/www", path)
}

func TestValidate_RootAllowsAbsolutePaths(t *testing.T) {
	v := filesystem.NewValidator("/")
	require.NoError(t, v.Validate("/etc/passwd"))
	require.NoError(t, v.Validate("/tmp/upload.txt"))
}

func TestDefaultAllowRoots_AllowsAnyAbsolutePath(t *testing.T) {
	assert.Contains(t, filesystem.DefaultAllowRoots, "/")
}

func TestValidate_RejectsEncodedTraversal(t *testing.T) {
	v := filesystem.NewValidator("/storage")
	err := v.Validate("/storage/foo/../../etc/passwd")
	require.Error(t, err)
	assert.Equal(t, apperror.CodePathTraversal, apperror.From(err).Code)
}

func TestValidate_AllowsNestedUnderWWW(t *testing.T) {
	v := filesystem.NewValidator("/www")
	require.NoError(t, v.Validate("/www/a/b/c.txt"))
}
