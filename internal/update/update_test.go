package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// fakeGitHub serves a latest release whose installer is setup and whose checksums file says sum.
func fakeGitHub(t *testing.T, tag, setup, sum string, assets ...string) Checker {
	t.Helper()
	if assets == nil {
		assets = []string{SetupAsset, ChecksumsAsset}
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repo/releases/latest":
			var list []string
			for _, a := range assets {
				list = append(list, fmt.Sprintf(`{"name":%q,"browser_download_url":"%s/dl/%s"}`, a, srv.URL, a))
			}
			fmt.Fprintf(w, `{"tag_name":%q,"assets":[%s]}`, tag, strings.Join(list, ","))
		case "/dl/" + SetupAsset:
			fmt.Fprint(w, setup)
		case "/dl/" + ChecksumsAsset:
			fmt.Fprintf(w, "%s  wtfsync.exe\n%s  %s\n", strings.Repeat("0", 64), sum, SetupAsset)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return Checker{Client: srv.Client(), API: srv.URL + "/repo"}
}

func sha(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func TestLatestFindsInstallerAndChecksums(t *testing.T) {
	c := fakeGitHub(t, "v0.5.0", "MZ", sha("MZ"))
	r, err := c.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.Version != "v0.5.0" || !strings.HasSuffix(r.SetupURL, "/dl/"+SetupAsset) || !strings.HasSuffix(r.ChecksumsURL, "/dl/"+ChecksumsAsset) {
		t.Fatalf("release %+v", r)
	}
}

func TestLatestWithoutInstallerIsAnError(t *testing.T) {
	c := fakeGitHub(t, "v0.5.0", "MZ", sha("MZ"), "wtfsync.exe", ChecksumsAsset)
	if _, err := c.Latest(context.Background()); err == nil || !strings.Contains(err.Error(), "no installer") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewer(t *testing.T) {
	for _, tt := range []struct {
		current, latest string
		want            bool
	}{
		{"v0.4.0", "v0.5.0", true},
		{"v0.4.0", "v0.4.1", true},
		{"v0.9.9", "v1.0.0", true},
		{"v0.10.0", "v0.9.0", false}, // numeric, not string, order
		{"v0.5.0", "v0.5.0", false},
		{"v0.5.0", "v0.4.9", false},
		{"dev", "v9.9.9", false}, // development builds never update
		{"v0.5.0", "nightly", false},
	} {
		if got := Newer(tt.current, tt.latest); got != tt.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}

func TestDownloadVerifiesChecksum(t *testing.T) {
	c := fakeGitHub(t, "v0.5.0", "MZ-installer", sha("MZ-installer"))
	r, _ := c.Latest(context.Background())
	dir := t.TempDir()
	path, err := c.Download(context.Background(), r, dir)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "MZ-installer" {
		t.Fatalf("installer = %q", b)
	}
}

func TestDownloadRejectsChecksumMismatch(t *testing.T) {
	c := fakeGitHub(t, "v0.5.0", "tampered", sha("MZ-installer"))
	r, _ := c.Latest(context.Background())
	dir := t.TempDir()
	if _, err := c.Download(context.Background(), r, dir); !errors.Is(err, ErrChecksum) {
		t.Fatalf("err = %v, want ErrChecksum", err)
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Fatalf("left behind: %v", left)
	}
}
