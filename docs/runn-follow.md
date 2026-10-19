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

### 公式スキーマで検証しない理由

runn の公式 JSON Schema（`runbook.schema.yaml`）は、上の 1 のとおりキーと `required` の出どころとして使っています。一方で、スキーマ全体を JSON Schema の検証器に通して runbook の形を判定することは、いまはしていません。理由は次のとおりです。

- スキーマとパーサが食い違うところ（上の 3 の一覧）で、runn が通す runbook を不正と判定してしまう。誤検知を減らす方針と逆になる
- step の HTTP / gRPC / DB の中身はスキーマの対象外なので、`http-request` などの検査は Go に残る
- 検証器のエラーは JSON Pointer で出るので、YAML の行と列へ戻す仕組みが別に要る。anchor や merge（`<<`）を含むと対応がずれる

今後の方針は次のとおりです。

1. キーの集合と `required` は、これまでどおりスキーマから読む。runn を上げたときは、スキーマを差し替えるだけで追従する
2. スキーマで表せる型（`desc` は文字列、`labels` は文字列の配列など）は、パーサと食い違わない項目から順にスキーマ由来へ移す。移すときは、上の 3 の上書きをスキーマの外に残す
3. 全体の検証器を入れるのは、パーサとの食い違いが上書きで吸収でき、行への対応が取れてからにする。入れる場合も、まず既定で無効のルールとして足し、実際の runbook で誤検知を測ってから既定にする

式の関数は名前を許可リストにしていません。`file()` や `hash()` が増えても、式として解析できればそのまま通ります。

## テストが失敗するとき

`go test ./...` は、毎週の CI からも、次を確認します。

- 取り込んだスキーマと `cdpfn.src` が、`VERSION` のタグのファイルと一致する
- 最新リリースの同じ2ファイルが取り込みと違うとき、失敗する

タグだけが進み、この2ファイルが同じなら失敗しません。依存関係だけのリリースで CI を赤くしないためです。

毎週の [runn follow](../.github/workflows/runn-follow.yml) は、その差があるときだけ次を行います。

1. 新しいタグの `runbook.schema.yaml` と `cdpfn.go` を、`internal/runnspec/runbook.schema.yaml` と `internal/runnspec/cdpfn.src` に置き換える。`internal/runnspec/VERSION` をそのタグにする。
2. `go test ./...` で、ルールと examples がまだ通るかを確認する。
3. 結果を本文に書いた PR を作る。テストが失敗した PR は draft にする。スキーマに無い規則は自動では変えないので、失敗しているときはパーサ上書きか examples を手で直す。

同じタグの PR やブランチが既にあるときは、新しくは作りません。手で追従するときも、上の 1 から 3 の順です。

取り込んだ2ファイルは [k1LoW/runn](https://github.com/k1LoW/runn) の MIT License の著作物です。中身は比較のため、注釈を足さずに写しています。
