package gotoolchain

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// stable が 1.27.2、1.28.0、1.26.9 で rc もある一覧から、最新安定版が 1.28.0 になることを確認する。
func TestLatestStablePicksHighestStable(t *testing.T) {
	data := []byte(`[
		{"version": "go1.28rc1", "stable": false},
		{"version": "go1.27.2", "stable": true},
		{"version": "go1.28.0", "stable": true},
		{"version": "go1.26.9", "stable": true}
	]`)
	got, err := LatestStable(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.28.0" {
		t.Fatalf("latest = %s", got)
	}
}

// stable が無く rc だけの一覧では、LatestStable がエラーになることを確認する。
func TestLatestStableRejectsReleaseCandidatesOnly(t *testing.T) {
	_, err := LatestStable([]byte(`[{"version": "go1.28rc1", "stable": false}]`))
	if err == nil {
		t.Fatal("安定版が無い一覧を受け付けました")
	}
}

// go1.27.2 を正規化したとき 1.27.2 になることを確認する。devel ビルドの文字列ではエラーになることを確認する。
func TestNormalize(t *testing.T) {
	got, err := Normalize("go1.27.2")
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.27.2" {
		t.Fatalf("normalize = %s", got)
	}
	if _, err := Normalize("devel go1.28-abcdef"); err == nil {
		t.Fatal("devel ビルドを受け付けました")
	}
}

// .go-version と go.mod の go 行が一致し、どちらも実行中の Go および go.dev の最新安定版と同じであることを確認する。
func TestUsesLatestStableGo(t *testing.T) {
	root := moduleRoot(t)
	fileVersion, modVersion, err := Declared(root)
	if err != nil {
		t.Fatal(err)
	}
	if fileVersion != modVersion {
		t.Fatalf(".go-version は %s、go.mod は %s です。同じバージョンにしてください", fileVersion, modVersion)
	}

	runtimeVersion, err := Normalize(runtime.Version())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	latest, err := FetchLatestStable(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if fileVersion == latest && runtimeVersion == latest {
		return
	}
	t.Fatalf("このリポジトリは Go の最新安定版を使います。最新安定版は %s、.go-version と go.mod は %s、実行中の Go は %s です。.go-version と go.mod の go 行を %s に更新し、その Go で go test ./... が通ることを確認してください", latest, fileVersion, runtimeVersion, latest)
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod が見つかりません")
		}
		dir = parent
	}
}
