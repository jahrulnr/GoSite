package website_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/website"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreate_StaticSite_ProvisionPathAndConfig(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	path := filepath.Join(stack.WebRoot, "static-site")
	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "static.example.com",
		Path:   path,
		Type:   sqlite.WebsiteTypeStatic,
	})
	require.NoError(t, err)
	assert.NotZero(t, site.ID)

	_, err = os.Stat(filepath.Join(path, "index.html"))
	require.NoError(t, err)

	cfg, err := stack.Nginx.ReadSiteConfig(ctx, site.Domain)
	require.NoError(t, err)
	assert.Contains(t, cfg, "root "+path)
}

func TestCreate_ProxyType_UpstreamInConfig(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	upstream := "http://127.0.0.1:9000"
	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain:   "proxy.example.com",
		Path:     filepath.Join(stack.WebRoot, "proxy-site"),
		Type:     sqlite.WebsiteTypeProxy,
		Upstream: upstream,
	})
	require.NoError(t, err)

	cfg, err := stack.Nginx.ReadSiteConfig(ctx, site.Domain)
	require.NoError(t, err)
	assert.Contains(t, cfg, "proxy_pass "+upstream)
}

func TestDelete_CleanFalse_KeepsFiles(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	path := filepath.Join(stack.WebRoot, "keep-files")
	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "keep.example.com",
		Path:   path,
	})
	require.NoError(t, err)

	require.NoError(t, stack.WebsiteSvc.Delete(ctx, site.ID, false))

	_, err = os.Stat(path)
	require.NoError(t, err)
}

func TestDelete_CleanTrue_RemovesFiles(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	path := filepath.Join(stack.WebRoot, "remove-files")
	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "remove.example.com",
		Path:   path,
	})
	require.NoError(t, err)

	require.NoError(t, stack.WebsiteSvc.Delete(ctx, site.ID, true))

	_, err = os.Stat(path)
	require.Error(t, err)
}

func TestToggle_ReloadFail_Rollback(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "toggle.example.com",
		Path:   filepath.Join(stack.WebRoot, "toggle"),
		Active: false,
	})
	require.NoError(t, err)

	stack.Cmd.ReloadErr = errors.New("reload failed")
	_, err = stack.WebsiteSvc.Toggle(ctx, site.ID)
	require.Error(t, err)

	updated, err := stack.WebsiteSvc.Get(ctx, site.ID)
	require.NoError(t, err)
	assert.False(t, updated.Active)

	activePath := filepath.Join(stack.Nginx.Paths().ActiveD, site.Domain+".conf")
	_, err = os.Lstat(activePath)
	require.Error(t, err)

	stack.Cmd.ReloadErr = nil
}

func TestValidate_RejectsTraversal(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	result := stack.WebsiteSvc.Validate(ctx, "bad.example.com", stack.WebRoot+"/../etc", 0)
	assert.False(t, result.Valid)
}

func TestValidate_RejectsDuplicatePath(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	path := filepath.Join(stack.WebRoot, "dup")
	_, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "first.example.com",
		Path:   path,
	})
	require.NoError(t, err)

	result := stack.WebsiteSvc.Validate(ctx, "second.example.com", path, 0)
	assert.False(t, result.Valid)
}

func TestUpdateNginxConfig_BackupAndReload(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "nginxcfg.example.com",
		Path:   filepath.Join(stack.WebRoot, "nginxcfg"),
	})
	require.NoError(t, err)

	newCfg := strings.Replace(testutil.SampleNginxSiteConfig, "example.test", site.Domain, 1)
	require.NoError(t, stack.WebsiteSvc.UpdateNginxConfig(ctx, site.ID, newCfg))

	backups, err := os.ReadDir(stack.Nginx.Paths().Backups)
	require.NoError(t, err)
	assert.NotEmpty(t, backups)
}

func TestListAndGet(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	created, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "list.example.com",
		Path:   filepath.Join(stack.WebRoot, "list"),
	})
	require.NoError(t, err)

	list, err := stack.WebsiteSvc.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	got, err := stack.WebsiteSvc.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.Domain, got.Domain)
}

func TestCreate_RejectsInvalidDomain(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	_, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "not-a-valid-domain",
		Path:   filepath.Join(stack.WebRoot, "bad"),
	})
	require.Error(t, err)
}

func TestToggle_EnablesSymlinkWhenActive(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "active-toggle.example.com",
		Path:   filepath.Join(stack.WebRoot, "active-toggle"),
		Active: false,
	})
	require.NoError(t, err)

	toggled, err := stack.WebsiteSvc.Toggle(ctx, site.ID)
	require.NoError(t, err)
	assert.True(t, toggled.Active)

	activePath := filepath.Join(stack.Nginx.Paths().ActiveD, site.Domain+".conf")
	_, err = os.Lstat(activePath)
	require.NoError(t, err)
}

func TestUpdate_ChangesDomainAndConfig(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "before.example.com",
		Path:   filepath.Join(stack.WebRoot, "before"),
	})
	require.NoError(t, err)

	newPath := filepath.Join(stack.WebRoot, "after")
	updated, err := stack.WebsiteSvc.Update(ctx, site.ID, website.UpdateInput{
		Domain: "after.example.com",
		Path:   newPath,
		Active: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "after.example.com", updated.Domain)

	cfg, err := stack.Nginx.ReadSiteConfig(ctx, updated.Domain)
	require.NoError(t, err)
	assert.Contains(t, cfg, "after.example.com")
}

func TestUpdate_ProxyRequiresUpstream(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "proxy-up.example.com",
		Path:   filepath.Join(stack.WebRoot, "proxy-up"),
	})
	require.NoError(t, err)

	_, err = stack.WebsiteSvc.Update(ctx, site.ID, website.UpdateInput{
		Domain: site.Domain,
		Path:   site.Path,
		Type:   sqlite.WebsiteTypeProxy,
		Active: true,
	})
	require.Error(t, err)
}

func TestGetNginxConfig_ReturnsContent(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "getcfg.example.com",
		Path:   filepath.Join(stack.WebRoot, "getcfg"),
	})
	require.NoError(t, err)

	content, err := stack.WebsiteSvc.GetNginxConfig(ctx, site.ID)
	require.NoError(t, err)
	assert.Contains(t, content, site.Domain)
}

func TestGet_NotFound(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	_, err := stack.WebsiteSvc.Get(context.Background(), 99999)
	require.Error(t, err)
}

func TestFormatToggleMessage(t *testing.T) {
	assert.Equal(t, "Site actived successfully", website.FormatToggleMessage(true))
	assert.Equal(t, "Site disabled successfully", website.FormatToggleMessage(false))
}

func TestCreate_ProxyWithoutUpstreamRejected(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	_, err := stack.WebsiteSvc.Create(context.Background(), website.CreateInput{
		Domain: "bad-proxy.example.com",
		Path:   filepath.Join(stack.WebRoot, "bad-proxy"),
		Type:   sqlite.WebsiteTypeProxy,
	})
	require.Error(t, err)
}

func TestCreate_ActiveSite_EnablesAndReloads(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "active-create.example.com",
		Path:   filepath.Join(stack.WebRoot, "active-create"),
		Active: true,
	})
	require.NoError(t, err)
	assert.True(t, site.Active)

	activePath := filepath.Join(stack.Nginx.Paths().ActiveD, site.Domain+".conf")
	_, err = os.Lstat(activePath)
	require.NoError(t, err)
}

func TestCreate_ActiveReloadFail_RollsBack(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()
	stack.Cmd.ReloadErr = errors.New("reload failed")

	_, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "rollback.example.com",
		Path:   filepath.Join(stack.WebRoot, "rollback"),
		Active: true,
	})
	require.Error(t, err)
	stack.Cmd.ReloadErr = nil
}

func TestDelete_NotFound(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	err := stack.WebsiteSvc.Delete(context.Background(), 99999, false)
	require.Error(t, err)
}

func TestToggle_DisableActiveSite(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "disable-toggle.example.com",
		Path:   filepath.Join(stack.WebRoot, "disable-toggle"),
		Active: true,
	})
	require.NoError(t, err)

	toggled, err := stack.WebsiteSvc.Toggle(ctx, site.ID)
	require.NoError(t, err)
	assert.False(t, toggled.Active)
}

func TestValidate_RejectsPathThatIsFile(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	filePath := filepath.Join(stack.WebRoot, "file-not-dir")
	require.NoError(t, os.WriteFile(filePath, []byte("x"), 0o644))

	result := stack.WebsiteSvc.Validate(ctx, "file.example.com", filePath, 0)
	assert.False(t, result.Valid)
}

func TestGetNginxConfig_NotFound(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	site, err := stack.WebsiteRepo.Create(context.Background(), sqlite.Website{
		Name: "noconf", Domain: "noconf2.example.com", Path: filepath.Join(stack.WebRoot, "noconf2"),
	})
	require.NoError(t, err)
	_, err = stack.WebsiteSvc.GetNginxConfig(context.Background(), site.ID)
	require.Error(t, err)
}

func TestUpdate_NotFound(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	_, err := stack.WebsiteSvc.Update(context.Background(), 99999, website.UpdateInput{
		Domain: "x.example.com", Path: filepath.Join(stack.WebRoot, "x"),
	})
	require.Error(t, err)
}

func TestUpdateNginxConfig_NotFound(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	err := stack.WebsiteSvc.UpdateNginxConfig(context.Background(), 99999, "server {}")
	require.Error(t, err)
}

func TestToggle_NotFound(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	_, err := stack.WebsiteSvc.Toggle(context.Background(), 99999)
	require.Error(t, err)
}

func TestCreate_WithCustomName(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	site, err := stack.WebsiteSvc.Create(context.Background(), website.CreateInput{
		Name:   "My Site",
		Domain: "named.example.com",
		Path:   filepath.Join(stack.WebRoot, "named"),
	})
	require.NoError(t, err)
	assert.Equal(t, "My Site", site.Name)
}

func TestValidate_AcceptsValidDomain(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	result := stack.WebsiteSvc.Validate(context.Background(), "valid.example.com",
		filepath.Join(stack.WebRoot, "valid"), 0)
	assert.True(t, result.Valid)
}

func TestUpdate_DeactivatesSite(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "deactivate.example.com",
		Path:   filepath.Join(stack.WebRoot, "deactivate"),
		Active: true,
	})
	require.NoError(t, err)

	updated, err := stack.WebsiteSvc.Update(ctx, site.ID, website.UpdateInput{
		Domain: site.Domain,
		Path:   site.Path,
		Active: false,
	})
	require.NoError(t, err)
	assert.False(t, updated.Active)

	activePath := filepath.Join(stack.Nginx.Paths().ActiveD, site.Domain+".conf")
	_, err = os.Lstat(activePath)
	require.Error(t, err)
}

func TestUpdate_ProxyWithUpstream(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "to-proxy.example.com",
		Path:   filepath.Join(stack.WebRoot, "to-proxy"),
	})
	require.NoError(t, err)

	updated, err := stack.WebsiteSvc.Update(ctx, site.ID, website.UpdateInput{
		Domain:   site.Domain,
		Path:     site.Path,
		Type:     sqlite.WebsiteTypeProxy,
		Upstream: "http://127.0.0.1:8081",
		Active:   true,
	})
	require.NoError(t, err)
	assert.Equal(t, sqlite.WebsiteTypeProxy, updated.Type)

	cfg, err := stack.Nginx.ReadSiteConfig(ctx, site.Domain)
	require.NoError(t, err)
	assert.Contains(t, cfg, "proxy_pass")
}

func TestDelete_ActiveSite_RemovesConfig(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "delete-active.example.com",
		Path:   filepath.Join(stack.WebRoot, "delete-active"),
		Active: true,
	})
	require.NoError(t, err)

	require.NoError(t, stack.WebsiteSvc.Delete(ctx, site.ID, false))

	_, err = stack.Nginx.ReadSiteConfig(ctx, site.Domain)
	require.Error(t, err)
}

func TestDelete_CleanWhenPathMissing(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	path := filepath.Join(stack.WebRoot, "gone-path")
	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "gone-path.example.com",
		Path:   path,
	})
	require.NoError(t, err)
	require.NoError(t, os.RemoveAll(path))

	require.NoError(t, stack.WebsiteSvc.Delete(ctx, site.ID, true))
}

func TestToggle_ReloadFail_RollbackFromActive(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	site, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
		Domain: "rollback-active.example.com",
		Path:   filepath.Join(stack.WebRoot, "rollback-active"),
		Active: true,
	})
	require.NoError(t, err)

	stack.Cmd.ReloadErr = errors.New("reload failed")
	_, err = stack.WebsiteSvc.Toggle(ctx, site.ID)
	require.Error(t, err)

	updated, err := stack.WebsiteSvc.Get(ctx, site.ID)
	require.NoError(t, err)
	assert.True(t, updated.Active)
	stack.Cmd.ReloadErr = nil
}

func TestValidate_RejectsDoubleDotDomain(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	result := stack.WebsiteSvc.Validate(context.Background(), "bad..example.com",
		filepath.Join(stack.WebRoot, "bad-domain"), 0)
	assert.False(t, result.Valid)
}

func TestValidate_RejectsEmptyPath(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	result := stack.WebsiteSvc.Validate(context.Background(), "ok.example.com", "", 0)
	assert.False(t, result.Valid)
}

func TestList_MultipleSites(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()
	for i, domain := range []string{"one.example.com", "two.example.com"} {
		_, err := stack.WebsiteSvc.Create(ctx, website.CreateInput{
			Domain: domain,
			Path:   filepath.Join(stack.WebRoot, "multi", fmt.Sprintf("site%d", i+1)),
		})
		require.NoError(t, err)
	}
	list, err := stack.WebsiteSvc.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}
