// Package update finds newer wtfsync releases on GitHub and downloads their installer.
package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	SetupAsset     = "wtfsync-setup.exe"
	ChecksumsAsset = "checksums.txt"
)

var ErrChecksum = errors.New("downloaded installer doesn't match its checksum")

// Release is a published version that has an installer.
type Release struct {
	Version      string // "v0.5.0"
	SetupURL     string
	ChecksumsURL string
}

// Checker talks to the GitHub releases API.
type Checker struct {
	Client *http.Client
	API    string // https://api.github.com/repos/<owner>/<repo>
}

// GitHub checks this project's releases.
func GitHub() Checker {
	return Checker{Client: &http.Client{Timeout: 5 * time.Minute}, API: "https://api.github.com/repos/rhwendt/wtfsync"}
}

// Latest returns the newest published release. A release without both the installer and the
// checksums file is an error: there's nothing safe to install.
func (c Checker) Latest(ctx context.Context) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.API+"/releases/latest", nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("check for updates: %s", resp.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Release{}, fmt.Errorf("check for updates: %w", err)
	}
	r := Release{Version: body.TagName}
	for _, a := range body.Assets {
		switch a.Name {
		case SetupAsset:
			r.SetupURL = a.URL
		case ChecksumsAsset:
			r.ChecksumsURL = a.URL
		}
	}
	if r.SetupURL == "" || r.ChecksumsURL == "" {
		return Release{}, fmt.Errorf("release %s has no installer", r.Version)
	}
	return r, nil
}

// Newer reports whether latest is a higher vMAJOR.MINOR.PATCH than current. Development builds
// (current "dev" or anything unparsable) never update.
func Newer(current, latest string) bool {
	c, ok1 := parse(current)
	l, ok2 := parse(latest)
	if !ok1 || !ok2 {
		return false
	}
	for i := range c {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

func parse(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// Download fetches r's installer into dir and checks it against r's checksums file. It returns
// the installer's path; on any failure nothing is left in dir.
func (c Checker) Download(ctx context.Context, r Release, dir string) (string, error) {
	sums, err := c.get(ctx, r.ChecksumsURL)
	if err != nil {
		return "", err
	}
	want, err := checksumFor(sums, SetupAsset)
	sums.Close()
	if err != nil {
		return "", err
	}
	body, err := c.get(ctx, r.SetupURL)
	if err != nil {
		return "", err
	}
	defer body.Close()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, "wtfsync-setup-"+r.Version+".exe")
	f, err := os.Create(dst + ".tmp")
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && hex.EncodeToString(h.Sum(nil)) != want {
		err = ErrChecksum
	}
	if err == nil {
		err = os.Rename(dst+".tmp", dst)
	}
	if err != nil {
		os.Remove(dst + ".tmp")
		return "", err
	}
	return dst, nil
}

func (c Checker) get(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return resp.Body, nil
}

// checksumFor reads `sha256sum` output ("<hex>  <name>" per line) and returns name's hash.
func checksumFor(r io.Reader, name string) (string, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%s has no checksum for %s", ChecksumsAsset, name)
}
