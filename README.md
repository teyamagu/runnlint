# runnlint

[English](README.en.md)

[![build](https://github.com/teyamagu/runnlint/actions/workflows/ci.yml/badge.svg)](https://github.com/teyamagu/runnlint/actions/workflows/ci.yml)
[![coverage](docs/coverage.svg)](docs/coverage.svg)
[![code to test ratio](docs/ratio.svg)](docs/ratio.svg)
[![test execution time](docs/time.svg)](docs/time.svg)

[runn](https://github.com/k1LoW/runn) の runbook を検査する linter である。

言葉は yamllint と ESLint に揃えている。**検査**（check）は runbook を見ること、**問題**（problem）は検査が見つけた1件、**報告**（report）はその問題を結果に載せることである。

インデント、重複キー、行長といった一般的な YAML の検査は [yamllint](https://yamllint.readthedocs.io/) に任せる。runn 固有の runner、step、`{{ }}`、`steps` の参照は、このツールが検査する。ルールの有効化、深刻度、追加は YAML で記述する。

## 必要環境

Go は最新安定版を使う。採用中のバージョンは `.go-version` と `go.mod` の `go` 行に書き、[Go の最新安定版](https://go.dev/dl/) と一致させる。GitHub Actions はその最新安定版で `go test ./...` を実行する。宣言が古いとき、またはその Go でテストが失敗したときに CI が失敗する。コードの変更が無い週も、新しい安定版が出ていないかを確認する。

一般的な YAML lint には yamllint を入れる。

```console
pip install yamllint
```

未導入でも runn 固有の検査は動作し、warning を 1 件報告して yamllint をスキップする。yamllint は GPL-3.0 の外部コマンドで、このリポジトリの Go モジュールには含めない。

## 使い方

```console
go install ./cmd/runnlint
runnlint path/to/runbook.yml
runnlint path/to/runbooks/
```

ディレクトリを渡すと、ルートに `steps` または `runners` がある YAML だけを検査する。

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

終了コードは、error が 1 件でもあれば `1`、設定や実行の失敗は `2`、それ以外は `0` である。warning と info だけでは失敗しない。`--strict` を付けると warning でも `1` になる。info は `--strict` でも失敗しない。`--baseline` で外した問題は数えないので、既存の warning を baseline に入れたまま、新しい warning だけで止められる。

表示は日本語が既定である。設定の `language: en`、または `--lang en` で英語にする。`--lang` は設定より優先する。`ja` と `en` 以外を渡すと終了コード `2` である。text、github、JSON の `message`、`-h` の使い方は、選んだ言語の文だけを出す。

`included-unreferenced` は、ほかのファイルからの include を見るので、検査する範囲が決まっている。ディレクトリを渡して走査したファイルと、`--include-root` の中のファイルだけを検査し、参照元はその範囲の YAML をすべて読む。ファイルを1つだけ渡したときは検査しない。include 元が走査範囲の外にあるときは `--include-root` で足す。詳しくは [docs/ja/configuration.md](docs/ja/configuration.md#included-unreferenced-の検査範囲) である。

### 行の問題を報告しない

yamllint と同じく、特定の行の問題を報告しないように YAML のコメントを書ける。書き方は `# yamllint disable-line rule:...` に揃えている。

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

- ルールは `rule:<id>` か `<id>` を、空白かカンマで区切って並べる。`--` から後ろは理由として読み飛ばす。ルールを省くと、その行のすべてのルールの問題を報告しない。範囲が広くなるので、理由を書く。
- 組み込みルール、YAML だけで書いたルール、`included-unreferenced`、`parse` のすべてに効く。行の無い、ファイル全体の問題は対象外である。yamllint の問題は `# yamllint disable-line` で報告しないようにする。
- 複数行のスカラー（`|` や `>`）の中の式の問題は、キーの行に報告される。報告しない印はキーの行の行末（`test: | # runnlint disable-line rule:expr-syntax`）か、その前の行に書く。ブロックの中に書いたコメントは値の一部なので、問題を報告しない印にならない。なお、`test` などの式の中の `# ...` は、runn と同じくコメントとして取り除いてから検査する。
- `unused-suppression`（warning）は、`# runnlint disable-line`（この行の問題は報告しない、という印）が、どの問題の報告も止めなかったときに報告する。ESLint が unused disable directive を報告するのと同じである。止められないのは、存在しないルール名（`rule:no-such-rule`）、yamllint の問題を runnlint の印で止めようとしたとき（`# yamllint disable-line rule:line-length` と書く）、`# runnlint disable` のように `disable-line` 以外を書いたとき、その行にはもともと問題が無いとき、`|` や `>` のブロックの中に書いたとき、である。`severity: off` のルールを名指しした印は対象外である。そのルールは問題を報告しないためである。

このコメントは baseline より先に効く。報告しないようにした問題は baseline に書かれず、`--baseline` で照合する前に取り除かれる。`unused-suppression` は普通の問題なので、baseline に入れることも、`severity: off` で止めることもできる。

### baseline

既存の問題を受け入れたまま導入するには、いまの問題を baseline として書き出し、以後はそれを渡す。baseline にある問題は報告せず、新しく増えた問題だけを報告する。終了コードも、残った問題だけで決まる。

```console
runnlint --write-baseline runnlint-baseline.json .
runnlint --baseline runnlint-baseline.json .
```

`--write-baseline` は error / warning / info をすべて書き出し、終了コード `0` で終わる。照合のキーは、baseline ファイルのあるディレクトリからの相対パス、ルール ID、メッセージである。行と列は使わないので、問題の前に行を足しても記録した問題のままである。同じキーの問題は件数で照合し、記録より増えた分を新しい問題として報告する。直した問題が baseline に残っているときは件数を標準エラーに出すので、同じコマンドで書き直す。

組み込みルールの問題のメッセージは、表示が日本語でも英語の文で記録し、英語の文で照合する。宣言的ルールに書いた `message` は、書いた文のままである。baseline ファイルの形式は version 2 である。version 1 は読み込めない。`--write-baseline` で書き直す。

製品の版は Semantic Versioning である。ルールファイルの `version: 1` とは別である。番号の上げ方と、タグからのリリースは [docs/ja/versioning.md](docs/ja/versioning.md) に書いてある。

## ルールファイル

フィールドの意味、継承、宣言的ルールの書き方は [docs/ja/configuration.md](docs/ja/configuration.md) にまとめている。runn の更新への追従は [docs/ja/runn-follow.md](docs/ja/runn-follow.md) である。

カレントディレクトリから親へ `.runnlint-runn.yaml`、`.runnlint-runn.yml` の順に探す。ファイルが無ければ組み込みのデフォルトを使う。`--config` でパスを直接指定することもできる。

`extends` を省略するとデフォルトを継承し、同じ `id` のルールだけを上書きする。同じ `id` を繰り返すと、後ろの項目が前の項目を上書きする。

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
  # CLI から渡す変数は、未定義の問題として報告しない。severity を省くと error になる。
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

`extends: none` にすると継承しない。`severity` は `error`、`warning`、`info`、`off` である。`ignore` はディレクトリ走査で集めたファイルにだけ適用する。パターンにファイルのパスを書いても、そのファイルを引数に渡したときは検査する。

### 宣言的ルール

`check.select` はドット区切りである。`*` は mapping の各値、または sequence の各要素である。`**` は深さを問わない任意の段で、`steps.*.**.query` はどの runner の下にある `query` でも選ぶ。`steps` が list でも map でも `steps.*` で各 step を指す。`.` を含むキーは `req."/v1/{id}.json"` のように二重引用符で囲む。

使える条件は `exists`、`kind`、`equals`、`not_equals`、`matches`、`not_matches`、`key_matches`、`key_not_matches`、`one_of`、`min_len`、`max_len`、`forbid` である。`kind` は値の型を固定し、`key_matches` はキー名を見る。map の step 名の規約に使える。`where` で絞り込んでから問題を報告する。`where` の `path` を `$.if` のように `$.` から始めると、runbook のルートから見る。`files` を書くと、そのパターンに当たるファイルにだけルールを当てる。`message` では `{{ .path }}`、`{{ .key }}`、`{{ .value }}`、`{{ .rule }}` を使える。

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

各 id が報告する問題である。初期の深刻度、`uses`、オプション、式の検査範囲は [docs/ja/configuration.md](docs/ja/configuration.md#組み込みルール) に書いてある。

| id | 問題 |
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
| `defer-experimental` | `defer` は実験的機能である |
| `unused-runner` | `runners` や `runner:` で定義した runner を、どの step も呼んでいない |
| `unused-bind` | `bind` で付けた名前を、このファイルの式も、この runbook を include・needs するファイルの式も読んでいない |
| `require-test-step` | 単独で実行できる runbook に、`test` だけの step（このシナリオで確かめたいことを名前にした検証 step）が無い。デフォルトでは検査せず、設定で `severity` を付けると検査する |
| `included-unreferenced` | include されたときだけ動く runbook が、どこからも include されていない |
| `unused-suppression` | 「この行の問題は報告しない」という印 `# runnlint disable-line` が、どの問題の報告も止めなかった。ルール名が違う、その行に問題が無い、`|` や `>` の中に書いた、`disable-line` 以外を書いた |

デフォルトには宣言的ルールも入っている。`require-desc` は `desc` が無いこと、`no-debug` は `debug` が有効なこと、`loop-until-with-test` は同じ step に `loop.until` と `test` があることの問題を報告する。`test` は繰り返す前に評価されるので、リトライが始まる前に失敗する。

## 開発

実装は、失敗するテストを先に追加してから行う。契約とテストの対応は [docs/ja/test-design.md](docs/ja/test-design.md) に書いてある。

```console
go test ./...
go run ./cmd/runnlint --no-yamllint examples/valid
go run ./cmd/runnlint --no-yamllint examples/bad/http-request.yml
go run ./cmd/runnlint --no-yamllint --include-root examples examples/bad/included-unreferenced.yml
```

`go test ./...` は、次も確認する。

- `.go-version`、`go.mod`、実行中の Go が同じバージョンであること
- そのバージョンが Go の最新安定版であること
- 依存ライブラリのライセンスが MIT、BSD、ISC、Apache-2.0 のいずれかであること

新しい安定版が出ると、`.go-version` と `go.mod` を上げるまでこのテストが失敗する。CI は先にその Go でテスト一式を実行するので、宣言を上げる前に互換性を確認できる。

`examples/valid` は通る runbook である。HTTP と DB は `examples/valid/http.yml` である。`examples/bad` は既定ルールごとに1ファイルの報告例で、ファイル名がルール id である。ディレクトリ走査では `examples/bad` を無視する。引数にファイルを書くと検査する。`included-unreferenced` だけは、ファイルを引数に書いただけでは検査しない。上の `--include-root examples` を付けたコマンドで報告する。`require-test-step` はデフォルトでは検査しないので、`severity: info` を付けた設定で `examples/bad/require-test-step.yml` を渡すと問題を報告する。

## ライセンス

[MIT License](LICENSE) である。

依存する Go ライブラリは、MIT のプロジェクトから利用できるものだけを使う。現在の直接依存は次のとおりである。

| モジュール | ライセンス |
| --- | --- |
| `github.com/expr-lang/expr` | MIT |
| `github.com/k1LoW/duration` | MIT |
| `gopkg.in/yaml.v3` | MIT および Apache-2.0 |
