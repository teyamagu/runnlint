# テスト設計

[English](../en/test-design.md)

検査の契約は [ルールファイル](configuration.md) と [runn への追従](runn-follow.md)、実行の流れ（コマンド、ファイル収集、ルール、yamllint、出力、終了コード）に分かれる。テストもその境界で分ける。

これ以降の変更は、先に失敗するテストを追加してから実装する。ここに書いた既存の契約は、いまの実装を仕様として固定するテストで確認する。

## バイナリの土台

コマンドはフラグを解釈して `app.Run` に渡す。終了コードは、`error` が1件でもあれば `1`、設定や実行の失敗は `2`、それ以外は `0` である。`warning` と `info` だけでは失敗しない。

| ID | 契約 | テスト |
| --- | --- | --- |
| BIN-01 | `-h` は 0。未知のフラグは 2 | `cmd/runnlint/main_test.go` |
| BIN-02 | `--print-config` と `--print-yamllint-config` は組み込み定義を出して 0 | 同 |
| BIN-03 | `--list-rules` は `id`、深刻度、`builtin:` または `check` | 同 |
| BIN-04 | 検査の終了コードをコマンドがそのまま返す。設定不正は 2 | 同 |
| BIN-05 | ディレクトリ走査は runbook 以外を飛ばす。`steps:` または `runners:` がある壊れた YAML は残す | `internal/app/contract_test.go` |
| BIN-06 | `ignore` はディレクトリ走査で集めたファイルだけを外す。パターンがファイルパスでも、引数のファイルは検査する。リテラルは走査パスの一致とその下。末尾の `/` を除くのはその下だけで、`examples/fixtures/openapi.yml/` は `examples/fixtures/openapi.yml` に当たらない。ベース名に当たるのはパターン全体がベース名と一致するときで、`*.gen.yml` は当たり `dir/*.yml` は `dir` の直下だけ。`**` は走査パスだけ。`./` 付きや、`/` を含むパターンは、走査パスの文字列が違うと当たらない。`openapi.yml` と `*.gen.yml` は絶対パスにもベース名で当たる | `internal/app/contract_test.go`、`internal/config/config_test.go` |
| BIN-07 | warning だけ、info だけでは 0。error は 1。不明な出力形式と存在しないパスは 2 | 同 |
| BIN-08 | text / github / json の形と、ファイル・行・列・ルール名での整列 | `internal/report/report_test.go` |
| BIN-09 | 継承、`extends: none`、同一 id の上書き、検証、探索順、JSON | `internal/config/contract_test.go` |
| BIN-10 | yamllint の parsable 出力と、設定パスの解決 | `internal/yamllint/yamllint_test.go` |
| BIN-11 | YAML の重複キー、alias、merge、複数 document、runner 種別 | `internal/runbook/parse_test.go` |
| BIN-12 | スキーマと `cdpfn.go` からキーと CDP アクションを読む。最新リリースとの差で失敗する | `internal/runnspec` |
| BIN-13 | Go の最新安定版と、依存ライセンス | `internal/gotoolchain`、`internal/license` |
| BIN-14 | 製品バージョンは semver。`--version` は `runnlint X.Y.Z`。`CHANGELOG.md` と `CHANGELOG.en.md` の `## [X.Y.Z]` が見出しとして一致し、現行版の節がある | `internal/version`、`cmd/runnlint/main_test.go` |
| BIN-15 | `--write-baseline` は全診断を書いて 0。`--baseline` はファイル・ルール・メッセージの件数で照合し、行がずれても既存の診断を出さない。新しい診断だけで終了コードを決める。組み込み診断のメッセージは英語で記録し、表示が日本語でもその英語で照合する。宣言的ルールの文は書いたまま。形式は version 2 で、version 1 は 2 で失敗する。読めない baseline は 2 | `internal/baseline`、`internal/app/contract_test.go`、`cmd/runnlint/main_test.go` |
| BIN-16 | 探索は `.runnlint-runn.yaml`、次に `.runnlint-runn.yml` だけを選ぶ。`.runnlint.yaml` などは読まず、警告も出さない。見つけたファイルは形が違えば 2。下の階層の未知のキーは警告して読む。`--config` は未知のキーを警告して読む | `internal/config/contract_test.go`、`internal/app/contract_test.go` |

BIN-12 と BIN-13 は、プロダクトの実行経路ではなく、このリポジトリの更新方針を固定する。記法定義の差を取り込んで PR にする処理は `.github/workflows/runn-follow.yml` と `cmd/runnfollow` です。差の有無は `internal/runnspec.Changed` が決めます。

## 組み込みルールと独自ルール

デフォルト設定は、登録された組み込みルールをすべて有効にする。宣言的ルールは `uses` を持たない。`severity: off` のルールは診断を出さない。`var-ref` の `options.ignore` と `http-request` の `options.require-leading-slash` が組み込みルールの追加設定である。

| ID | 契約 | テスト |
| --- | --- | --- |
| RULE-01 | デフォルトの `uses` は登録済みの組み込みと一致し、宣言的ルールは `check` | `internal/rules/catalog_test.go` |
| RULE-02 | 正常な HTTP、DB、map steps、動的 runner、inline include、アンカー | `lint_test.go` `TestValidRunbooks` `TestAnchorMerge` |
| RULE-03 | 代表的な組み込みの検出と深刻度 | `lint_test.go` `TestRunnSpecificRules` |
| RULE-04 | 未カバーだった組み込み。`unknown-key` と `x-`、`book-shape`、`require-steps`、`defer-experimental`、`ssh-request`、`cdp-request`、`agent-request`、`include-request`、`needs-path`、`local-path`、自分自身の step 参照 | `internal/rules/builtin_test.go` |
| RULE-05 | `var-ref` の `options.ignore` と `severity: off`。`parent.vars` は include の外では警告 | 同 |
| RULE-06 | デフォルトの `require-desc` と `no-debug` | 同 |
| RULE-07 | 宣言的ルールの `exists`、`equals`、`not_equals`、`matches`、`not_matches`、`key_matches`、`key_not_matches`、`one_of`、`min_len`、`max_len`、`where`、`forbid`、メッセージ、`steps.*`、壊れた正規表現 | `internal/rules/declare_test.go` `lint_test.go` `TestCustomRule` |
| RULE-08 | `{{ }}` は引用符の中を式の終わりにしない。時間の数値とテンプレート | `internal/rules/expr_test.go` |
| RULE-09 | 各検査関数の分岐。HTTP の media type、gRPC の `message` / `messages`、DB、exec、SSH、CDP の引数、agent、include、runner 設定、book の型、step 参照の可視性、`parent.vars`、`i` と `previous` | `internal/rules/branch_test.go` |
| RULE-10 | yamllint が無いとき、`missing` の深刻度で診断する。`off` と無効化では診断しない | `internal/yamllint/yamllint_test.go` |
| RULE-11 | `examples/bad/<ルール id>.yml` は、その既定ルールの診断を出す。`examples/valid` は診断を出さない | `internal/rules/examples_test.go` |
| RULE-12 | 式は、runn と同じく値の `${...}` を展開してから解析する。未設定の環境変数でも `expr-syntax`、`var-ref`、`step-ref` を出さない | `internal/rules/falsepositive_test.go` |
| RULE-13 | `test` や `if` などの式は、括弧の外の `#` から行末をコメントとして取り除いてから解析する。文字列リテラルとクロージャの `#` は残す。`{{ }}` の中は取り除かない | 同 |
| RULE-14 | `{{ }}` は式の `( )` `[ ]` `{ }` の中の `}}` を終わりにしない。括弧が釣り合わないときは引用符の外で最初の `}}` で切る | 同 |
| RULE-15 | `bind` の値は mapping、配列、数値も受ける。中の文字列は式として検査する | 同 |
| RULE-16 | `included-unreferenced` は、走査範囲か `--include-root` の中にある `if: included`（`included == true`、`included && ...` を含む）の runbook が、どこからも include されていないときだけ出す。`||`、`!`、`if` 無し、動的な参照で照合しうるもの、直接渡しただけのファイルは出さない。ignore したファイルも参照元に数える | `internal/rules/includes_test.go`（`testdata/included`）、`internal/app/contract_test.go` |
| RULE-17 | `# runnlint disable-line` は行末ならその行、コメント行なら次の空でもコメントでもない行の診断を、全ルール共通の層で消す。複数行スカラーの診断はキーの行で消せる。引用符とブロックの中の `#` は抑制ではない。未知のルール、yamllint のルール、未知の指示、何も消さない抑制は `unused-suppression`。off のルールは除く。baseline より先に効く | `internal/rules/suppress_test.go`、`internal/app/contract_test.go` |
| RULE-18 | HTTP と gRPC の空の `headers`（`{}` と null）は受ける | `internal/rules/falsepositive_test.go` |
| RULE-19 | `/` で始まらない HTTP の path は既定で診断しない。`options.require-leading-slash: true` のときだけ warning | 同 |
| RULE-20 | `var-ref` は、include 元の `include.vars` で渡す変数を定義済みとする。include 元があれば `parent.vars` を外と言わない。include 元が見つからない `if: included` の runbook では `vars` を調べない | `internal/rules/includes_test.go`（`testdata/includevars`） |
| RULE-21 | `test` と `bind` の式の中の `{{` はテンプレートにしない。値全体が1つの `{{ }}` のときは中身を式として解析し、`expr-braces` を出す。`{{` で始まるが1つに収まらないときは `expr-braces` だけで、式の解析エラーは出さない | `internal/rules/falsepositive_test.go` |
| RULE-22 | `unused-runner` は、`runners` と step の `runner:` の runner のうち、その runbook と inline include のどの step も呼ばないものを warning で出す。予約語の名前は出さない。`require-test-step` は、トップレベルの `if` が `included` を要しない runbook に、runner の呼び出し・include・exec・`runner:` が無く `test` だけの step（`desc`・`if`・`dump`・`bind` は添えてよい）が本体にも inline include にも無いとき、`steps` の行に info で出す。`if: included` の runbook と step が無い runbook は出さない | `internal/rules/unused_test.go` |
| RULE-23 | `expr-sharp-comment` は、`test`・`if`・`until`・`bind` などの式に括弧の外の `#` コメントがあるとき、その欄の行に info で出す。文字列の中の `#`、クロージャの `#`、`//` のコメントは出さない。YAML のコメント（空白の後の `#`）は値に含まれないので出さない | `internal/rules/sharp_comment_test.go` |

RULE-03 は各ルールの主経路を1件ずつ見る。RULE-04 と RULE-07 は、そこから漏れていた契約をルール単位で見る。RULE-09 は、その中の分岐をメッセージまで固定する。
