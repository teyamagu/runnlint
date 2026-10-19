# テスト設計

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
| BIN-06 | `ignore` はディレクトリ走査だけ。引数のファイルは検査する | 同 |
| BIN-07 | warning だけ、info だけでは 0。error は 1。不明な出力形式と存在しないパスは 2 | 同 |
| BIN-08 | text / github / json の形と、ファイル・行・列・ルール名での整列 | `internal/report/report_test.go` |
| BIN-09 | 継承、`extends: none`、同一 id の上書き、検証、探索順、JSON | `internal/config/contract_test.go` |
| BIN-10 | yamllint の parsable 出力と、設定パスの解決 | `internal/yamllint/yamllint_test.go` |
| BIN-11 | YAML の重複キー、alias、merge、複数 document、runner 種別 | `internal/runbook/parse_test.go` |
| BIN-12 | スキーマと `cdpfn.go` からキーと CDP アクションを読む。最新リリースとの差で失敗する | `internal/runnspec` |
| BIN-13 | Go の最新安定版と、依存ライセンス | `internal/gotoolchain`、`internal/license` |
| BIN-14 | 製品バージョンは semver。`--version` は `runnlint X.Y.Z`。変更履歴にその節がある | `internal/version`、`cmd/runnlint/main_test.go` |

BIN-12 と BIN-13 は、プロダクトの実行経路ではなく、このリポジトリの更新方針を固定する。

## 組み込みルールと独自ルール

デフォルト設定は、登録された組み込みルールをすべて有効にする。宣言的ルールは `uses` を持たない。`severity: off` のルールは診断を出さない。`var-ref` の `options.ignore` だけが組み込みルールの追加設定である。

| ID | 契約 | テスト |
| --- | --- | --- |
| RULE-01 | デフォルトの `uses` は登録済みの組み込みと一致し、宣言的ルールは `check` | `internal/rules/catalog_test.go` |
| RULE-02 | 正常な HTTP、DB、map steps、動的 runner、inline include、アンカー | `lint_test.go` `TestValidRunbooks` `TestAnchorMerge` |
| RULE-03 | 代表的な組み込みの検出と深刻度 | `lint_test.go` `TestRunnSpecificRules` |
| RULE-04 | 未カバーだった組み込み。`unknown-key` と `x-`、`book-shape`、`require-steps`、`defer-experimental`、`ssh-request`、`cdp-request`、`agent-request`、`include-request`、`needs-path`、`local-path`、HTTP path の warning、自分自身の step 参照 | `internal/rules/builtin_test.go` |
| RULE-05 | `var-ref` の `options.ignore` と `severity: off`。`parent.vars` は include の外では警告 | 同 |
| RULE-06 | デフォルトの `require-desc` と `no-debug` | 同 |
| RULE-07 | 宣言的ルールの `exists`、`equals`、`not_equals`、`matches`、`not_matches`、`one_of`、`min_len`、`max_len`、`where`、`forbid`、メッセージ、`steps.*`、壊れた正規表現 | `internal/rules/declare_test.go` `lint_test.go` `TestCustomRule` |
| RULE-08 | `{{ }}` は引用符の中を式の終わりにしない。時間の数値とテンプレート | `internal/rules/expr_test.go` |
| RULE-09 | 各検査関数の分岐。HTTP の media type、gRPC の `message` / `messages`、DB、exec、SSH、CDP の引数、agent、include、runner 設定、book の型、step 参照の可視性、`parent.vars`、`i` と `previous` | `internal/rules/branch_test.go` |
| RULE-10 | yamllint が無いとき、`missing` の深刻度で診断する。`off` と無効化では診断しない | `internal/yamllint/yamllint_test.go` |
| RULE-11 | `examples/bad/<ルール id>.yml` は、その既定ルールの診断を出す。`examples/valid` は診断を出さない | `internal/rules/examples_test.go` |
| RULE-12 | 式は、runn と同じく値の `${...}` を展開してから解析する。未設定の環境変数でも `expr-syntax`、`var-ref`、`step-ref` を出さない | `internal/rules/falsepositive_test.go` |
| RULE-13 | `test` や `if` などの式は、括弧の外の `#` から行末をコメントとして取り除いてから解析する。文字列リテラルとクロージャの `#` は残す。`{{ }}` の中は取り除かない | 同 |

RULE-03 は各ルールの主経路を1件ずつ見る。RULE-04 と RULE-07 は、そこから漏れていた契約をルール単位で見る。RULE-09 は、その中の分岐をメッセージまで固定する。
