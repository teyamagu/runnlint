# ルールファイル

[English](../en/configuration.md)

runnlint が runbook を検査するときの設定です。YAML で書き、同じ内容を JSON で渡すこともできます。JSON は探索対象にならないので、`-c` でパスを指定します。

## 探索

どの名前のファイルを設定として読むかです。

`-c` / `--config` が無いとき、カレントディレクトリから親へ、各ディレクトリで次の順に探して最初の1件を使います。

1. `.runnlint-runn.yaml`
2. `.runnlint-runn.yml`

見つけたファイルは読み飛ばしません。形が違えば終了コード `2` で失敗します。形の違いには、トップレベルに未知のキーがある、型が違う、`version` が `1` でない、YAML として読めない、を含みます。`rules` の項目の中など、トップレベルより下の未知のキーは書き損じとみなし、`warning:` を出したうえで読み込みます。

`--config` で渡したファイルは、未知のキーや `1` 以外の `version` があっても `warning:` を出して読み込みます。

見つからなければ、バイナリに埋め込んだデフォルトを使います。中身は次で確認できます。

```console
runnlint --print-config
runnlint --list-rules
```

`--list-rules` は、読み込んだ結果の `id`、`severity`、種別（`builtin:名前` または `check`）を出します。

## 全体

トップレベルに書くフィールドです。

```yaml
version: 1
extends: default
language: ja

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
| `version` | ルールファイルの版。いま受け付けるのは `1` だけです。省略すると、継承後に `1` が残ります。`1` でないときの扱いは「探索」のとおりです。 |
| `language` | 診断、読み込みエラー、`-h` の表示言語。`ja` または `en`。省略すると日本語です。`--lang` を付けると、こちらが優先します。それ以外の値は終了コード `2` です。表示は選んだ言語の文だけです。baseline に書く組み込み診断は、この設定にかかわらず英語です。 |
| `extends` | `default` または省略でデフォルトを継承します。`none`（大文字小文字を問わない）で継承しません。それ以外の文字列も、いまは継承と同じです。 |
| `ignore` | ディレクトリ走査で読み飛ばすパス。コマンドの引数にファイルを渡したときは適用しません。指定するとデフォルトの一覧の後ろに足します。キーを省略するとデフォルトのままです。 |
| `yamllint` | 一般的な YAML 検査の委譲先。書いた項目だけを上書きします。 |
| `rules` | ルールの一覧。同じ `id` はマージされ、未知の `id` は末尾に足されます。 |

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

ディレクトリを走査して集めたファイルのうち、検査しないパスです。パターンには、ディレクトリもファイルのパスも書けます。

パターンを見るのは、引数がディレクトリのときだけです。引数にファイルを書いたときは、パターンに一致していても検査します。

```console
runnlint .
runnlint examples/fixtures/openapi.yml
```

`ignore` に `examples/fixtures/openapi.yml` があるとき、カレントディレクトリに `examples/` があれば、1つ目は走査でそのパスに当たったファイルを外します。2つ目は引数がファイルなので、同じパターンがあっても検査します。

パターンは、走査で得たパス（スラッシュ区切り）と、そのベース名の両方に当てます。走査で得たパスは、引数のディレクトリを起点に連結したパスです。引数が `.` のときは `examples/fixtures/openapi.yml`、引数が `books` のときは `books/a.yml` です。設定ファイルがあるディレクトリからの相対パスには揃えません。引数が絶対パスのときは、走査で得たパスも絶対パスです。`**` を含むパターンは、走査で得たパスだけに当てます。

`*`、`?`、`[` を含まないパターンは、走査で得たパスがパターンと一致するときと、その下にあるときに当たります。末尾の `/` を除くのは、その下を見るときだけです。`examples/bad` と `examples/bad/` は、どちらも `examples/bad/...` に当たります。`examples/fixtures/openapi.yml/` は、走査パス `examples/fixtures/openapi.yml` には当たりません。パターンに `/` が無いときは、ベース名が一致するファイルにも当たります。`./examples/bad` のように `./` を付けると、走査で得た `examples/bad/...` とは別の文字列なので当たりません。

| 書き方 | 例 |
| --- | --- |
| ファイルのパス | `examples/fixtures/openapi.yml` は、走査で得たパスがこれと一致するファイル。`examples/valid/openapi.yml` はこのパターンに当たらない |
| ディレクトリ | `examples/bad` や `vendor` は、そのパスとその下。`vendor` は `/` を含まないので、ベース名が `vendor` のファイルにも当たる |
| ベース名 | `openapi.yml` は、ディレクトリが違ってもベース名が `openapi.yml` のファイル |
| `*` と `?` | `*.gen.yml` はベース名にも当たる。`dir/*.yml` は `dir` の直下だけ。`*` と `?` はディレクトリ区切りをまたがない |
| `**` | `vendor/**` は、走査で得たパスが `vendor/` 以下のファイル。途中のディレクトリをまたぐ。`other/vendor/a.yml` には当たらない。`**/` は 0 個以上のディレクトリ全体なので、その後ろの `*` と `?` は 1 つの段の中に収まる。`a/**/?.yml` は `a/c.yml` と `a/x/c.yml` に当たり、`a/x/cc.yml` には当たらない |

`.git`、`vendor`、`node_modules` は、`ignore` が無くてもディレクトリごと降りません。include の参照元としても読みません。

`ignore` で検査から外したファイルは、include の参照元としては読みます。範囲は「included-unreferenced の判定範囲」です。

## yamllint

一般的な YAML の検査を、外部コマンドの yamllint に渡す設定です。

| フィールド | 意味 |
| --- | --- |
| `enabled` | `true` / `false`。省略時は有効です。 |
| `bin` | 実行ファイル名。省略時は `yamllint`。 |
| `config` | yamllint 自身の設定ファイル。相対パスは、ルールファイルがあるディレクトリから探します。 |
| `missing` | コマンドが無いときの深刻度。`error`、`warning`、`info`、`off`。省略時は `warning`。 |

`config` を書かないときは、runbook 向けに緩めた組み込み設定を渡します。内容は `runnlint --print-yamllint-config` で見られます。`extends: none` でも、`bin` と `missing` を空のままにすると、それぞれ `yamllint` と `warning` になります。yamllint を止めるには `enabled: false` を書きます。

診断のルール ID は `yamllint/trailing-spaces` のように、yamllint のルール名に接頭辞を付けたものです。yamllint は GPL-3.0 の外部コマンドで、このモジュールには含めません。実行時に外すには `--no-yamllint` を使います。

## ルールの共通項目

ルールを1件書くときのフィールドです。組み込みの深刻度を変えるときも、YAML だけでルールを足すときも、ここを書きます。

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
| `uses` | 組み込み実装の名前。`id` と別の名前にもできます。 |
| `severity` | `error`、`warning`、`info`、`off`。省略すると `error`。 |
| `message` | 宣言的ルールの診断文。 |
| `files` | このルールを当てるファイルのパターンの配列。省略するとすべてのファイルに当てます。 |
| `check` | 宣言的ルールの条件。`uses` と同時には書けません。 |
| `options` | 組み込み実装へ渡す追加設定。 |

`off` 以外では、`uses` か `check` のどちらかが必要です。`check.select` は空にできません。未知の `uses` は実行前に失敗し、終了コードは `2` です。

### files

`files` を書くと、パターンに当たるファイルにだけそのルールを当てます。組み込みルール（`uses`）にも宣言的ルール（`check`）にも効きます。継承したルールに `files` だけを書いて、範囲を狭めることもできます。

```yaml
rules:
  - id: integrations-only-include
    severity: error
    files: ['integrations/**']
    check:
      select: 'steps.*'
      where:
        - path: include
          exists: false
      forbid: true
  - id: require-desc
    files: ['books/**']
```

パターンの当て方は「ignore」と同じです。走査で得たパス（スラッシュ区切り）とそのベース名に当て、`**` を含むパターンは走査で得たパスだけに当てます。違いは1つで、`ignore` は引数がディレクトリのときだけ見ますが、`files` は引数にファイルを渡したときにも見ます。`runnlint integrations/a.yml` と書けば、`integrations/**` のルールが当たります。走査で得たパスは引数のディレクトリを起点にするので、`runnlint .` では `integrations/a.yml`、`runnlint books` では `books/integrations/a.yml` です。`files: []` と、空のパターンは、設定の読み込みで失敗します。

終了コードは、`error` が1件でもあれば `1`、設定や実行の失敗は `2`、それ以外は `0` です。`warning` と `info` だけでは失敗しません。コマンドに `--strict` を付けると `warning` でも `1` になります。`info` は失敗しません。ルールファイルにこの設定はありません。

## 組み込みルール

`uses` に書ける名前、初期の深刻度、その名前が知らせる失敗です。`id` を同じ名前にして `severity` だけ変えるのが通常です。

| uses | デフォルト | 知らせる失敗 |
| --- | --- | --- |
| `duplicate-key` | error | 同じ mapping に、同じキーが2回書いてある |
| `unknown-key` | error | トップレベルに、runbook のフィールドでも `x-` 始まりでも `<<` でもないキーがある |
| `book-shape` | error | runbook 直下のフィールドの値が、runn の形になっていない |
| `require-steps` | error | 実行する step が無い |
| `label-format` | error | label が空であるか、空白や `!+=|.*%^?><` を含んでいる。文字列でないときもこの id です |
| `runner-reserved` | error | runner の名前に、`test` や `exec` などの予約語を使っている |
| `runner-config` | error | runner の種別が分からない、またはその種別の設定が runn の形になっていない |
| `runner-defined` | error | step が、定義されていない runner を呼んでいる |
| `step-shape` | error | 1つの step で、実行する処理が1つに決まっていない。`if` や `dump` など、step に添える値の形が違うときもこの id です |
| `http-request` | error | HTTP の呼び出しが runn の形になっていない |
| `grpc-request` | error | gRPC の呼び出しが runn の形になっていない |
| `db-request` | error | DB の step に、実行する SQL が無い。`trace` の形が違うときもこの id です |
| `exec-request` | error | exec に、実行するコマンドが無い、または付随する項目の形が違う |
| `ssh-request` | error | SSH の step に `command` 以外が書いてある、または `command` が無い |
| `cdp-request` | error | ブラウザ操作の `actions` が、runn の操作として成り立っていない |
| `agent-request` | error | agent への依頼文が無い、または形が違う |
| `include-request` | error | include が、取り込む runbook を1つに決めていない |
| `include-path` | warning | include の参照先ファイルが無い |
| `needs-path` | warning | `needs` の参照先ファイルが無い |
| `local-path` | warning | runner が参照する証明書や proto などのファイルが無い |
| `expr-syntax` | error | `{{ }}` や `test` などの式が、式として読めない |
| `expr-braces` | warning | 式として読む欄を `{{ }}` で包んでいる |
| `expr-sharp-comment` | info | `test`、`if`、`until`、`bind` などの式に、括弧の外の `#` コメントがある。runn は取り除いて動くが「`#` comment is deprecated. Use `//` instead.」と警告する。文字列の中の `#` と、`( )` や `{ }` の中のクロージャの `#` は出さない。`{{ }}` の中は見ない |
| `step-ref` | error | 配列の step を名前で参照している、名前付きの step を番号で参照している、またはまだ実行されていない step を参照している |
| `var-ref` | warning | `vars` に無い変数を参照している |
| `loop-index` | warning | `loop` の外で添字 `i` を使っている |
| `previous-step` | warning | 直前の step が無い場所で `previous` を使っている |
| `defer-experimental` | info | `defer` は実験的機能だという知らせ |
| `unused-runner` | warning | `runners` や `runner:` で定義した runner を、どの step も呼んでいない。inline include の中の step が呼ぶものは使っているとみなします。予約語の名前は `runner-reserved` に任せます |
| `unused-bind` | warning | `bind` で付けた名前を、このファイルの式も、この runbook を include・needs するファイルの式も読んでいない。範囲は「unused-bind の判定範囲」 |
| `require-test-step` | info | 単独で実行できる runbook に、`test` だけの step が無い。リクエストに付けた `test` は1つの応答の確認で、`test` だけの step はシナリオで確かめたいことを名前にします。トップレベルの `if` が `included` を要する runbook は呼び出し側が確かめるので対象外です。step が無い runbook は `require-steps` に任せます |
| `included-unreferenced` | warning | include されたときだけ動く runbook が、どこからも include されていない |
| `unused-suppression` | warning | 抑制のコメントが、診断を抑制できていない |

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
- include 元が見つからず、トップレベルの `if` が `included` の runbook は、ファイルを1つだけ渡したときも `vars` を調べません。include されて動く前提です。
- それ以外の runbook は、これまでどおり、その runbook の `vars` だけを見ます。

`http-request` は `options.require-leading-slash` を読みます。`true` にすると、`/` でも `://` でも始まらない path（`graphql` など）を、この id の warning にします。runn は endpoint と path をつないで動かすので、既定ではこの path を失敗にしません。

```yaml
rules:
  - id: http-request
    severity: error
    options:
      require-leading-slash: true
```

### 式と step の参照

`expr-syntax` が見るのは、`{{ }}` の中と、`test`、`if`、`until`、`bind` です。`{{ }}` は mapping の値だけでなく、HTTP のパスや body の項目名のようなキーの中も見ます。runn はキーも展開するためです。検査の前に、値の `${...}` を展開し、括弧の外の `#` から行末をコメントとして除きます。`test` と `bind` の式の中にある `{{` はテンプレートにしません。`bind` は文字列のほか、mapping、配列、数値も受け、中の文字列を式として検査します。

`expr-braces` の `test` と `bind` は、値全体が `{{ }}` のときだけ出します。文字列の中の `{{` は見ません。

`step-ref` では、自分自身の step は参照できます。

### unused-bind の判定範囲

`bind` で付けた名前は、同じ runbook の式から名前のまま読めるほか、include した側からは `steps.<n>.<名前>` や `previous.<名前>`、`needs` で読み込んだ側からは `needs.<名前>.<bind 名>` で読めます。このルールは次の順で「読まれている」と判定し、どれにも当たらない bind を出します。

- このファイルの式（`test`、`if`、`until`、`dump`、`bind` の値、`{{ }}`）のどこかに、その名前が識別子かメンバー名として出てくる
- このファイルを固定パスで include するファイル、`{{ }}` や `${...}` を含むパスで include しうるファイル、`needs` に書いたファイルの式のどこかに、その名前が出てくる。`include.vars` の値は runn が呼び出し側の式として評価するので、`{{ }}` が無くても式として読めれば数えます。参照元を読む範囲は「included-unreferenced の判定範囲」と同じです

次のときは判定しません。トップレベルの `if` が `included` を要する runbook を、固定パスで include も needs もするファイルが無いとき（呼び出し元が走査の外にあるかもしれないため）。このファイルか参照元のどれかに、式として読めない値があるとき。`a.b` や `a[0]` のような bind のキーは、すでにある `a` への代入なので、新しい名前ではなく `a` の参照として数えます。

### included-unreferenced の判定範囲

`if: included` は「include されたときだけ動く」という意味です。どこからも include されていない runbook は実行されず、置き去りになっています。このルールは、ほかのファイルからの参照を見ます。見る範囲は次のとおりです。

- 判定するのは、ディレクトリを渡して走査したファイルと、`--include-root` の中のファイルだけです。ファイルを直接渡したときは、そのファイルが `--include-root` の中に無ければ判定しません。
- 参照元として、走査したディレクトリと `--include-root` の中の YAML をすべて読みます。`ignore` で検査から外したファイルも参照元として読みます。直接渡したファイルも参照元になります。
- include のパスは `include-path` と同じく、include する側のファイルのあるディレクトリから解決します。`include: path`、`include.path`、include runner の `path` を参照として数えます。
- `{{ }}` や `${...}` を含むパスは値が分からないので、最後の動的な部分より後ろの文字列で終わるファイルをすべて参照されていると数えます。`{{ vars.name }}.yml` なら、すべての `.yml` が該当します。
- `if` が `included || ...` や `!included` のように単体でも真になりうる式は対象外です。`included && ...` は、include されたときにしか動かないので対象にします。

include 元が走査範囲の外にあるときは、親ディレクトリを `--include-root` に足してください。

```console
runnlint --include-root . books/payments
```

デフォルトには、宣言的ルールも入っています。`require-desc` は `desc` が無いこと、`no-debug` は `debug` が有効なこと、`loop-until-with-test` は同じ step に `loop.until` と `test` があることを知らせます。`test` は繰り返す前に評価されるので、リトライが始まる前に失敗します。これらに `uses` はありません。

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

どの値に条件を当てるかです。ドット区切りです。`*` は mapping の各値、または sequence の各要素で、1段だけ降ります。`**` は 0 段以上の任意の段で、mapping の値と sequence の要素を深さを問わず辿ります。`steps` が list でも map でも `steps.*` で各 step を指します。

| select | 指すもの |
| --- | --- |
| `desc` | トップレベルの `desc` |
| `vars.password` | `vars` の `password` |
| `steps.*` | 各 step |
| `steps.*.test` | 各 step の `test` |
| `steps.*.**.query` | どの runner 名、どの path やメソッドの下にあっても、step の中の `query` |
| `vars.**` | `vars` の下にあるスカラーすべて。深さは問いません |
| `steps.*.req."/v1/{id}.json".get` | `.` を含むキー `/v1/{id}.json` を1つの段として辿る |

`**` が途中にあるときは、その後ろの段が見つかるまで何段でも降ります。0 段も含むので、`vars.**.flat` は `vars.flat` にも当たります。`**` が最後にあるときは、そこから下のスカラーをすべて選びます。選んだ値自身がスカラーなら、それを選びます。mapping や sequence そのものは選びません。

キーに `.` が含まれるときは、その段を二重引用符で囲みます。囲んだ段は `*`、`**`、`$` を書いても文字どおりのキーとして扱います。`vars."*"` はキー `*` の値です。閉じていない引用符と、閉じ引用符の直後に `.` 以外が続く書き方は、設定の読み込みで失敗します。囲んだ中の `"` はエスケープできません。

list の要素を選んだときの `key` は `0` から始まる番号、map のときの `key` はキー名です。`path` は `steps[0]` や `steps.login` のような位置です。`.` を含むキーは、`path` でも `steps[0].req."/v1/{id}.json".get` のように二重引用符で囲んで出します。

### 判定

選んだ値が、どんなときに診断になるかです。書いた条件をすべて当て、1つでも外れると診断です。

| 条件 | 診断になるとき |
| --- | --- |
| `exists: true` | `where` に合う値が1件も無い。このときは他の条件を見ません |
| `exists: false` | `where` に合う値が1件以上ある。このときは他の条件を見ません |
| `kind` | 値の型が違う。`mapping`、`sequence`、`string`、`number`、`bool`、`null` のどれかを書きます。他の条件より先に見ます |
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

`kind` は、選んだ値の型を固定します。list 形式の `steps` を禁じるには、`select: steps` と `kind: mapping` を書きます。`steps` が無いときは何も選ばないので、診断になりません。`steps` が無いことは `require-steps` が知らせます。`kind` の既定の文は「値の型が sequence で、mapping ではありません」のように、実際の型と期待した型を出します。未知の型名は設定の読み込みで失敗します。

```yaml
rules:
  - id: steps-must-be-map
    severity: error
    check:
      select: steps
      kind: mapping
```

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

`where` は、判定の前に値を絞ります。一覧でも、mapping 1件でも書けます。複数あるときはすべてに合う値だけを残します。`where` に書けるのは `path`、`exists`、`kind`、`equals`、`matches` です。`path` は、選んだ値からの相対パスです。`select` と同じく、`.` を含むキーは二重引用符で囲みます。`path` に `*` と `**` は書けません。`exists`、`forbid`、通常の判定のどれでも、`where` で残った値だけを見ます。

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

`path` を `$.if` のように `$.` から始めると、選んだ値からの相対ではなく、runbook のルートから辿ります。`$` だけならルートそのものです。選んだ値によらない条件なので、runbook ごとに1回だけ見る門として働きます。1つでも合わないと、そのルールはその runbook に何も出しません。合えば、相対パスの条件で値を絞り、判定に進みます。`exists: true` と組み合わせると、「トップレベルの `if` が無いのに `labels` が無い」のような、runbook の種類で変わる必須項目が書けます。

```yaml
rules:
  - id: included-vars-have-no-id-default
    severity: warning
    check:
      select: 'vars.*'
      where:
        - path: $.if
          matches: included
      not_matches: '^[0-9A-Za-z]{26}$|^[0-9a-fA-F-]{36}$'
  - id: standalone-needs-labels
    severity: warning
    check:
      select: labels
      where:
        - path: $.if
          exists: false
      exists: true
```

### message

診断に出す文です。空なら、条件に応じた文を出します。書いた場合は [Go の text/template](https://pkg.go.dev/text/template) です。使えるデータは次の4つです。未知の名前は空文字になります。

| 名前 | 中身 |
| --- | --- |
| `{{ .path }}` | `vars.password` や `steps[0]` |
| `{{ .key }}` | 最後のキー、または list の番号 |
| `{{ .value }}` | スカラーの文字列。mapping や sequence は空 |
| `{{ .rule }}` | ルールの `id` |

`exists: true` でパスが無いときの `path` は、存在する最も深い親です。`*` と `**` の手前で止まるので、`steps.*.**.timeout` が無いときは `steps` です。トップレベルが無いときは空です。このとき `value` は `null` です。
