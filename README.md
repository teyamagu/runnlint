# runnlint

[English](README.en.md)

[![build](https://github.com/teyamagu/runnlint/actions/workflows/ci.yml/badge.svg)](https://github.com/teyamagu/runnlint/actions/workflows/ci.yml)
[![coverage](docs/coverage.svg)](docs/coverage.svg)
[![code to test ratio](docs/ratio.svg)](docs/ratio.svg)
[![test execution time](docs/time.svg)](docs/time.svg)

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
runnlint --strict .
runnlint --include-root . books/payments
runnlint --list-rules
runnlint --print-config > .runnlint-runn.yaml
runnlint --version
```

終了コードは、error が 1 件でもあれば `1`、設定や実行の失敗は `2`、それ以外は `0` です。warning と info だけでは失敗しません。`--strict` を付けると warning でも `1` になります。info は `--strict` でも失敗しません。`--baseline` で消した診断は数えないので、既存の warning を baseline に入れたまま、新しい warning だけで止められます。

表示は日本語が既定です。設定の `language: en`、または `--lang en` で英語にします。`--lang` は設定より優先します。`ja` と `en` 以外を渡すと終了コード `2` です。text、github、JSON の `message`、`-h` の使い方は、選んだ言語の文だけを出します。

`included-unreferenced` は、ほかのファイルからの include を見るので、判定する範囲が決まっています。ディレクトリを渡して走査したファイルと、`--include-root` の中のファイルだけを判定し、参照元はその範囲の YAML をすべて読みます。ファイルを1つだけ渡したときは判定しません。include 元が走査範囲の外にあるときは `--include-root` で足します。詳しくは [docs/ja/configuration.md](docs/ja/configuration.md#included-unreferenced-の判定範囲) です。

### インライン抑制

行ごとの診断は、YAML のコメントで抑制できます。書き方は yamllint の `# yamllint disable-line rule:...` に揃えています。

```yaml
steps:
  - req:
      /legacy:
        get:
          body: null
    # コメントだけの行に書くと、次の行（空行とコメント行は飛ばす）に効く
    # runnlint disable-line rule:var-ref -- include 元から渡される
    test: current.res.status == vars.expected
  # 行末に書くと、その行に効く
  - dump: vars.legacy # runnlint disable-line rule:var-ref -- 理由
  - dump: vars.legacy # runnlint disable-line -- ルールを省くと、その行のすべてのルール
```

- ルールは `rule:<id>` か `<id>` を、空白かカンマで区切って並べます。`--` から後ろは理由として読み飛ばします。ルールを省くとその行のすべてのルールを抑制します。範囲が広くなるので、理由を書いてください。
- 組み込みルール、YAML だけで書いたルール、`included-unreferenced`、`parse` のすべてに効きます。行の無い、ファイル全体の診断は対象外です。yamllint の診断は `# yamllint disable-line` で抑制します。
- 複数行のスカラー（`|` や `>`）の中の式の診断は、キーの行に出ます。抑制はキーの行の行末（`test: | # runnlint disable-line rule:expr-syntax`）か、その前の行に書きます。ブロックの中に書いたコメントは値の一部なので抑制になりません。なお、`test` などの式の中の `# ...` は、runn と同じくコメントとして取り除いてから検査します。
- 存在しないルールを書いた抑制、何も抑制しなかった抑制、ブロックの中に書いた抑制、`disable-line` 以外の指示は、`unused-suppression`（warning）で知らせます。`severity: off` にしたルールを名指ししたものは知らせません。

インライン抑制は baseline より先に効きます。抑制した診断は baseline に書かれず、`--baseline` で照合する前に取り除かれます。`unused-suppression` は普通の診断なので、baseline に入れることも、`severity: off` で止めることもできます。

### baseline

既存の違反を受け入れたまま導入するには、いまの診断を baseline として書き出し、以後はそれを渡します。baseline にある診断は出さず、新しく増えた診断だけを出します。終了コードも、残った診断だけで決まります。

```console
runnlint --write-baseline runnlint-baseline.json .
runnlint --baseline runnlint-baseline.json .
```

`--write-baseline` は error / warning / info をすべて書き出し、終了コード `0` で終わります。照合のキーは、baseline ファイルのあるディレクトリからの相対パス、ルール ID、メッセージです。行と列は使わないので、違反の前に行を足しても既存の違反のままです。同じキーの診断は件数で照合し、記録より増えた分を新しい診断として出します。直した違反が baseline に残っているときは件数を標準エラーに出すので、同じコマンドで書き直してください。

組み込み診断のメッセージは、表示が日本語でも英語の文で記録し、英語の文で照合します。宣言的ルールに書いた `message` は、書いた文のままです。baseline ファイルの形式は version 2 です。version 1 は読み込めません。`--write-baseline` で書き直してください。

製品の版は Semantic Versioning です。ルールファイルの `version: 1` とは別です。番号の上げ方と、タグからのリリースは [docs/ja/versioning.md](docs/ja/versioning.md) に書いてあります。

## ルールファイル

フィールドの意味、継承、宣言的ルールの書き方は [docs/ja/configuration.md](docs/ja/configuration.md) にまとめています。runn の更新への追従は [docs/ja/runn-follow.md](docs/ja/runn-follow.md) です。

カレントディレクトリから親へ `.runnlint-runn.yaml`、`.runnlint-runn.yml` の順に探します。ファイルが無ければ組み込みのデフォルトを使います。`--config` でパスを直接指定することもできます。

`extends` を省略するとデフォルトを継承し、同じ `id` のルールだけを上書きします。同じ `id` を繰り返すと、後ろの項目が前の項目を上書きします。

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

`extends: none` にすると継承しません。`severity` は `error`、`warning`、`info`、`off` です。`ignore` はディレクトリ走査で集めたファイルにだけ適用します。パターンにファイルのパスを書いても、そのファイルを引数に渡したときは検査します。

### 宣言的ルール

`check.select` はドット区切りです。`*` は mapping の各値、または sequence の各要素です。`steps` が list でも map でも `steps.*` で各 step を指します。

使える条件は `exists`、`equals`、`not_equals`、`matches`、`not_matches`、`key_matches`、`key_not_matches`、`one_of`、`min_len`、`max_len`、`forbid` です。`key_matches` はキー名を見ます。map の step 名の規約に使えます。`where` で絞り込んでから判定します。`message` では `{{ .path }}`、`{{ .key }}`、`{{ .value }}`、`{{ .rule }}` を使えます。

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

診断に出た id が、どの失敗を指すかの一覧です。初期の深刻度、`uses`、オプション、式の検査範囲は [docs/ja/configuration.md](docs/ja/configuration.md#組み込みルール) に書いてあります。

| id | 知らせること |
| --- | --- |
| `duplicate-key` | 同じ mapping に、同じキーが2回書いてある |
| `unknown-key` | トップレベルに、runbook のフィールドでも `x-` 始まりでも `<<` でもないキーがある |
| `book-shape` | runbook 直下のフィールドの値が、runn の形になっていない |
| `require-steps` | 実行する step が無い |
| `label-format` | label が空であるか、runn が受け付けない文字を含んでいる |
| `runner-reserved` | runner の名前に、`test` や `exec` などの予約語を使っている |
| `runner-config` | runner の種別が分からない、またはその種別の設定が runn の形になっていない |
| `runner-defined` | step が、定義されていない runner を呼んでいる |
| `step-shape` | 1つの step で、実行する処理が1つに決まっていない |
| `http-request` | HTTP の呼び出しが runn の形になっていない |
| `grpc-request` | gRPC の呼び出しが runn の形になっていない |
| `db-request` | DB の step に、実行する SQL が無い |
| `exec-request` | exec に、実行するコマンドが無い、または付随する項目の形が違う |
| `ssh-request` | SSH の step に `command` 以外が書いてある、または `command` が無い |
| `cdp-request` | ブラウザ操作の `actions` が、runn の操作として成り立っていない |
| `agent-request` | agent への依頼文が無い、または形が違う |
| `include-request` | include が、取り込む runbook を1つに決めていない |
| `include-path` | include の参照先ファイルが無い |
| `needs-path` | `needs` の参照先ファイルが無い |
| `local-path` | runner が参照する証明書や proto などのファイルが無い |
| `expr-syntax` | `{{ }}` や `test` などの式が、式として読めない |
| `expr-braces` | 式として読む欄を `{{ }}` で包んでいる |
| `expr-sharp-comment` | `test` や `if` などの式に `#` のコメントがある。runn は取り除いて動くが、非推奨の警告を出す |
| `step-ref` | 配列の step を名前で参照している、名前付きの step を番号で参照している、またはまだ実行されていない step を参照している |
| `var-ref` | `vars` に無い変数を参照している |
| `loop-index` | `loop` の外で添字 `i` を使っている |
| `previous-step` | 直前の step が無い場所で `previous` を使っている |
| `defer-experimental` | `defer` は実験的機能だという知らせ |
| `unused-runner` | `runners` や `runner:` で定義した runner を、どの step も呼んでいない |
| `unused-bind` | `bind` で付けた名前を、このファイルの式も、この runbook を include・needs するファイルの式も読んでいない |
| `require-test-step` | 単独で実行できる runbook に、`test` だけの step（このシナリオで確かめたいことを名前にした検証 step）が無い |
| `included-unreferenced` | include されたときだけ動く runbook が、どこからも include されていない |
| `unused-suppression` | 抑制のコメントが、診断を抑制できていない |

デフォルトには宣言的ルールも入っています。`require-desc` は `desc` が無いこと、`no-debug` は `debug` が有効なこと、`loop-until-with-test` は同じ step に `loop.until` と `test` があることを知らせます。`test` は繰り返す前に評価されるので、リトライが始まる前に失敗します。

## 開発

実装は、失敗するテストを先に追加してから行う。契約とテストの対応は [docs/ja/test-design.md](docs/ja/test-design.md) に書いてある。

```console
go test ./...
go run ./cmd/runnlint --no-yamllint examples/valid
go run ./cmd/runnlint --no-yamllint examples/bad/http-request.yml
go run ./cmd/runnlint --no-yamllint --include-root examples examples/bad/included-unreferenced.yml
```

`go test ./...` は、次も確認します。

- `.go-version`、`go.mod`、実行中の Go が同じバージョンであること
- そのバージョンが Go の最新安定版であること
- 依存ライブラリのライセンスが MIT、BSD、ISC、Apache-2.0 のいずれかであること

新しい安定版が出ると、`.go-version` と `go.mod` を上げるまでこのテストが失敗します。CI は先にその Go でテスト一式を実行するので、宣言を上げる前に互換性を確認できます。

`examples/valid` は通る runbook です。HTTP と DB は `examples/valid/http.yml` です。`examples/bad` は既定ルールごとに1ファイルの検出例で、ファイル名がルール id です。ディレクトリ走査では `examples/bad` を無視します。引数にファイルを書くと検査します。`included-unreferenced` だけは、ファイルを引数に書いただけでは判定しません。上の `--include-root examples` を付けたコマンドで出します。

## ライセンス

[MIT License](LICENSE) です。

依存する Go ライブラリは、MIT のプロジェクトから利用できるものだけを使います。現在の直接依存は次のとおりです。

| モジュール | ライセンス |
| --- | --- |
| `github.com/expr-lang/expr` | MIT |
| `github.com/k1LoW/duration` | MIT |
| `gopkg.in/yaml.v3` | MIT および Apache-2.0 |
