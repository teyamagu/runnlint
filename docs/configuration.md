# ルールファイル

runnlint が runbook を検査するときの設定です。YAML で書き、同じ内容を JSON で渡すこともできます。JSON は探索対象にならないので、`-c` でパスを指定します。

## 探索

`-c` / `--config` が無いとき、カレントディレクトリから親へ、各ディレクトリで次の順に探して最初の1件を使います。

1. `.runnlint-runn.yaml`
2. `.runnlint-runn.yml`
3. `.runnlint.yaml`
4. `.runnlint.yml`
5. `runnlint.yaml`
6. `runnlint.yml`

1 と 2 は runnlint だけが使う名前で、新しく置くときはこちらを使います。3 から 6 は従来の名前で、同じ名前の別のツールの設定ファイルと重なることがあります。そのため従来の名前のファイルは、runnlint のルールファイルの形のときだけ使います。トップレベルに未知のキーがある、型が違う、`version` が 1 でない、YAML として読めない、のどれかなら、標準エラーに `warning:` を出して読み飛ばし、次の候補（同じディレクトリの残りの名前、親ディレクトリ）を探します。従来の名前で書いた runnlint の設定は、これまでどおり読まれます。`rules` の項目の中など、トップレベルより下の未知のキーは runnlint の設定の書き損じとみなし、`warning:` を出したうえで読み込みます。

1 と 2 の固有の名前と、`--config` で渡したファイルは読み飛ばしません。固有の名前は形が違えば終了コード `2` で失敗します。形の違いには、`version` が `1` でないことも含みます。`--config` のファイルは後方互換のため、未知のキーや `1` 以外の `version` があっても `warning:` を出して読み込みます。

見つからなければ、バイナリに埋め込んだデフォルトを使います。中身は次で確認できます。

```console
runnlint --print-config
runnlint --list-rules
```

`--list-rules` は、読み込んだ結果の `id`、`severity`、種別（`builtin:名前` または `check`）を出します。

## 全体

```yaml
version: 1
extends: default

ignore:
  - examples/bad
  - vendor/**

yamllint:
  enabled: true
  bin: yamllint
  missing: warning
  # config: .yamllint.yaml

rules:
  - id: var-ref
    severity: warning
    options:
      ignore:
        - token
```

| フィールド | 意味 |
| --- | --- |
| `version` | いまは `1` だけを受けます。省略すると、継承後にデフォルトの `1` が残ります。`1` 以外のとき、固有の名前は終了コード `2` で失敗し、従来の名前は `warning:` を出して読み飛ばします。`--config` で渡したファイルは `warning:` を出して読み込みます。 |
| `extends` | `default` または省略でデフォルトを継承します。`none`（大文字小文字を問わない）で継承しません。それ以外の文字列も、いまは継承と同じです。 |
| `ignore` | ディレクトリ走査で読み飛ばすパス。指定するとデフォルトの一覧の後ろに足します。キーを省略するとデフォルトのままです。 |
| `yamllint` | 一般的な YAML 検査の委譲先。書いた項目だけを上書きします。 |
| `rules` | ルールの一覧。同じ `id` はマージされ、未知の `id` は末尾に足されます。 |

`extends: none` でも、`yamllint.bin` と `yamllint.missing` を空のままにすると、それぞれ `yamllint` と `warning` になります。yamllint を止めるには `yamllint.enabled: false` を書きます。

## 継承

デフォルトのルールを残したまま、一部だけ変えるのが通常の書き方です。

同じ `id` の項目は、次の規則で重なります。

- `severity` を書くと、その値に変わる。
- `message` を書くと、その文字列に変わる。組み込みルールは診断文を自分で出すので、`message` は宣言的ルールにだけ効きます。
- `options` はキーごとに上書きする。リストの中身は結合しません。
- `check` を書き `uses` を書かないと、組み込み実装を外して宣言的ルールに差し替える。
- `uses` を書くと、その組み込み実装に差し替える。同時に `check` を書かない場合、既存の `check` は消えます。

1つのファイルに同じ `id` を複数書くとエラーにはならず、後ろの項目が前の項目へ同じ規則で上書きされます。`extends: none` のときはマージせず、同じ `id` が2つあると読み込みに失敗します。

`severity` を省いた項目は、マージの前に `error` になります。デフォルトが `warning` のルールへ `options` だけ足したいときは、`severity` も明示します。省くと `error` に上がります。

```yaml
rules:
  - id: var-ref
    severity: warning
    options:
      ignore:
        - token
```

ルールを止めるときは、その `id` に `severity: off` を書きます。`off` のルールは `uses` も `check` も要りません。

```yaml
rules:
  - id: var-ref
    severity: off
```

## ignore

ディレクトリを渡したときだけ使います。ファイルパスを引数に書いた場合は、`ignore` に一致しても検査します。

パターンは、走査で得たパス（スラッシュ区切り）と、そのベース名の両方に当てます。

| 書き方 | 例 |
| --- | --- |
| パスそのもの | `examples/fixtures/openapi.yml` |
| 末尾がディレクトリ | `vendor` は `vendor/...` を含む |
| `*` と `?` | `*.gen.yml` はベース名にも一致する |
| `**` | `vendor/**` は途中のディレクトリをまたぐ |

`*` はディレクトリ区切りをまたぎません。`.git`、`vendor`、`node_modules` は、`ignore` が無くてもディレクトリごと降りません。

## yamllint

| フィールド | 意味 |
| --- | --- |
| `enabled` | `true` / `false`。省略時は有効です。 |
| `bin` | 実行ファイル名。省略時は `yamllint`。 |
| `config` | yamllint 自身の設定ファイル。相対パスは、ルールファイルがあるディレクトリから探します。 |
| `missing` | コマンドが無いときの深刻度。`error`、`warning`、`info`、`off`。省略時は `warning`。 |

`config` を書かないときは、runbook 向けに緩めた組み込み設定を渡します。内容は `runnlint --print-yamllint-config` で見られます。

診断のルール ID は `yamllint/trailing-spaces` のように、yamllint のルール名に接頭辞を付けたものです。yamllint は GPL-3.0 の外部コマンドで、このモジュールには含めません。実行時に外すには `--no-yamllint` を使います。

## ルールの共通項目

```yaml
rules:
  - id: no-plain-password
    severity: error
    message: "vars.password は ${VAR} を参照してください"
    check:
      select: vars.password
      matches: '^\$\{.+\}$'
```

| フィールド | 意味 |
| --- | --- |
| `id` | 診断に出る名前。設定内で一意です。 |
| `uses` | 組み込み実装の名前。`id` と同じにする必要はありません。 |
| `severity` | `error`、`warning`、`info`、`off`。省略すると `error`。 |
| `message` | 宣言的ルールの診断文。 |
| `check` | 宣言的ルールの条件。`uses` と同時には書けません。 |
| `options` | 組み込み実装へ渡す追加設定。 |

`off` 以外では、`uses` か `check` のどちらかが必要です。`check.select` は空にできません。未知の `uses` は実行前に失敗し、終了コードは `2` です。

終了コードは、`error` が1件でもあれば `1`、設定や実行の失敗は `2`、それ以外は `0` です。`warning` と `info` だけでは失敗しません。

## 組み込みルール

`uses` に書ける名前と、デフォルトの深刻度です。

| uses | デフォルト | 見ているもの |
| --- | --- | --- |
| `duplicate-key` | error | 同一 mapping の明示的なキー重複 |
| `unknown-key` | error | トップレベルのうち、runbook のフィールドでも `x-` 始まりでも `<<` でもないキー |
| `book-shape` | error | `desc`、`labels`、`needs`、`runners`、`vars`、`secrets`、`hostRules`、`debug`、`interval`、`if`、`skipTest`、`loop`、`concurrency`、`force`、`trace` の型 |
| `require-steps` | error | `steps` が1件以上あること |
| `label-format` | error | label が `^[^ \n\r!+=|.*%^?><]+$` に合うこと |
| `runner-reserved` | error | `test` `dump` `bind` `include` `exec` `runner` `if` `desc` `loop` `defer` `force` を runner 名に使っていないこと |
| `runner-config` | error | runner の種別判定と、種別ごとの設定キー |
| `runner-defined` | error | step の runner が `runners` か、それより前の `runner:` で定義されていること |
| `step-shape` | error | step の main runner は1つ。`runner:` は他の runner と同じ step に書けない |
| `http-request` | error | path、メソッド、`headers` / `body` / `useCookie` / `trace`。`/` で始まらない path は既定では見ない（下の `options`） |
| `grpc-request` | error | `package.Service/Method`、`message` と `messages` の排他 |
| `db-request` | error | 空でない `query` と、任意の `trace` |
| `exec-request` | error | `command`、`shell`、`stdin`、`background`、`liveOutput`、`env` |
| `ssh-request` | error | `command` だけ |
| `cdp-request` | error | `actions` とアクション名、引数 |
| `agent-request` | error | `prompt`、`clearContext` |
| `include-request` | error | `path` と `steps` はどちらか一方 |
| `include-path` | warning | include の path と、include runner の path がファイルとして存在すること |
| `needs-path` | warning | `needs` の値がファイルとして存在すること |
| `local-path` | warning | HTTP / gRPC / SSH runner の証明書や proto などのローカルパスが存在すること |
| `expr-syntax` | error | `{{ }}` の中と、`test` / `if` / `until` / `bind` の式。式の前に値の `${...}` を展開し、括弧の外の `#` から行末をコメントとして除く。`test` と `bind` の式の中にある `{{` はテンプレートにしない。`bind` は文字列のほか、mapping、配列、数値も受け、中の文字列を式として検査する |
| `expr-braces` | warning | 式として読むフィールドを `{{ }}` で包んでいないこと。`test` と `bind` は、値全体が `{{ }}` のときだけ。文字列の中の `{{` は対象外 |
| `step-ref` | error | list は `steps[n]`、map は `steps.name`。まだ実行されていない step は参照できない。自分自身は参照できる |
| `var-ref` | warning | `vars.name` が、その runbook の `vars` か、include 元の `include.vars` にあること。`parent.vars` は include の内側だけで、親の `vars` を見る |
| `loop-index` | warning | `i` は `loop` の中だけ |
| `previous-step` | warning | 先頭 step と book 直下では `previous` を使えない |
| `defer-experimental` | info | `defer` は実験的機能 |
| `included-unreferenced` | warning | トップレベルの `if` が `included`（`included == true` や `included && ...` を含む）の runbook が、走査した範囲のどこかから include されていること |
| `unused-suppression` | warning | `# runnlint disable-line` のうち、存在しないルールを書いたもの、何も抑制しなかったもの、複数行スカラーの中に書いたもの、`disable-line` 以外の指示。書き方は README の「インライン抑制」 |

`options` を読む組み込みルールは `var-ref` と `http-request` です。

```yaml
rules:
  - id: var-ref
    severity: warning
    options:
      ignore:
        - token
```

`ignore` は文字列の配列です。ここに書いた名前は、`vars` にも `parent.vars` にも無くて警告しません。CLI から渡す変数向けです。

include 元から渡す変数は、`ignore` に書かなくても数えます。lint の前に、走査したディレクトリと `--include-root`、引数のファイルから include を集めます（`included-unreferenced` と同じ範囲です）。

- include 元が見つかったときは、どれかの include 元が `include.vars` で渡す変数を定義済みとします。`include.vars` が mapping でなく中身が分からないときは、そのファイルの `vars` を調べません。`parent.vars` も、include 元ごとに違うので調べません。
- パスに `{{ }}` や `${...}` を含む include は、参照先になりうるファイルに、渡しうる変数として `include.vars` を足すだけです。include 元が見つかったとは扱いません。
- include 元が見つからず、トップレベルの `if` が `included` の runbook は、include されて動く前提なので `vars` を調べません。ファイルを1つだけ渡したときに誤検知しないためです。
- それ以外の runbook は、これまでどおり、その runbook の `vars` だけを見ます。

`http-request` は `options.require-leading-slash` を読みます。`true` にすると、`/` でも `://` でも始まらない path（`graphql` など）を warning にします。runn は endpoint と path をつないで動かすので、既定では見ません。

```yaml
rules:
  - id: http-request
    severity: error
    options:
      require-leading-slash: true
```

### included-unreferenced の判定範囲

`if: included` は「include されたときだけ動く」という意味です。どこからも include されていない runbook は実行されず、置き去りになっています。このルールは、ほかのファイルからの参照を見る唯一のルールです。参照元が lint の対象に入っていないと誤検知になるので、判定する範囲を次のように決めています。

- 判定するのは、ディレクトリを渡して走査したファイルと、`--include-root` の中のファイルだけです。ファイルを直接渡したときは、そのファイルが `--include-root` の中に無ければ判定しません。
- 参照元として、走査したディレクトリと `--include-root` の中の YAML をすべて読みます。`ignore` で検査から外したファイルも参照元として読みます。直接渡したファイルも参照元になります。
- include のパスは `include-path` と同じく、include する側のファイルのあるディレクトリから解決します。`include: path`、`include.path`、include runner の `path` を参照として数えます。
- `{{ }}` や `${...}` を含むパスは値が分からないので、最後の動的な部分より後ろの文字列で終わるファイルをすべて「参照されている」とみなします。`{{ vars.name }}.yml` なら、すべての `.yml` が該当します。指摘しない側に倒すためです。
- `if` が `included || ...` や `!included` のように単体でも真になりうる式は対象外です。`included && ...` は、include されたときにしか動かないので対象にします。

include 元が走査範囲の外にあるときは、親ディレクトリを `--include-root` に足してください。

```console
runnlint --include-root . books/payments
```

デフォルトには、宣言的ルールの `require-desc`、`no-debug`、`loop-until-with-test` も入っています。これらに `uses` はありません。

## 宣言的ルール

`check` は、runbook の値を選んで条件を見ます。コードは要りません。

```yaml
rules:
  - id: no-plain-password
    severity: error
    message: "{{ .path }} は ${VAR} を参照してください"
    check:
      select: vars.password
      matches: '^\$\{.+\}$'
```

### select

ドット区切りです。`*` は mapping の各値、または sequence の各要素です。`steps` が list でも map でも `steps.*` で各 step を指します。

| select | 指すもの |
| --- | --- |
| `desc` | トップレベルの `desc` |
| `vars.password` | `vars` の `password` |
| `steps.*` | 各 step |
| `steps.*.test` | 各 step の `test` |

list の要素を選んだときの `key` は `0` から始まる番号、map のときの `key` はキー名です。`path` は `steps[0]` や `steps.login` のような位置です。

### 判定

選んだ各値に、書いた条件をすべて当てます。1つでも外れると診断です。

| 条件 | 診断になるとき |
| --- | --- |
| `exists: true` | パスが1件も無い。このときは他の条件を見ません |
| `exists: false` | パスが1件以上ある。このときは他の条件を見ません |
| `equals` | 値が違う。`null` は YAML の null と比較できます |
| `not_equals` | 値が同じ |
| `matches` | 文字列か数値が、正規表現に一致しない |
| `not_matches` | 文字列か数値が、正規表現に一致する |
| `key_matches` | キーが正規表現に一致しない |
| `key_not_matches` | キーが正規表現に一致する |
| `one_of` | 値が一覧のどれとも違う |
| `min_len` | 文字列の文字数、sequence の件数、mapping の件数が小さい |
| `max_len` | 同じく、大きい |
| `forbid: true` | `where` に合った値がある。他の条件は見ません |

正規表現は Go の RE2 です。壊れていると、その runbook のルートに診断を出します。

`key_matches` と `key_not_matches` は、選んだ値のキーを見ます。map の `steps.*` では step 名、list の `steps.*` では `0` から始まる番号です。診断の位置はキーの行です。list にはキーのノードが無いので、値の行になります。既定のルールに step 名の規約はありません。必要なプロジェクトが書きます。既存の step 名は baseline で許容できます。

```yaml
rules:
  - id: step-name
    severity: warning
    message: "step 名 {{ .key }} が規約に合いません"
    check:
      select: steps.*
      key_matches: '^[a-z][a-z0-9-]*$'
```

`where` は、判定の前に値を絞ります。一覧でも、mapping 1件でも書けます。複数あるときはすべてに合う値だけを残します。`where` に書けるのは `path`、`exists`、`equals`、`matches` です。`path` は、選んだ値からの相対パスです。

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

`where` の1件は、次のようにも書けます。

```yaml
where:
  path: test
  exists: true
```

### message

空なら、条件に応じた文を出します。書いた場合は [Go の text/template](https://pkg.go.dev/text/template) です。使えるデータは次の4つです。未知の名前は空文字になります。

| 名前 | 中身 |
| --- | --- |
| `{{ .path }}` | `vars.password` や `steps[0]` |
| `{{ .key }}` | 最後のキー、または list の番号 |
| `{{ .value }}` | スカラーの文字列。mapping や sequence は空 |
| `{{ .rule }}` | ルールの `id` |

`exists: true` でパスが無いときの `path` は、存在する最も深い親です。トップレベルが無いときは空です。
