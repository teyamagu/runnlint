# バージョンとリリース

[English](../en/versioning.md)

runnlint の製品バージョンは [Semantic Versioning 2.0.0](https://semver.org/lang/ja/) です。値は `internal/version/version.go` の `Version` に置き、Git のタグは先頭に `v` を付けます。`0.1.0` のタグは `v0.1.0` です。

ルールファイルの `version: 1` は、設定の形の番号です。製品バージョンとは別です。

baseline ファイルの `version` も別です。いまの形式は `2` です。組み込み診断は英語の文で記録します。`1` のファイルは読み込めないので、`--write-baseline` で書き直します。`1.0.0` より前は、診断の文言を変えても製品バージョンは上げません。変えたことは変更履歴に書き、既存の baseline は書き直します。

## 番号の上げ方

`0.y.z` のあいだは、公開してからの互換をまだ約束しません。互換を壊す変更も MINOR に置けます。壊す内容は変更履歴に書きます。

`1.0.0` 以降は、次のように分けます。

| 上げる桁 | 変更 |
| --- | --- |
| MAJOR | フラグ、終了コードの意味、設定ファイルの読み方、既存の出力フィールドを変える。既定で error になるルールを増やす |
| MINOR | 既定が warning または info のルールを足す。既存の設定のまま動く項目を足す |
| PATCH | 誤検知の修正、診断の文言、依存の更新 |

既定で error になるルールを `1.0.0` 以降に足すと、それまで通っていた runbook の終了コードが `0` から `1` に変わります。これは MAJOR です。

## リリース手順

1. `CHANGELOG.md` の `[Unreleased]` にある項目を、新しい `## [X.Y.Z]` へ移す。`CHANGELOG.en.md` も同じ見出しにし、対応する英語の項目を書く。GitHub Release の本文は `CHANGELOG.md` から取る。
2. `internal/version/version.go` の `Version` を `X.Y.Z` にする。プレリリースは `1.2.3-rc.1` のように書く。
3. `go test ./...` が通ることを確認する。このテストは、`Version` が semver であることと、`CHANGELOG.md` と `CHANGELOG.en.md` に同じ `## [X.Y.Z]` があることを見る。
4. そのコミットに `vX.Y.Z` のタグを付けて push する。
5. GitHub Actions の Release が、タグと `runnlint --version` の一致を確認してから、各 OS のバイナリと `SHA256SUMS` を GitHub Release に載せる。タグに `-` が含まれるときはプレリリースになる。

バイナリ名は `runnlint_X.Y.Z_GOOS_GOARCH` です。Windows だけ末尾が `.exe` です。
