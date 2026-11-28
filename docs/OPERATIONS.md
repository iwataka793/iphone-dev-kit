# 運用の手順書（Claude 向け）

**名前**：claude-hands（クロードハンド。2026-11-26 に iphone-dev-kit から変えた）。カオルは「クロードハンドで〜」とも、ふつうに「PC の〜を見て」とも頼む。どちらでもこの道具を使う

この repo で iPhone アプリを作るときに、Claude が道具を使いこなすための手順書です。
この repo には、iPhone アプリの開発のほかに、**Claude が PC と iPhone を見て・動かす道具**（拡張機能 2 つ：iPhone の道具 [MCP.md](MCP.md)・PC の道具 [PC.md](PC.md)）と、カオルが任せる範囲を決める**スイッチの窓**（tools/switch-app）も入っている。PC の容量・不具合・片付け・アプリの出し入れも、この repo の PC の道具でやる。
新しいセッションでは、まずこれを全部読んでから作業します。経過・分かったことの記録は [LOG.md](LOG.md)（新しいものが上）。

- 「未検証」と書いてある所は、まだ実機で確かめていない。使う前に記録を取って確かめる
- 手順が変わったら、この文書も同じコミットで直す

---

## 1. 全体の形

```
[GitHub]  iwataka793/claude-hands（公開）
   ├─ Actions ios.yml   … main への push で DevKit をビルド → シミュレーターで UI テスト → Releases に build-N（最新版）
   ├─ Actions wda.yml   … WebDriverAgent をビルド → Releases に wda-N（最新版にはしない）
   └─ Actions core-tests.yml … Core/ のテスト（Linux）

[Windows PC]（カオルの PC。Claude はデスクトップアプリ経由でつなぐ）。置き場所の全部と、それぞれ何かは 1-1
   ├─ %USERPROFILE%\devkit\go-ios\ios.exe   … iPhone とやりとりする道具（go-ios v1.3.2）
   ├─ %USERPROFILE%\devkit\switch-app\     … 予備のスイッチの窓（PowerShell。今の窓は devkit\hands\claude-hands-switch.exe）
   ├─ %USERPROFILE%\devkit\installers\     … 入れ物（.mcpb・.msi・.ipa など）
   └─ %USERPROFILE%\devkit\setup\          … 台本（go.bat / go.ps1 / plan.json）、ID などの設定 config.ps1（公開 repo に入れない）、記録（runs\）

[iPhone 16e / iOS 27.2.1]（USB で PC につなぐ）
   ├─ LiveContainer … 自作アプリ（DevKit など）はこの中で動かす。SideStore も中に同梱
   ├─ SideStore     … 無料 Apple ID で署名・7 日ごとの更新（LiveContainer と WebDriverAgent）
   ├─ LocalDevVPN   … SideStore が iPhone 自身にインストールするための VPN
   └─ WebDriverAgent（WebDriverAgentRunner-Runner）… PC から画面を操作するための部品
```

Claude の手元のシェル（Bash）はクラウドの Linux で、PC ではない。PC と iPhone には次の道具で触る（速い順）。

- **PC の道具**（`pc_` で始まる。[PC.md](PC.md)）：PowerShell・容量・不具合の記録・アプリ・ファイル・ごみ箱・画面・窓の部品。PC の作業はまずこれ
- **iPhone の道具**（`iphone_` で始まる。[MCP.md](MCP.md)）：画面・タップ・打つ・ログ・アプリの起動・中のファイル・LiveContainer への入れ替えなど。iPhone の作業はまずこれ
- **ブラウザの道具**（`browser_` で始まる。PC の道具の中。[PC.md](PC.md) の「ブラウザ」）：カオルのいつもの Chrome を、画面を撮らずに文字と番号で読み（browser_read）、何手もまとめて動かす（browser_do）。Chrome の作業はまずこれ。Claude in Chrome・内蔵ブラウザより速い。スイッチは PC と別の「ブラウザ」（許可は共通）
- ファイルを置く・読む（デスクトップアプリの橋渡し：device_commit_files・device_stage_files）
- 画面を見てクリックする（computer use）：上の道具でできないときだけ
- **作業のまとまり（hands）**：PC・ファイル・iPhone の中身を変える作業は、始める前に `hands action=start title=…`、終わったら `hands action=done`（DONE の報告をカオルに見せる）。戻すときは `hands action=undo task=…`（[PC.md](PC.md) の「作業のまとまり」）
- どの道具も、カオルのスイッチ（デスクトップの「Claude 操作スイッチ」）の範囲でだけ動く。足りないと「許可しますか？」の窓が出る

---

### 1-1. PC のどこに何があるか（2026-11-25 に整理した）

カオルに「これは何？」と聞かれたら、この表で答える。ここに無いものは、開発とは関係のないカオルのもの（勝手に動かさない）。

| 場所 | 中身 | 消してよいか |
| --- | --- | --- |
| デスクトップ「Claude 操作スイッチ」 | スイッチの窓を開くショートカット（アイコンは緑と灰色のスイッチ 2 つ） | 消してもよい（pc-setup.bat を動かすとまた作る） |
| デスクトップ `selfIdentity.plist` | 鍵の写し（go-ios を別のフォルダで動かしたときにできたもの）。2026-11-25 にカオルが消した。本物は `devkit\go-ios`。またできたら、カオルに聞いてから消す | — |
| `devkit\switch-app\` | 予備のスイッチの窓（devkit-switch.ps1 / .vbs / .ico。`/old` で開く）。アイコンはデスクトップのショートカットも使う | 消さない |
| `devkit\requests\` | Claude の道具がスイッチ不足のときに置く「頼み」と、窓の返事（*.answer.json） | 消さない |
| `devkit\iphone-mcp\` | iPhone の道具の状態：mode.txt（スイッチ）・last-used.txt・server.log・logs\（iPhone のログ。100 MB ずつ回して 2 つまで。mcp-25 から）・backup\（LiveContainer に入れ替える前の DevKit の控え）・files\ | 消さない（backup の古いものは消してよい） |
| `devkit\pc-agent\` | PC の道具の常駐役（pc-agent.exe）・スイッチ（mode.txt）・合言葉（token.txt）・記録（audit.log・agent.log） | 消さない |
| `%ProgramData%\pc-agent\allow.txt` | 10 分の許可（管理者しか書けない場所） | 触らない |
| `devkit\hands\` | 作業のまとまり：timeline\（TIMELINE）・task.json（今の作業）・done\（DONE の報告）・git-folders.txt（Git で記録するフォルダ）・claude-hands-live.exe（LIVE の窓。スタートアップの「claude-hands LIVE」から起動）・claude-hands-switch.exe（スイッチと「許可しますか？」の窓。デスクトップの「Claude 操作スイッチ」）・asks.jsonl（許可・断るの記録）・tests\（試しの台本） | 消さない |
| `devkit\downloads\` | Claude が落としたもの（2026-11-28 から。カオルのダウンロードには置かない）。14 日たったら容量の点検が取り置きへ | Claude が片付ける |
| `devkit\backup\` | バックアップの台本と結果（2026-11-27）：setup-restore.ps1（システムの保護の設定。管理者で 1 回）・onedrive-policy.ps1（OneDrive のフォルダの移動を止めるポリシー。管理者で 1 回）・copy-onedrive.ps1（OneDrive\PC-Backup へ写す。毎時）・restore-test.ps1（戻す練習。管理者で読むだけ）と、それぞれの結果の .txt / .log | 消さない |
| `%ProgramData%\claude-hands\` | 守りの場所（pc-15 から。pc-setup.bat が UAC で作る）：holding（取り置き。Claude は入れるだけ）・status（毎日の点検の結果）・bin（点検の台本） | 触らない（取り置きから戻すのはカオル） |
| 予定のタスク `\claude-hands\` | RestorePoint（SYSTEM・毎日 12:00 に復元ポイント・30 日たった取り置きを消す・status\daily.txt に結果。pc-15 から台本は bin\hands-daily.ps1）・CopyToOneDrive（カオル・毎時とサインイン時に OneDrive へ写す） | 消さない |
| `OneDrive\PC-Backup\` | デスクトップ・ドキュメント・ピクチャ・ビデオ・ミュージックの写し（元の場所はそのまま）。「クラウドだけ」の印つき（PC には 0.1 MB だけ。ショートカットは OneDrive の決まりで PC に残る）。2026-11-28 に「常に保持」から変えた | 消さない（中身は毎時作り直す） |
| `OneDrive\デスクトップ` ほか | 2026-06-10 に OneDrive のフォルダのバックアップが止まったときに残った前のデスクトップなど（クラウドだけ） | カオルが決める |
| `C:\Program Files\Sandboxie-Plus\`・`C:\Windows\Sandboxie.ini`・`C:\Sandbox\` | 箱（Sandboxie-Plus 1.18.6。2026-11-28 カオルが入れた）。Claude 用の箱 ClaudeBox の設定は pc-setup.bat が入れる（変えるには Sandboxie の設定のパスワードが要る。カオルだけが知る）。C:\Sandbox は箱の中身 | 本体と設定は触らない。箱の中身は Claude が Sandboxie の機能で空にする（PC.md の「箱」） |
| `devkit\go-ios\` | ios.exe・wintun.dll・鍵 selfIdentity.plist（iPhone の信頼はこれと結びついている。消すと iPhone で「信頼」をやり直す） | 消さない |
| `devkit\setup\` | go.bat / go.ps1（道具が無いときの古いやり方）・config.ps1（ID など。公開しない）・files\・記録 | 消さない |
| `devkit\installers\` | 入れ直し用：claude-hands-iphone.mcpb（mcp-25）・claude-hands-pc.mcpb（pc-61）と pc-switch.zip（pc-61。常駐役・スイッチと許可の窓・LIVE と DONE の窓・予備の PowerShell の窓・守りの場所と箱の台本）・iloader の .msi・Apple Devices のインストーラー・WebDriverAgentRunner-ss.ipa・Microsoft.Services.Store.winmd | 新しい版が来たら古い版は消してよい（取り置きへ） |
| Claude の拡張機能 | `AppData\Local\Claude-Data\…\Claude Extensions\local.mcpb.iwataka793.iphone-dev-kit`・`…pc-dev-kit`（.mcpb を入れるとできる）。2026-11-26 に名前を claude-hands に変えたが、拡張機能の中の名前（iphone-dev-kit・pc-dev-kit）はそのまま（変えると別の拡張機能として二重に入るため）。見える名前は「iPhone（claude-hands）」「PC（claude-hands）」 | 触らない（入れ直しは設定の画面から） |
| `ドキュメント\pwtest-docker-code.zip` | Docker に残っていた pwtest（2026-02 の PC 版。CityRank の前身）のイメージ 3 つ（cloudv2:run・cloudv2:test・pwtest-core）から取り出した Python のコード。ほかに写しが無かったので残した | カオルが決める |
| Docker Desktop | 2026-11-25 にアンインストールした。どの開発でも PC の Docker は使っていない（CityRank の観測の箱は GitHub の上で作る）。Git（`C:\Program Files\Git`）は残した（今は使っていないが軽い） | — |
| 開発の道具でアンインストールしたもの（2026-11-25） | Swift Developer Toolkit 6.4.3・Visual Studio Build Tools 2022・Windows SDK・VS Installer（11/24 に Windows で swift test を試すために入れた。今は GitHub の CI で試す）。また要るときは winget で入れ直す | — |
| `AppData\Local\ms-playwright` | Playwright のブラウザ（701 MB・2026-08）。今は使っていないが、カオルが「あとで使うかも」で残した | 消さない |
| `.cache\codex-runtimes` | OpenAI の ChatGPT（Codex）アプリの部品（2.4 GB）。アプリはストア版で入っていて、いつも動いている（サービス「ChatGPT」） | 消さない（アプリを使うなら要る） |
| スタートアップ「新しい Outlook（最小化で起動）」 | Windows の起動時に新しい Outlook（olk）を最小化で開く。新しい Outlook は閉じると通知が来ないので、開いたままにするため（2026-11-25） | 消さない |
| `devkit\setup\outlook-new-check.ps1` | 新しい Outlook の設定の点検（そのまま実行）と戻し（-Fix）。repo の `tools/pc-settings/` が元。手で確かめる一覧も出す | 消さない |
| `devkit\setup\backup\Outlook（最小化で起動）.lnk` | 昔の Outlook（classic）の自動起動のショートカット。新しい Outlook に替えたので外した（両方動くと通知が二重） | classic に戻すときはスタートアップへ戻す |
| `AppData\Local\Microsoft\Olk_old_20261125` | 新しい Outlook の古いデータ（534 MB。7 月の古いアカウントが残っていてサインインできなかったので、リセットの前に名前を変えた）。2026-11-25 にカオルの許可でごみ箱へ | — |
| ダウンロード・デスクトップの .bat | もう無い。スイッチは窓に替えた（2026-11-25 にごみ箱へ）。.bat は Releases の zip の中に予備として残っている | — |

- 2026-11-25 にごみ箱へ送ったもの：ダウンロードの iphone-mcp・pc-mcp（古い .bat と pc-agent.exe）、デスクトップの .bat 7 つ、`devkit\iphone-dev-kit`（11/24 に Windows で swift test を試したときの repo の写しとビルドの残り 174 MB。今は CI で試す。要るときは clone し直す）、古い .mcpb、作業の残り。ごみ箱から戻せる
- 版の確かめ方：iPhone は iphone_status の「版」、PC の道具は拡張機能の server\pc-mcp.exe の日付、常駐役は pc_status の「版」

## 2. 開発の 1 周

1. **書く**：`App/`（画面）と `Core/`（iPhone に依存しないロジック）を直す。動きを確かめたい所には、iPhone のログに出る印を入れる（例：`logger.notice("DEVKIT ...")`、`privacy: .public` を付ける）
2. **ビルド**：main に push → ios.yml が動く
   - 結果は REST で見る（このセッションの gh は GraphQL を使うコマンドが通らないことがある）：
     `gh api 'repos/iwataka793/claude-hands/actions/runs?per_page=5' --jq '.workflow_runs[] | "\(.id) \(.name) \(.status) \(.conclusion) \(.head_sha[0:7])"'`
     失敗したら `gh api repos/iwataka793/claude-hands/actions/runs/<id>/jobs` で段階を見る
   - ファイルは `curl -sL https://github.com/iwataka793/claude-hands/releases/download/<タグ>/<ファイル>` で取れる。最新のタグは `gh api repos/iwataka793/claude-hands/releases/latest --jq .tag_name`。そのタグのコミットは `gh api repos/iwataka793/claude-hands/git/ref/tags/build-N --jq .object.sha`（リリースの説明文にも書いてある）
   - Releases の `build-N` に DevKit.ipa・simulator-check.zip（シミュレーターの画面）・simulator-test.log が置かれる
   - このセッションからは Releases を作れない（403）。作るのは Actions に任せる
3. **シミュレーターで確かめる**：`curl -sL https://github.com/iwataka793/claude-hands/releases/download/build-N/simulator-check.zip` などで取って見る
4. **iPhone に入れる**（人の手なし。2026-11-24 確認済み）：3-5 の手順で PC から LiveContainer に直接置く
   - 予備：カオルに Safari で `https://github.com/iwataka793/claude-hands/releases/latest/download/DevKit.ipa` を開いて LiveContainer で入れてもらう
   - 入ったかどうかは DEVKIT のログの `commit=` か、画面下の `build:` がコミット番号と同じかで確かめる
   - 比べる相手は「入れた build-N のタグのコミット」。ios.yml は App/・Core/・ios.yml・scripts/*.sh が変わったときだけ動くので、ほかのファイル（docs など）だけのコミットでは新しい build-N はできない
   - Actions が実行中なら、終わるまで待ってから比べる
5. **実機で動かして見る**：PC の go.bat（3 章）。ログ・画面・操作が一度にできる
6. **記録**：分かったことは LOG.md の一番上に書く

### 2-0. PC・iPhone が無いときは stage のブランチで用意しておく（2026-11-29〜）

- カオルの PC・iPhone がつながっていない間に作れるもの（確かめが CI でできるもの）は、`stage/…` のブランチで作って push する。CI（pc-mcp build）は stage のブランチでも組み立てと Windows の通しの試験をするが、入れ直し用（pc-N）は作らない（release は main だけ）
- PC がつながったら、明日の確かめ（今の main の版）が済んでから stage を main に入れる → 入れ直し用が 1 つできる → カオルの入れ替えは 1 回 → Claude が箱と実機で確かめる（LOG にその回の「PC で確かめること」の一覧を書いておく）
- 作る前に PC・iPhone で確かめる決まりのもの（LINK-DESIGN・ASK-DESIGN の「PC で確かめること」）は、stage でも作らない（とにかくログ）

### 2-1. Claude が作った PC の台本・道具は、まず箱で試してから渡す（2026-11-28〜。pc-57）

- カオルに「動かしてみて」と頼む前に、**Claude が pc_try で ClaudeBox の中で動かして確かめる**（本物の PC は変わらない・ネットは許可したサイトだけ・管理者なし・鍵などは見えない）
  1. `pc_try path=<台本や道具のファイル>`（または `script=<PowerShell の台本>`）。返事に、終わりのコード（0＝成功）・画面の文字とエラー・増えた／変わった（前 → 後の行）／消えたファイルが出る
  2. だめなら直して、もう一度 `pc_try`（箱はそのまま続く。前の回の結果の上で試せる）。画面の文字をもっと見るときは `action=show lines=…`
  3. 動いたら：本物に入れたいファイルがあれば `action=offer` でカオルの TRY の窓に出す（入れるかはカオルが決める）。試しだけなら `action=discard`
  4. カオルには「箱で試して、こう動いた（コード・画面の文字・変わったファイル）」を添えて渡す
- **pc_powershell も、何かを変える台本は既定で箱の中**（pc-61〜。カオルが決めた B）。読むだけの台本は本物で。本物を変えるのは、頼まれた作業で本物を変える必要があるときだけ `real: true`（許可・控え・確かめは今までどおり）。箱で作ったものを本物に入れたいときは `pc_try action=offer`（入れるのはカオルの TRY の窓）。claude-hands の新しい版を devkit\downloads に落とすのも本物に置く作業なので `real: true`（と ALLOW）
- 箱で試せないもの（管理者が要る・許可していないサイトが要る・箱を嫌うアプリ）は、そう言ってから今までどおりカオルに頼む
- claude-hands 自身の入れ替え（pc-setup.bat の UAC・.mcpb）はカオルの手のまま（Claude が自分の守りを入れ替えられないように）
- 鍵・トークン・パスワードを置くソフト（CLI・パスワード管理・クラウドの道具など）を PC に入れたら、その置き場所を `tools/pc-mcp/switch/box-closed.ps1` に足し、次の pc-setup で箱から見えなくする（一覧のやり方なので、足さないと箱から読める。PC.md の「箱から見えない場所」）

---

## 3. PC の使い方

**PC そのものを調べる・動かすとき**（容量・不具合・アプリの操作など）は、道具に `pc_status` があれば [PC.md](PC.md) の道具を使う。画面を撮って押す computer use より速い。

**ブラウザ（Chrome）を動かすとき**は、道具に `browser_read` があればそれを使う（[PC.md](PC.md) の「ブラウザ」）。読む順は：部品の一覧（browser_read）→ 文字（mode=text）→ ページの中のデータ（browser_js。見えているものだけ）→ 画像（browser_screenshot）は最後。次の手が読めるときは browser_do に何手もまとめる（1 回の往復で済む）。人のまねをしなくてよい（2026-11-26 カオル）

**iPhone のときは、まず MCP（[MCP.md](MCP.md)）が使えるか確かめる**：道具に `iphone_status` があれば、iPhone の確認・操作・DevKit の入れ替えはそれで行う（速い）。スイッチがオフなら、カオルに「見るだけ」か「操作も可」にしてもらう。MCP が無いときだけ、以下の go.bat のやり方を使う。

### 3-1. つながっているか

- `get_device_info` → `connectedFolders` に `%USERPROFILE%\Downloads`（ユーザーフォルダの Downloads） があれば OK
- つながらないとき：PC が寝ている／Claude デスクトップアプリが閉じている。カオルに頼む
- iPhone は USB でつなぎ、ロックを外してもらう（ロック中はつながらないことがある）

### 3-2. 台本を置く（初回・更新時）

最初に、PC の go.ps1 / go.bat が repo と同じかを確かめる（`device_stage_files` で取って `cmp`）。違えば置き直す。

repo の `scripts/pc/` を PC の `%USERPROFILE%\devkit\setup\` に置く（2026-11-25 に Downloads\devkit-setup から移した）。このフォルダが get_device_info の connectedFolders に無ければ、先に `device_request_folder_access` で `~\devkit\setup` をつないでもらう。

1. `SendUserFile` で go.ps1 / go.bat を送る（file_uuid が返る）
2. `device_commit_files` で `<ユーザーフォルダ>\devkit\setup\go.ps1`（get_device_info の connectedFolders の下） などに書く（既にあるなら `force: true`）
3. 初回だけ：`config.example.ps1` を元に `setup\config.ps1` を作って置く（Claude が作れる。LiveContainer の bundle id は `ios apps --list` の LiveContainer の行だけを絞って調べる）
   - 値は PC の中だけに置く。公開 repo に書かない

.ps1 は「UTF-8（BOM 付き）＋ CRLF」でないと、Windows PowerShell 5.1 で日本語が壊れる。repo では `.gitattributes` で CRLF にし、BOM はファイルに入れてある。新しく作るときも同じにする。

### 3-3. 実行する（go.bat）

1. `plan.json`（3-4）を作り、`SendUserFile` → `device_commit_files` で `setup\plan.json` に置く
2. computer use でエクスプローラーを開き、`devkit\setup` の `go.bat` をダブルクリックする
   - 開き方：`computer_open_application("エクスプローラー")`（またはタスクバーのフォルダのアイコン）→ ホームが開くので、アドレス欄の上の一覧から「devkit」→「setup」と開く
   - 許可していないアプリ（通知の窓など）が一番前にあるとクリックが断られる。エクスプローラーの窓を一度押して前に出してからやり直す
   - 初回は `computer_resolve_access(["File Explorer"])` → `computer_request_access`
   - エクスプローラーは click 権限だけ（打鍵・右クリック不可）。ダブルクリックで .bat を動かすのは可
   - 一覧の並びはファイルを置くたびに変わる（新しい順）。**ファイルを置いたら 2 秒待ち、押す直前に `computer_zoom` で名前を確かめてから、別の呼び出しで押す**（zoom と押すのを同じ batch に入れると確かめた意味がない。2026-11-24 に config.ps1 を開いてしまった）
   - 待ちは 1 回 50 秒まで。長いときは分けて待つ
3. 結果を読む：`device_stage_files` で `setup\last-result.txt` を取る。画面は `runs\<日時>-<name>\*.png`
   - かかる時間：WDA ありで 40〜90 秒（トンネル 15 秒＋WDA の起動 5 秒＋操作）。最後の行が `== 完了` なら終わっている。1 行目の時刻が今回のものかも確かめる（前の結果を読まない）
   - iPhone のログの時刻は PC の記録より 2 秒ほど早く出ることがある（時計のずれ）
4. 終わったら `computer_release_lock`

### 3-4. plan.json の書き方

```json
{
  "name": "swipe",
  "match": "DEVKIT",
  "wda": true,
  "target": "livecontainer",
  "actions": [
    {"do": "activate"},
    {"do": "screenshot", "name": "before"},
    {"do": "drag", "from": [195, 250], "to": [195, 550], "duration": 0.2},
    {"do": "tap", "at": [195, 400]},
    {"do": "wait", "sec": 2},
    {"do": "source"},
    {"do": "screenshot", "name": "after"}
  ]
}
```

- よく使う形（DevKit を前に出して上下にスワイプ）：`{"name":"check","match":"DEVKIT","wda":true,"target":"livecontainer","actions":[{"do":"activate"},{"do":"wait","sec":2},{"do":"drag","from":[195,250],"to":[195,550]},{"do":"wait","sec":2},{"do":"drag","from":[195,550],"to":[195,250]},{"do":"screenshot","name":"after"}]}`
  - `activate` で LiveContainer の一覧（マイアプリ）が出たときは、DevKit は動いていない。`{"do":"tapText","text":"実行"}` と `wait` 8 秒を足す。どちらが出るかは、その前の状態しだい
- `match`：iPhone のログから抜き出す文字（正規表現）。結果に最後の 60 行が出る。全部は `runs\...\syslog-match.txt`
- `wda: false`：操作せず、`seconds` 秒ログを取って前後の画面を撮るだけ
- 座標は WebDriverAgent の単位（iPhone 16e は 390×844）。結果の「画面の大きさ」に出る
- `source`：画面の部品一覧（XML）。ボタンの位置や文字を調べるのに使う
- `target`：`livecontainer` は config.ps1 の LiveContainer。ほかのアプリは bundle id を書く
- ほかの操作：`terminate`（アプリを終わらせる）、`tapText`（`"text":"実行"` のように文字で部品を探して押す。`index` で何番目か）、`tree` / `pull` / `push` / `pushdir`（開発者署名のアプリの中のファイル。書き方は go.ps1 の先頭）
- WDA の失敗は理由（返ってきた本文）も結果に出る

### 3-5. DevKit を LiveContainer に直接入れる（確認済み）

LiveContainer は起動前に `LCAppInfo.plist` の `LCPatchRevision` を見て、古ければ実行ファイルにパッチを当てて自分の証明書で署名し直す。これを使う。

1. 今の `LCAppInfo.plist` を取る（plan）：
   `{"name":"lc-pull","wda":false,"actions":[{"do":"pull","app":"livecontainer","path":"/Documents/Applications/io.github.iwataka793.devkit.app"}]}`
   → `device_stage_files` で `runs\<日時>-lc-pull\pulled\io.github.iwataka793.devkit.app\LCAppInfo.plist` を取る（これが控えにもなる）
2. クラウドで作る：Releases `build-N` の DevKit.ipa を取り、
   `python3 -I tools/lc-install/prepare.py DevKit.ipa LCAppInfo.plist out/`
   （.app の中身 ＋ `LCPatchRevision` を消した LCAppInfo.plist ができる）
3. `out/` の各ファイルを `SendUserFile` → `device_commit_files` で `setup\files\` に置く（`force: true`）
4. plan で入れて起動：
   ```json
   {"name":"lc-install","match":"DEVKIT","wda":true,"target":"livecontainer","actions":[
     {"do":"terminate"},
     {"do":"pushdir","app":"livecontainer","local":"files","remote":"/Documents/Applications/io.github.iwataka793.devkit.app"},
     {"do":"activate"},{"do":"wait","sec":2},
     {"do":"tapText","text":"実行"},{"do":"wait","sec":8},
     {"do":"screenshot","name":"after-run"}
   ]}
   ```
   - 先に `terminate` で LiveContainer を止める（動いている実行ファイルを上書きしない）
   - LiveContainer の画面は日本語（ボタンは「実行」）。`tapText` で押せる
   - LiveContainer を起動し直したとき、前に動かしていたアプリがそのまま立ち上がることがある（`tapText` が 0 個でも、DEVKIT のログが出ていれば起動している）
5. 確かめる：DEVKIT のログの `commit=` が新しい番号か、`after-run.png` の画面下 `build:`
- 注意：`pushdir` は上書きだけで、新しい版で無くなったファイルは消えずに残る
- うまくいかないときは、1 の控え（pulled の中身）を `files` にして `pushdir` すれば元に戻せる

### 3-6. 守ること（PC）

- 結果（last-result.txt・iPhone のログ）には、チーム ID 付きの bundle id や端末の名前が入る。LOG.md などに写すときは `<チームID>` のように伏せる
- iPhone の出力は**必要な行だけに絞ってから読む**。`ios apps --list` などはアプリ一覧が全部出る（個人情報）
- SideStore の記録（ConsoleLogs）には Apple ID のメールアドレスやログイン用の一時データが入る。チャットに出さない・repo に入れない
- PC のファイルを消すときは、先にカオルに確認する。消すときは完全削除ではなく「ごみ箱へ」（`Microsoft.VisualBasic.FileIO.FileSystem.DeleteFile(..., 'SendToRecycleBin')`）
- iloader のパスワード（資格情報マネージャー "iloader"）には触らない

---

## 4. go-ios のコツ

| やること | コマンド | 注意 |
| --- | --- | --- |
| ログ | `ios syslog` | **トンネルより先に始める**（後だと失敗する）。トンネル無しで動く |
| トンネル | `ios tunnel start --userspace` | 管理者権限いらない。15 秒ほど待つ |
| 画面 | `ios screenshot --output=x.png` | トンネルが要る |
| アプリ一覧 | `ios apps --list` | 全部出るので必ず絞る |
| WDA 起動 | `ios runwda --bundleid=B --testrunnerbundleid=B --xctestconfig=WebDriverAgentRunner` | テスト本体名は `.xctest` なし（7 章） |
| 転送 | `ios forward 8100 8100` | PC の 8100 番 → iPhone の WDA |
| 落ちた記録 | `ios crash ls` | 名前だけ絞って見る |
| アプリの中のファイル | `ios fsync --app=<bundle id> tree --path=/` / `pull --srcPath= --dstPath=` | 開発者署名のアプリ（LiveContainer など）の中が読める。Windows では tree の字下げが出ない（全部平ら） |
| 受け渡し場所 | `ios fsync tree --path=/PublicStaging` | インストール後は空になる |

- 詳しいやりとりを全部見るとき：`-t`（trace）。エラーの中身（NSError）は go-ios が説明文しか出さないことがある
- `runwda` は `--log-output=<file>` でテスト側のログが取れる
- LiveContainer の中の場所：`/Documents/Applications/<bundle id>.app`（入れたアプリ）、`/Documents/Data/Application/<UUID>/`（アプリのデータ）、`/Documents/SideStore/Documents/`（同梱 SideStore の書類。ConsoleLogs・ResignedApps）

---

## 5. WebDriverAgent（PC から実機を操作する部品）

- ビルド：wda.yml。Releases `wda-N` の **WebDriverAgentRunner-ss.ipa** を使う（`WebDriverAgentRunner.ipa` は旧形式）
- 入れ方：iPhone の Safari で ss.ipa を落とす → LocalDevVPN をオン → SideStore の My Apps「＋」→ 選ぶ
  - LiveContainer の中ではなく iPhone 本体に入る（LiveContainer の中では動かない）
- 更新：SideStore の 7 日更新（Refresh）に任せる。PC 側の自動更新はやめた
- 使う API（go.ps1 の中）：`/status`、`POST /session`、`/session/{id}/wda/apps/activate`、`/session/{id}/window/size`、`/session/{id}/wda/dragfromtoforduration`、`/session/{id}/wda/tap`、`/session/{id}/source`

---

## 6. SideStore / LiveContainer の注意

- SideStore は LiveContainer 同梱版（0.6.4-20260714、LiveContainer/SideStore の ddf08f58）
- **本番ビルドでも DEBUG が有効**で、署名の準備のときに PlugIns の中の「.xctest」を含む名前のものを黙って消す（2026-11-24 の LOG）。テスト部品を含むアプリは名前から `.xctest` を外す（`tools/wda-sidestore/make_ss_ipa.py`）
- LocalDevVPN の Tunnel IP は `10.7.0.0`（SideStore は Tunnel IP + 1 に話しかける）。SideStore の Health Check がすべて緑なら OK
- Anisette サーバーは「SideStore (New)」
- 無料 Apple ID の制限：有効なアプリ 3 つまで、App ID は 7 日で 10 個まで（My Apps の下に残り数）、証明書は 1 年
- 同じ ID のアプリを別の道具（iloader など）で入れると上書きし合い、SideStore の一覧から消える
- 隠しメニュー（DIAGNOSTICS）：Settings で 3 本指で下から上へ素早く 3 回スワイプ。詳しい記録は「Enable Verbose Ops Logging」
- 記録の場所：LiveContainer の中 `/Documents/SideStore/Documents/ConsoleLogs/console-*.log`（ログイン用データを含む）
- 「Export Resigned Apps」をオンにすると、送る直前の署名済みアプリが `ResignedApps/` に残る（調べるとき用）

---

## 7. 調べ方の決まり

- **とにかくログ**：推測やソースを読むより先に、実物（送られたファイル・実機のログ・書き出したもの）を見る。2026-11-24 は、ソースを読んで「入っているはず」と決めて遠回りした
- 原因が分からないときは、わざと同じ失敗を起こして形を比べる（例：無い名前を指定して同じエラーになるか）
- 実際に動いている版のソースを見る（SideStore は LiveContainer 版、AltSign + ldid。upstream とは別物）

---

## 8. 未検証・これから

- 新しいセッションでの試験は 2026-11-24 に済み（LOG 参照）。手順が大きく変わったらもう一度やる

### 次に PC がつながったらやること（2026-11-25 カオルに頼まれた）

PC の道具（[PC.md](PC.md)）を入れて確かめる：
1. （済 2026-11-25。2〜5 も済。残りは DPI と画面・部品の操作）Releases の最新の `pc-N`（いまは pc-4）の `claude-hands-pc.mcpb` と `pc-switch.zip` を PC のダウンロードに置く
2. カオルに `pc-switch.zip` の中の `pc-setup.bat` を 1 回実行してもらい、`claude-hands-pc.mcpb` を拡張機能に入れてもらう
3. カオルに PC-VIEW を押してもらい、`pc_status` → `pc_disk`（引数なし）→ `pc_problems` で、カオルの PC で実際に動くか確かめる
4. PC-ALLOW の流れ（UAC の「はい」→ 10 分の許可 → 切り替えで取り消し）を 1 回試す
5. PC.md の最後の「まだ確かめていないこと」（ストア版 Claude からの 127.0.0.1 と token.txt・UAC と %ProgramData% の書き込みの制限・.bat の日本語・画面の拡大率）を 1 つずつ確かめて、PC.md と LOG.md を直す

iPhone の道具（[MCP.md](MCP.md)）の残り：
6. （済 0.1.7）文字で押す（iphone_tap_text）：同じ文字が複数あるとき、見えていて押せる方を選ぶ（マップの「出発」で外れた）
7. （済 0.1.7。かかっている・外れているの両方を実機で確かめた）ロックの判定：画面（真っ黒）ではなく iOS の状態で見て、外れていなければ「カオルに外してもらう」と返す
8. （済 0.1.7）ログを PC のファイルにも残す（3 万行からあふれて、前のことを確かめられなかった）
9. （済 2026-11-25）設定アプリの操作（名前で開く → 検索欄に打つ → スクロールして探す。見るだけで何も変えない）と、ショートカットを 1 つ作って URL で実行するところまで確かめる
10. （ショートカット DevKitRefresh を作り、URL で動いた。期限前の Refresh は何もしない（App ID の期限まで延ばせない。LOG 2026-11-25）。12 月 1 日 18:25 以降〜12 月 2 日 06:54 までに Refresh All をして、7 日に戻るか確かめる）SideStore に、ショートカットから入れ直し（Refresh All）を呼ぶ機能があるか見る（次の入れ直しの期限は 12 月 1 日ごろ）

安全の見直し（2026-11-25）：

11. （済 2026-11-25）PC に残っていた SideStore のログの写しは消した

遠隔で PC の電源を入れる（Wake-on-LAN）の下調べ（2026-11-25 カオルの頼み）：

12. （終わり 2026-11-25：今の PC では合図で起こせないと分かり、カオルが打ち切った。試しのものは片付けた。LOG 参照）PC の道具で、見るだけで調べる（何も変えない）：スリープの方式（`powercfg /a`。モダン スタンバイか S3 か）、LAN アダプター（USB-C の変換）の種類と起こす機能（`Get-NetAdapter`・`Get-NetAdapterPowerManagement`・`Get-NetAdapterAdvancedProperty` のマジックパケット）、起こせる機器（`powercfg /devicequery wake_armed`）、最近の起きた理由（イベントログの Power-Troubleshooter）。見込み：USB の LAN アダプターは電源を切った状態からは起こせない。スリープからなら見込みあり。結果と、変えるなら要る設定をまとめてカオルに出し、変えるのは決めてから

iPhone の中身を調べる道具（PC の道具の iPhone 版。2026-11-25 カオルの頼み）：

13. （済 2026-11-25。仮の位置・Safari のページ・DevKit ののぞき窓まで）今の iPhone の道具は画面の操作が中心。PC の道具のように、iPhone の中身を見る道具にする。先に、各窓口が iOS 27 で答えるかを見るだけで 1 つずつ試す（go-ios v1.3.2 に部品あり）：端末の情報と空き容量（lockdown・disk_usage）、アプリごとの大きさ（installationproxy）、クラッシュの記録（crashreport。ほかのアプリの分も）、プログラムごとの CPU・メモリ（instruments の sysmontap）、電池の状態（diagnostics の IORegistry）、構成プロファイル（mcinstall）。答えたものだけを道具にする（5〜7 個に抑える。見るだけで使える）。アプリの一覧・クラッシュの記録は個人の情報なので repo に写さない
    さらに、画面を毎回操作するのではなく、中から動かす分も同じ形で試す（カオルの頼み：PC の道具のように効率よく内部で動く）：アプリの起動・終了（instruments の processcontrol。WDA なし）、自分のアプリ・LiveContainer のファイルの読み書き（house_arrest。テストのデータ・設定）、写真などのフォルダ（afc）、クリップボードへの書き込み（pasteboard。打つ代わりに貼る）、URL・ショートカットの直接実行、開発用の仮の位置（simlocation）、言語・時刻の形式・タイムゾーンなどの一部の設定（lockdown。変える前にカオルに確かめる）、WDA なしの画面（instruments の screenshot）、Safari の中のページ（webinspector。設定でオンが要る）。ほかのアプリの細かい操作だけは画面（WDA。部品の名前で押す）が残る。自分のアプリには PC から呼べる受け口（のぞき窓）を入れる

残り（2026-11-25 時点）：

14. 「消してから打つ」：設定の検索欄は 0.1.16 で確かめた（2026-11-25。「Wi-Fi」→「一般」に入れ替わった）。残り：Flutter のアプリで「消してから打った」と返したのに実際は消えていなかった件（Wolow で見つけた。Wolow は消したので、次に Flutter のアプリを触るときに、打ったあとに欄の値を読んで確かめるよう直す）
15. PC の道具：画面の拡大率（DPI）が 100% でないときの座標と、画面・部品の操作（pc_ui_do・pc_keys）を、カオルの PC で確かめる
16. （済 2026-11-26）ブラウザの道具をカオルの PC で確かめた（pc-13：chrome://inspect のリモートデバッグ・Chrome の許可・ブラウザのスイッチ・速さ。PC.md の「ブラウザ」）。残り：ふつうのサイト（買い物・設定の画面）で、読む量（トークン）と往復の数を見る
17. 守りと窓の作り直し（2026-11-26〜27 に設計だけ。[ASK-DESIGN.md](ASK-DESIGN.md)）：作る前に、PC で確かめること 12 こ（ASK-DESIGN.md の最後。古い道 go.bat の転送を PC の中だけに直すことも含む）を記録を取って確かめる。作るのはカオルが「作って」と言ってから
18. 3 つの手の連動（2026-11-26 に設計だけ。[LINK-DESIGN.md](LINK-DESIGN.md)）：17 とまとめて作る。作る前に PC で確かめること 5 つ（LINK-DESIGN.md の最後）を記録を取って確かめる
19. iPhone を中から扱う（2026-11-27 に方針だけ。[LINK-DESIGN.md](LINK-DESIGN.md) の「iPhone を中から扱う」）：内部の窓口の棚卸し → 読むだけの窓口を試す → 読み方を比べる（Claude の分かり方を第一に）→ 中から動かす道を調べる → 地図を書いて道具に入れる
20. 操作研究（2026-11-28〜29 に構想だけ。[RESEARCH-DESIGN.md](RESEARCH-DESIGN.md)）：考えるのは AI・手を動かすのはプログラム。残りタスクのあと、8 番の RECIPE を第 1 歩に作る。最後の目標はブラウザ版のストリートファイター II でベガを倒す

