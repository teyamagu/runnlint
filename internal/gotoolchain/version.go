// Package gotoolchain checks that this repository uses the latest stable Go.
package gotoolchain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/teyamagu/runnlint/internal/message"
)

const releaseURL = "https://go.dev/dl/?mode=json"

// Release is one entry in the go.dev release list.
type Release struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

// LatestStable returns the newest stable release from a release list. The result has the form 1.27.2.
func LatestStable(data []byte) (string, error) {
	var releases []Release
	if err := json.Unmarshal(data, &releases); err != nil {
		return "", message.Wrap(message.GoListParse, err)
	}
	best := ""
	for _, release := range releases {
		if !release.Stable {
			continue
		}
		version, err := Normalize(release.Version)
		if err != nil {
			continue
		}
		if best == "" || Compare(version, best) > 0 {
			best = version
		}
	}
	if best == "" {
		return "", message.New(message.GoNoStable)
	}
	return best, nil
}

// FetchLatestStable fetches the latest stable release from go.dev.
func FetchLatestStable(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "runnlint-gotoolchain")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", message.Wrap(message.GoListFetch, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", message.New(message.GoListStatus, resp.StatusCode)
	}
	return LatestStable(body)
}

// Declared returns the .go-version value and the go line of go.mod.
func Declared(root string) (fileVersion, modVersion string, err error) {
	raw, err := os.ReadFile(filepath.Join(root, ".go-version"))
	if err != nil {
		return "", "", message.Wrap(message.GoVersionRead, err)
	}
	fileVersion, err = Normalize(string(raw))
	if err != nil {
		return "", "", message.Prefix(".go-version", err)
	}
	modRaw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", "", message.Wrap(message.GoModRead, err)
	}
	modVersion, err = GoModVersion(string(modRaw))
	if err != nil {
		return "", "", err
	}
	return fileVersion, modVersion, nil
}

// GoModVersion extracts the version from the go line of go.mod.
func GoModVersion(src string) (string, error) {
	for _, line := range strings.Split(src, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && fields[0] == "go" {
			return Normalize(fields[1])
		}
	}
	return "", message.New(message.GoModLine)
}

// Normalize canonicalizes go1.27.2 and 1.27.2 to 1.27.2.
func Normalize(version string) (string, error) {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "go")
	if i := strings.IndexAny(version, " \t+"); i >= 0 {
		version = version[:i]
	}
	if strings.Contains(version, "devel") || version == "" {
		return "", message.New(message.VerNotStable, version)
	}
	parts := strings.Split(version, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return "", message.New(message.VerNotTriple, version)
	}
	nums := make([]int, len(parts))
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return "", message.New(message.VerNotNumber, version)
		}
		nums[i] = n
	}
	patch := 0
	if len(nums) == 3 {
		patch = nums[2]
	}
	return fmt.Sprintf("%d.%d.%d", nums[0], nums[1], patch), nil
}

// Compare returns a positive value if a is newer, 0 if a and b are equal, and a negative value if a is older.
func Compare(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		av, bv := 0, 0
		if i < len(as) {
			av, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bv, _ = strconv.Atoi(bs[i])
		}
		if av != bv {
			return av - bv
		}
	}
	return 0
}
