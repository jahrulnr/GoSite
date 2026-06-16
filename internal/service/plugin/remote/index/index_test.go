package index

import (
	"testing"
)

func TestSelectAsset(t *testing.T) {
	raw := `{
	  "id": "acme/demo",
	  "distribution": {
	    "releases": [{
	      "version": "1.0.0",
	      "minGoSiteVersion": "1.3.0",
	      "assets": [
	        {"os":"linux","arch":"amd64","url":"https://x/a.zip","sha256":"abc"},
	        {"os":"linux","arch":"arm64","url":"https://x/b.zip","sha256":"def"}
	      ]
	    }]
	  }
	}`
	f, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	asset, rel, err := f.SelectAsset("v1.0.0", "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if asset.SHA256 != "abc" || rel.MinGoSiteVersion != "1.3.0" {
		t.Fatalf("unexpected selection: %+v %+v", asset, rel)
	}
	_, _, err = f.SelectAsset("v1.0.0", "darwin", "amd64")
	if err == nil {
		t.Fatal("expected platform error")
	}
}
