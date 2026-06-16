package nginx

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReplaceHttpDIncludeForTest_UsesAbsoluteTempPath(t *testing.T) {
	t.Parallel()

	base := "http {\n\tinclude /etc/nginx/http.d/*.conf;\n}\n"
	got := replaceHttpDIncludeForTest(base, "/etc/nginx/http.d", "/tmp/nginx-default-test.conf")

	assert.Contains(t, got, "include /tmp/nginx-default-test.conf;")
	assert.NotContains(t, got, "http.d//tmp")
}

func TestReplaceSiteIncludeForTest_UsesAbsoluteTempPath(t *testing.T) {
	t.Parallel()

	base := "http {\n\tinclude /storage/webconfig/site.d/*.conf;\n}\n"
	got := replaceSiteIncludeForTest(base, "/storage/webconfig/site.d", "/tmp/nginx-site-test-example.com.conf")

	assert.Contains(t, got, "include /tmp/nginx-site-test-example.com.conf;")
	assert.NotContains(t, got, "webconfig//tmp")
}

func TestReplaceSiteIncludeForTest_PrefersConfiguredSiteD(t *testing.T) {
	t.Parallel()

	siteD := "/data/storage/webconfig/site.d"
	base := "include " + siteD + "/*.conf;\n"
	got := replaceSiteIncludeForTest(base, siteD, "/tmp/site.conf")

	assert.Equal(t, "include /tmp/site.conf;\n", got)
}

func TestReplaceSiteIncludeForTest_LegacyFallback(t *testing.T) {
	t.Parallel()

	base := "include /storage/webconfig/site.d/*.conf;\n"
	got := replaceSiteIncludeForTest(base, "/other/site.d", "/tmp/site.conf")

	assert.True(t, strings.Contains(got, "/tmp/site.conf"))
}
