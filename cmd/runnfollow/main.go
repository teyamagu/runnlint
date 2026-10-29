// Command runnfollow reimports notation definitions from the latest runn.
// It writes nothing when there is no diff. It does not change rules or examples.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/teyamagu/runnlint/internal/message"
	"github.com/teyamagu/runnlint/internal/runnspec"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "internal", "runnspec")
	currentSchema, err := os.ReadFile(filepath.Join(dir, "runbook.schema.yaml"))
	if err != nil {
		return err
	}
	currentCDP, err := os.ReadFile(filepath.Join(dir, "cdpfn.src"))
	if err != nil {
		return err
	}
	tag, err := fetchLatestTag()
	if err != nil {
		return err
	}
	schema, err := fetch(rawURL(tag, "runbook.schema.yaml"))
	if err != nil {
		return err
	}
	cdp, err := fetch(rawURL(tag, "cdpfn.go"))
	if err != nil {
		return err
	}
	changed := runnspec.Changed(currentSchema, currentCDP, schema, cdp)
	if len(changed) == 0 {
		fmt.Printf("記法の差はありません（最新は %s）\n", tag)
		return nil
	}
	if err := runnspec.WriteNotation(dir, tag, schema, cdp); err != nil {
		return err
	}
	fmt.Printf("runn %s の %s を取り込みました\n", tag, strings.Join(changed, ", "))
	return nil
}

func rawURL(tag, name string) string {
	return "https://raw.githubusercontent.com/k1LoW/runn/" + tag + "/" + name
}

func fetchLatestTag() (string, error) {
	body, err := fetch("https://api.github.com/repos/k1LoW/runn/releases/latest")
	if err != nil {
		return "", err
	}
	var meta struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		return "", message.Wrap(message.RunnRelease, err)
	}
	if strings.TrimSpace(meta.TagName) == "" {
		return "", message.New(message.RunnTagEmpty)
	}
	return meta.TagName, nil
}

func fetch(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "runnlint-runnfollow")
	req.Header.Set("Accept", "application/vnd.github+json")
	if strings.Contains(url, "://api.github.com/") {
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		} else if token := os.Getenv("GH_TOKEN"); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, message.Wrap(message.HTTPFetch, err, url)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, message.New(message.HTTPStatus, url, resp.StatusCode)
	}
	if len(bytesTrim(body)) == 0 {
		return nil, message.New(message.HTTPEmpty, url)
	}
	return body, nil
}

func bytesTrim(b []byte) string {
	return strings.TrimSpace(string(b))
}

func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", message.New(message.GoModMissing)
		}
		dir = parent
	}
}
