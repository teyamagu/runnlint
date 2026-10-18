# runn の更新への追従

runnlint が合わせる runn は [v1.11.1](https://github.com/k1LoW/runn/releases/tag/v1.11.1) です。タグは `internal/runnspec/VERSION` にあります。

[CHANGELOG](https://github.com/k1LoW/runn/blob/main/CHANGELOG.md) を v0.101（2024-03）から v1.11.1（2026-09）まで見ると、リリースの大半は依存関係の更新です。runbook の記法が変わるリリースは少なく、変わり方は次の3種類に分かれます。追従の仕組みはこの分かれ方に合わせています。

## 記法の変わり方

| 種類 | 例 | スキーマに出るか |
| --- | --- | --- |
| キーや runner の追加 | v1.2.0 の `exec.env`、v0.135.0 の CDP `flags`、v0.138.0 の CDP `timeout`、v1.7.0 の inline include、v1.8.0 の agent runner | 出る。スキーマ自体は v1.4.0 で追加され、v1.6.2 でトップレベルキーが閉じられた |
| パーサにしか無い形 | v0.132.0 で CDP の `latestTab` が `tabTo` に変わった。v0.134.0 で HTTP の content-type 制限が外れた | 出ない。ステップの HTTP / gRPC / DB の中身は [runbook.schema.yaml](https://github.com/k1LoW/runn/blob/main/runbook.schema.yaml) の対象外 |
| 式の関数追加 | v0.130.0 の `file()`、v0.137.0 の JWT、v0.140.1 の `faker.Regex()`、v1.3.0 の `hash()` | 出ない。YAML の形は変わらない |

破壊的な変更の多くは、環境変数のスナップショット（v1.11.0）や include 間の cookie（v1.9.0）のように実行時の挙動です。YAML の形とは別なので、linter の追従対象にしていません。

## 3つの層

1. **スキーマ**。`internal/runnspec/runbook.schema.yaml` を読み、トップレベルキー、`x-`、label の正規表現、runner や `exec` / `dump` / `loop` / `trace` のキー、`required` を検査に使います。`exec.env` のような追加は、このファイルを差し替えると許可されます。
2. **CDP アクション**。スキーマにアクション名が無いので、`internal/runnspec/cdpfn.src`（runn の `cdpfn.go`）から名前、別名、入力引数を読みます。`latestTab` から `tabTo` への変更はここで検知します。
3. **パーサ上書き**。スキーマと実装が違うところ、スキーマが書かないところは Go に残します。runn を上げても自動では変わりません。
   - `loop` の mapping には `count` が必要。スキーマは必須にしていない
   - `interval` や時間のフィールドは数値も受ける。スキーマの book の `interval` は文字列
   - step の `if` は bool も受ける。スキーマは文字列
   - `agent` は `claude` / `copilot` / `codex`。スキーマは任意の文字列
   - SSH の `identityFile` と `identityKey` は同時に書けない
   - `shell` は `bash`、`sh`、または `{0}` を含む文字列
   - include の `path` と `steps` は排他。スキーマは `oneOf` なので、キー集合の和はどちらのモードよりも広い
   - HTTP の path と body、gRPC のメソッド、DB の `query`、runner 種別の判定順。これらは runn の `parse.go` と `book.go`

式の関数は名前を許可リストにしていません。`file()` や `hash()` が増えても、式として解析できればそのまま通ります。

## テストが失敗するとき

`go test ./...` は、毎週の CI からも、次を確認します。

- 取り込んだスキーマと `cdpfn.src` が、`VERSION` のタグのファイルと一致する
- 最新リリースの同じ2ファイルが取り込みと違うとき、失敗する

タグだけが進み、この2ファイルが同じなら失敗しません。依存関係だけのリリースで CI を赤くしないためです。

失敗したら、次の順で更新します。

1. 新しいタグの `runbook.schema.yaml` と `cdpfn.go` を、`internal/runnspec/runbook.schema.yaml` と `internal/runnspec/cdpfn.src` に置き換える
2. `internal/runnspec/VERSION` をそのタグにする
3. 上のパーサ上書きが、今回の changelog とまだ合っているか見る。HTTP や gRPC の中身が変わったリリースでは、ここを手で直す
4. `go test ./...` を再実行する

取り込んだ2ファイルは [k1LoW/runn](https://github.com/k1LoW/runn) の MIT License の著作物です。中身は比較のため、注釈を足さずに写しています。
