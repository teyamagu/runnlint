# runnlint

[runn](https://github.com/k1LoW/runn) の runbook を検査する linter です。

インデント、重複キー、行長といった一般的な YAML の検査は [yamllint](https://yamllint.readthedocs.io/) に任せます。runn 固有の runner、step、`{{ }}`、`steps` の参照は、このツールが検査します。ルールの有効化、深刻度、追加は YAML で記述します。

## 必要環境

Go は最新安定版を使います。採用中のバージョンは `.go-version` と `go.mod` の `go` 行に書き、[Go の最新安定版](https://go.dev/dl/) と一致させます。GitHub Actions はその最新安定版で `go test ./...` を実行します。宣言が古いとき、またはその Go でテストが失敗したときに CI が失敗します。コードの変更が無い週も、新しい安定版が出ていないかを確認します。

一般的な YAML lint には yamllint を入れます。

```console
pip install yamllint
```

未導入でも runn 固有の検査は動作し、警告を 1 件出して yamllint をスキップします。yamllint は GPL-3.0 の外部コマンドで、このリポジトリの Go モジュールには含めません。

## 使い方

```console
go install ./cmd/runnlint
runnlint path/to/runbook.yml
runnlint path/to/runbooks/
```

ディレクトリを渡すと、ルートに `steps` または `runners` がある YAML だけを検査します。

```console
runnlint -f json .
runnlint -f github .
runnlint --no-yamllint .
runnlint --baseline runnlint-baseline.json .
runnlint --list-rules
runnlint --print-config > .runnlint.yaml
runnlint --version
```

終了コードは、error が 1 件でもあれば `1`、設定や実行の失敗は `2`、それ以外は `0` です。warning と info だけでは失敗しません。

### baseline

既存の違反を受け入れたまま導入するには、いまの診断を baseline として書き出し、以後はそれを渡します。baseline にある診断は出さず、新しく増えた診断だけを出します。終了コードも、残った診断だけで決まります。

```console
runnlint --write-baseline runnlint-baseline.json .
runnlint --baseline runnlint-baseline.json .
```

`--write-baseline` は error / warning / info をすべて書き出し、終了コード `0` で終わります。照合のキーは、baseline ファイルのあるディレクトリからの相対パス、ルール ID、メッセージです。行と列は使わないので、違反の前に行を足しても既存の違反のままです。同じキーの診断は件数で照合し、記録より増えた分を新しい診断として出します。直した違反が baseline に残っているときは件数を標準エラーに出すので、同じコマンドで書き直してください。

製品の版は Semantic Versioning です。ルールファイルの `version: 1` とは別です。番号の上げ方と、タグからのリリースは [docs/versioning.md](docs/versioning.md) に書いてあります。

## ルールファイル

フィールドの意味、継承、宣言的ルールの書き方は [docs/configuration.md](docs/configuration.md) にまとめています。runn の更新への追従は [docs/runn-follow.md](docs/runn-follow.md) です。

カレントディレクトリから親へ `.runnlint.yaml`、`.runnlint.yml`、`runnlint.yaml`、`runnlint.yml` の順に探します。ファイルが無ければ組み込みのデフォルトを使います。`extends` を省略するとデフォルトを継承し、同じ `id` のルールだけを上書きします。同じ `id` を繰り返すと、後ろの項目が前の項目を上書きします。

```yaml
version: 1
extends: default

yamllint:
  enabled: true
  bin: yamllint
  missing: warning
  # config: .yamllint.yaml

ignore:
  - examples/bad

rules:
  # 組み込みルールを止めるときは、同じ id に severity: off だけを書く。
  # CLI から渡す変数は未定義警告にしない。severity を省くと error になる。
  - id: var-ref
    severity: warning
    options:
      ignore:
        - token

  # YAML だけでルールを足す
  - id: no-plain-password
    severity: error
    message: "vars.password は ${VAR} を参照してください"
    check:
      select: vars.password
      matches: '^\$\{.+\}$'
```

`extends: none` にすると継承しません。`severity` は `error`、`warning`、`info`、`off` です。`ignore` はディレクトリ走査のときだけ適用され、パスを直接指定したファイルは検査します。

### 宣言的ルール

`check.select` はドット区切りです。`*` は mapping の各値、または sequence の各要素です。`steps` が list でも map でも `steps.*` で各 step を指します。

使える条件は `exists`、`equals`、`not_equals`、`matches`、`not_matches`、`one_of`、`min_len`、`max_len`、`forbid` です。`where` で絞り込んでから判定します。`message` では `{{ .path }}`、`{{ .key }}`、`{{ .value }}` を使えます。

```yaml
rules:
  - id: loop-until-with-test
    severity: warning
    message: "step {{ .key }} は loop.until と test を同時に持っています"
    check:
      select: steps.*
      where:
        - path: loop.until
          exists: true
        - path: test
          exists: true
      forbid: true
```

### 組み込みルール

| id | 内容 |
| --- | --- |
| `duplicate-key` | 同一 mapping 内のキー重複 |
| `unknown-key` | runbook に無いトップレベルキー |
| `book-shape` | `desc` や `loop`、`concurrency` などの型 |
| `require-steps` | `steps` が空でないこと |
| `label-format` | label に使えない文字 |
| `runner-reserved` | `exec` や `test` などの予約名 |
| `runner-config` | runner 種別の判定と設定キー |
| `runner-defined` | step から未定義 runner を呼んでいないこと |
| `step-shape` | main runner は 1 つ、`runner:` は単独 |
| `http-request` | path、メソッド、`body` の media type |
| `grpc-request` | `package.Service/Method`、`message` |
| `db-request` | `query` |
| `exec-request` | `command` と `shell` |
| `ssh-request` | `command` |
| `cdp-request` | `actions` とアクション名 |
| `agent-request` | `prompt` |
| `include-request` | `path` と `steps` の排他 |
| `include-path` / `needs-path` / `local-path` | 参照先ファイルの存在 |
| `expr-syntax` | `{{ }}` と `test` / `if` / `until` / `bind` の式 |
| `expr-braces` | 式フィールドを `{{ }}` で囲んでいないこと |
| `step-ref` | list は `steps[n]`、map は `steps.name`。未来の step は不可 |
| `var-ref` | `vars.name` が `vars` にあること |
| `loop-index` | `i` は `loop` の中だけ |
| `previous-step` | 先頭 step の `previous` |
| `defer-experimental` | `defer` は実験的機能 |

`require-desc`、`no-debug`、`loop-until-with-test` はデフォルトの宣言的ルールです。

## 開発

実装は、失敗するテストを先に追加してから行う。契約とテストの対応は [docs/test-design.md](docs/test-design.md) に書いてある。

```console
go test ./...
go run ./cmd/runnlint --no-yamllint examples/valid
go run ./cmd/runnlint --no-yamllint examples/bad/http-request.yml
```

`go test ./...` は、次も確認します。

- `.go-version`、`go.mod`、実行中の Go が同じバージョンであること
- そのバージョンが Go の最新安定版であること
- 依存ライブラリのライセンスが MIT、BSD、ISC、Apache-2.0 のいずれかであること

新しい安定版が出ると、`.go-version` と `go.mod` を上げるまでこのテストが失敗します。CI は先にその Go でテスト一式を実行するので、宣言を上げる前に互換性を確認できます。

`examples/valid` は通る runbook です。HTTP と DB は `examples/valid/http.yml` です。`examples/bad` は既定ルールごとに1ファイルの検出例で、ファイル名がルール id です。ディレクトリ走査では `examples/bad` を無視します。ファイルを直接指定すると検査します。

## ライセンス

[MIT License](LICENSE) です。

依存する Go ライブラリは、MIT のプロジェクトから利用できるものだけを使います。現在の直接依存は次のとおりです。

| モジュール | ライセンス |
| --- | --- |
| `github.com/expr-lang/expr` | MIT |
| `github.com/k1LoW/duration` | MIT |
| `gopkg.in/yaml.v3` | MIT および Apache-2.0 |
