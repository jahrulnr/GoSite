package regression_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/website"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Legacy bug: delete clean flag was always true due to ($remove !== null || $remove !== false).
func TestRegression_DeleteCleanFalse_KeepsDocumentRoot(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	path := filepath.Join(stack.WebRoot, "regression-keep")
	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "regression-keep.example.com",
		Path:   path,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(path, "marker.txt"), []byte("x"), 0o644))

	require.NoError(t, stack.WebsiteSvc.Delete(ctx, site.ID, false))
	_, err = os.Stat(filepath.Join(path, "marker.txt"))
	require.NoError(t, err, "clean=false must keep files (legacy always deleted)")
}

// Legacy bug: enable/disable did not reload nginx.
func TestRegression_ToggleCallsReload(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "regression-toggle.example.com",
		Path:   filepath.Join(stack.WebRoot, "regression-toggle"),
		Active: false,
	})
	require.NoError(t, err)

	before := len(stack.Cmd.SnapshotCalls())
	_, err = stack.WebsiteSvc.Toggle(ctx, site.ID)
	require.NoError(t, err)

	reloadSeen := false
	for _, call := range stack.Cmd.SnapshotCalls()[before:] {
		if call.Name == "nginx" {
			for _, arg := range call.Args {
				if arg == "reload" || arg == "-s" {
					reloadSeen = true
					break
				}
			}
		}
	}
	assert.True(t, reloadSeen, "toggle must reload nginx (legacy skipped reload)")
}

// Legacy bug: Validator::make args reversed — invalid domain must fail validation.
func TestRegression_InvalidDomainRejected(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	result := stack.WebsiteSvc.Validate(ctx, website.ValidateInput{
		Domain: "not a valid domain!!!",
		Path:   filepath.Join(stack.WebRoot, "bad"),
		Type:   sqlite.WebsiteTypeStatic,
	})
	assert.False(t, result.Valid)
}
