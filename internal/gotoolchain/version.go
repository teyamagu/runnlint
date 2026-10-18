// Package gotoolchain は、このリポジトリが Go の最新安定版を使っているかを確認する。
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
)

const releaseURL = "https://go.dev/dl/?mode=json"

// Release は go.dev のリリース一覧の1件。
type Release struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

// LatestStable はリリース一覧から最も新しい安定版を返す。戻り値は 1.27.2 の形。
func LatestStable(data []byte) (string, error) {
	var releases []Release
	if err := json.Unmarshal(data, &releases); err != nil {
		return "", fmt.Errorf("Go のリリース一覧を解釈できません: %w", err)
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
		return "", fmt.Errorf("安定版がリリース一覧にありません")
	}
	return best, nil
}

// FetchLatestStable は go.dev から最新安定版を取得する。
func FetchLatestStable(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "runnlint-gotoolchain")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Go のリリース一覧を取得できません: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Go のリリース一覧が HTTP %d を返しました", resp.StatusCode)
	}
	return LatestStable(body)
}

// Declared は .go-version と go.mod の go 行を返す。
func Declared(root string) (fileVersion, modVersion string, err error) {
	raw, err := os.ReadFile(filepath.Join(root, ".go-version"))
	if err != nil {
		return "", "", fmt.Errorf(".go-version を読めません: %w", err)
	}
	fileVersion, err = Normalize(string(raw))
	if err != nil {
		return "", "", fmt.Errorf(".go-version: %w", err)
	}
	modRaw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", "", fmt.Errorf("go.mod を読めません: %w", err)
	}
	modVersion, err = GoModVersion(string(modRaw))
	if err != nil {
		return "", "", err
	}
	return fileVersion, modVersion, nil
}

// GoModVersion は go.mod の go 行からバージョンを取り出す。
func GoModVersion(src string) (string, error) {
	for _, line := range strings.Split(src, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && fields[0] == "go" {
			return Normalize(fields[1])
		}
	}
	return "", fmt.Errorf("go.mod に go 行がありません")
}

// Normalize は go1.27.2 や 1.27.2 を 1.27.2 に揃える。
func Normalize(version string) (string, error) {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "go")
	if i := strings.IndexAny(version, " \t+"); i >= 0 {
		version = version[:i]
	}
	if strings.Contains(version, "devel") || version == "" {
		return "", fmt.Errorf("バージョン %q は安定版の番号ではありません", version)
	}
	parts := strings.Split(version, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return "", fmt.Errorf("バージョン %q は major.minor.patch ではありません", version)
	}
	nums := make([]int, len(parts))
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return "", fmt.Errorf("バージョン %q を数値として読めません", version)
		}
		nums[i] = n
	}
	patch := 0
	if len(nums) == 3 {
		patch = nums[2]
	}
	return fmt.Sprintf("%d.%d.%d", nums[0], nums[1], patch), nil
}

// Compare は a が新しければ正、同じなら 0、古ければ負を返す。
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
