# 変更履歴

[English](CHANGELOG.en.md)

バージョンは [Semantic Versioning 2.0.0](https://semver.org/lang/ja/) です。上げ方は [docs/ja/versioning.md](docs/ja/versioning.md) に書いてあります。

## [Unreleased]

### 追加

- `--strict`。warning でも終了コードを 1 にする。info は対象外。`--baseline` で消した診断は数えない。
- `unused-runner`（warning）。`runners` や step の `runner:` で定義した runner を、どの step も呼んでいないことを知らせる。inline include の中の step が呼ぶものは使っているとみなす。
- `require-test-step`（info）。単独で実行できる runbook に `test` だけの step が無いことを知らせる。リクエストに付けた `test` は1つの応答の確認で、`test` だけの step はシナリオで確かめたいことを名前にする。`if: included` の runbook は対象外。
- `expr-sharp-comment`（info）。`test` や `if` などの式に括弧の外の `#` コメントがあることを知らせる。runn は取り除いて動くが、非推奨の警告を出して `//` を求める。
- `unused-bind`（warning）。`bind` で付けた名前を、このファイルの式も、この runbook を include・needs するファイルの式も読んでいないことを知らせる。`if: included` で固定パスの参照元が無い runbook は判定しない。

### 変更

- 日本語の文書を `docs/ja/` に置く。
- mapping のキー（HTTP のパス、body の項目名）の中の `{{ }}` も式として読む。runn はキーも展開するため。`expr-syntax`、`var-ref`、`step-ref`、`unused-bind` がキーの中の参照を見る。
- `examples` の runbook に、確かめたいことを名前にした `test` だけの step を足した。

### 修正

- `ignore` の `**/` の後ろの `*` と `?` がディレクトリ区切りをまたいで当たっていた（`a/**/?.yml` が `a/x/cc.yml` に当たる）。`**/` を 0 個以上のディレクトリ全体として扱う。
- 宣言的ルールの `equals: null`、`not_equals: null`、`where` の `equals: null` が無視されていた。YAML の null は decode されないため、キーが書かれたことを設定の読み込みで記録するようにした。

## [0.2.0]

### 追加

- 表示言語。設定の `language` は `ja` または `en` で、省略すると日本語です。`--lang` は設定より優先します。text、github、JSON の `message` と使い方は、選んだ言語だけを出します。
- README、ルールファイル、バージョン、テスト設計、runn への追従、変更履歴の英語版。

### 変更

- 組み込み診断の baseline 照合は、表示言語にかかわらず英語の文です。宣言的ルールの `message` は書いた文のままです。baseline ファイルの形式を version 2 にしました。version 1 は読み込めないので、`--write-baseline` で書き直してください。
- コード内のコメントを英語にしました。

## [0.1.4]

### 追加

- README に、ビルド結果、カバレッジ、コードとテストの比率、テスト時間のバッジを出す。
- runn のスキーマと CDP アクション定義が変わったとき、取り込みを更新してルールと examples を確認し、修正 PR を作る。

### 変更

- 設定ファイルの探索は `.runnlint-runn.yaml` と `.runnlint-runn.yml` だけにする。`.runnlint.yaml`、`.runnlint.yml`、`runnlint.yaml`、`runnlint.yml` は読まない。
- 組み込みルールの説明を、診断が知らせる失敗に合わせる。
- テスト時間のバッジは、CI の `テスト` ステップの実行時間から作る。
- 設定の `version`、`{{ .rule }}`、`db-request` の `trace`、式の `${...}` と行コメント、`bind` の値の形を文書に書く。`included-unreferenced` の検出例を再現するコマンドを書く。

## [0.1.3]

### 追加

- 宣言的ルールの `key_matches` と `key_not_matches`。選んだ値のキー名を正規表現で検査する。step 名の規約は、必要なプロジェクトがこれで書く。既定のルールには入れない。

### 変更

- `test` と `bind` の式の中にある `{{` はテンプレートとして読まない。文字列の中の `{{` で `expr-syntax` や `expr-braces` を出さない。式全体を `{{ }}` で包んだときは、これまでどおり `expr-braces` を出す。

## [0.1.2]

### 追加

- `--write-baseline FILE` と `--baseline FILE`。既存の診断を記録し、新しく増えた診断だけを出す。照合はファイル・ルール ID・メッセージの件数で、行のずれに影響されない。
- インライン抑制 `# runnlint disable-line [rule:<id>...] [-- 理由]`。行末ならその行、コメント行なら次の行の診断を消す。使われない抑制は `unused-suppression`（warning）で知らせる。
- `included-unreferenced`（warning）。`if: included` の runbook が、走査した範囲のどこからも include されていないことを指摘する。参照元を探す範囲を足す `--include-root DIR`。
- 設定ファイルの固有の名前 `.runnlint-runn.yaml` / `.runnlint-runn.yml`。従来の名前より先に探す。
- `http-request` の `options.require-leading-slash`。

### 変更

- 式は、runn と同じく `${...}` を展開し、`#` の行コメントを取り除いてから解析する。未設定の環境変数やコメントで `expr-syntax` を出さない。
- `{{ }}` の中の入れ子の括弧と波括弧で、式の終わりを読み違えない。
- `bind` の値に mapping や配列、数値を書ける。中の文字列は式として検査する。
- 空の `headers`（`{}` と null）を受ける。
- `/` で始まらない HTTP の path は、既定では warning にしない。
- `var-ref` は、include 元の `include.vars` で渡す変数を定義済みとする。include 元が見つからない `if: included` の runbook では `vars` を調べない。
- 探索で見つけた `.runnlint.yaml` などが runnlint のルールファイルの形でないときは、警告を出して読み飛ばす。終了コード 2 で止まらない。

## [0.1.1]

- 既定ルールごとの検出例を `examples/bad` に、通る runbook を `examples/valid` に置く。検出例のコメントは、壊れている箇所を日本語で書く。

## [0.1.0]

最初のリリースです。runn の runbook について、runn 固有の記法を検査します。
