# PC の道具（tools/pc-mcp）

この md は道具の動かし方だけを書く。GitHub の Windows（windows-latest）で通しの試験をしたもの。カオルの PC でまだ確かめていないものは「（未確認）」と書く。

Claude が Windows の PC を、画面を撮って押す（computer use）のではなく、PowerShell・ファイル・Windows の UI オートメーションで直接見て・動かすための道具。1 回の呼び出しは 0.1 秒前後（最初だけ PowerShell の起動で数秒）。

## 作り

| 役 | ファイル | 何をするか |
| --- | --- | --- |
| 拡張機能 | `claude-hands-pc.mcpb`（中に `pc-mcp.exe`） | Claude デスクトップの中で動く。道具を並べて、中身は常駐役に頼むだけ |
| 常駐役 | `%USERPROFILE%\devkit\pc-agent\pc-agent.exe` | カオルのスイッチ（.bat）で起動・停止する。PowerShell を開きっぱなしにして実際の作業をする。PC の中（127.0.0.1:60210）だけで受け付け、合言葉（token.txt）がないと答えない |

常駐役を分けたわけ：ストア版の Claude から動くプログラムは、AppData への書き込みが別の場所に振り分けられる（iPhone の道具で確かめた）。常駐役はカオルがエクスプローラーから起動するので、振り分けを受けない。

## 入れ方（カオル・最初と、新しい版にするとき）

1. Releases の最新の `pc-N` から `claude-hands-pc.mcpb` と `pc-switch.zip` を落とす
2. `pc-switch.zip` を開いて、中の `pc-setup.bat` を実行する（常駐役を `devkit\pc-agent` に、スイッチの窓 `claude-hands-switch.exe` と LIVE の窓を `devkit\hands` に、予備の PowerShell の窓を `devkit\switch-app` に置き、デスクトップに窓のショートカット「Claude 操作スイッチ」を作る（.bat は zip の中の予備）。動いている常駐役・窓は止めてから入れ替える）
3. Claude デスクトップの「設定 → デスクトップアプリ → 拡張機能」に `claude-hands-pc.mcpb` をドラッグする
4. 道具ごとの「許可」の確認は、カオルが「常に許可」を選べば次から出ない

## スイッチと許可（カオルだけが切り替える）

| .bat | 状態 | Claude ができること |
| --- | --- | --- |
| PC-OFF | オフ | 何もできない（常駐役は数秒で終わる） |
| PC-VIEW | 見るだけ | 容量・不具合の記録・プロセス・アプリ・ファイルを読む・画面・窓・部品の一覧・読むだけの PowerShell |
| PC-OPERATE | 操作も可 | 上に加えて、PowerShell（何でも。ただし下の「許可が要るもの」は除く）・開く・部品を押す・打つ・ごみ箱 |
| PC-ALLOW | 許可（10 分） | 下の「許可が要るもの」も使える。管理者の確認（UAC の「はい」）を通る。スイッチを切り替えると取り消し |

- 使わない時間が 30 分続くとオフに戻り、常駐役が終わる
- 置き場所：スイッチ `%USERPROFILE%\devkit\pc-agent\mode.txt`、許可 `%ProgramData%\pc-agent\allow.txt`
- 許可のファイルは、管理者しか書けないフォルダにある。PC-ALLOW.bat が UAC を通ってから書く。Claude の PowerShell は普通の権限なので書けない（試験で確かめた）
- Claude 側の決まり：.bat・窓のボタンを押さない・mode.txt / allow.txt を書かない。オフなら「オンにしてください」、許可が要るなら「PC-ALLOW.bat（窓の「10 分 許可」）を押してください」と頼む

## スイッチの窓と「許可しますか？」（2026-11-27 から新しい窓）

- `%USERPROFILE%\devkit\hands\claude-hands-switch.exe`（Go ＋ Wails。tools/switch-window）。デスクトップの「Claude 操作スイッチ」から開く。見た目はモック（ASK-DESIGN.md の「見た目」）：黒地、OFF 赤・VIEW 灰・OPERATE 緑・ALLOW オレンジ
- **SWITCH**：iPhone・PC・WEB（ブラウザ）を OFF・VIEW・OPERATE で切り替える（mode.txt を書く。PC か WEB をオンにしたとき常駐役が動いていなければ起こす）。ALLOW の［10 MIN］は UAC を通って allow.txt を書く。許可中は［CANCEL］（PC の mode.txt を書き直して取り消す）。いちばん下は LIVE の帯（動いているとき：緑の点・入っているスイッチの札・作業の名前・最後の操作・［STOP／止める］。止まっているとき：IDLE・最後の動き・［ALL OFF／すべてオフ］。どちらも全部のスイッチを OFF）。右下に出て、開いている間は LIVE の窓が隠れる（2 つを 1 枚にまとめた形。案 B）。窓の高さは中身に合わせる（画面が高さを知らせる）。頼みの窓を出している間は題名が「claude-hands ASK」になり、LIVE は隠れない。［×］で隠す（通知領域に残る）、［—］で最小化、上のところをつかんで動かせる
- **通知領域**：丸の色がいちばん強いスイッチ（OFF 赤・VIEW 灰・OPERATE 緑・ALLOW オレンジ）。左で押すと SWITCH、右で押すとメニュー（開く・すべてオフ・終わる）。マウスを乗せると「iPhone：OFF / PC：VIEW / WEB：OFF」
- **頼みの窓（VIEW・OPERATE・ALLOW）**：画面の真ん中・いちばん前に出て、通知音を鳴らす。フォーカスは取らない（打っている途中のキーが入らない。キーでは答えられない。出てから 1 秒は押せない）。中身は WHAT（しようとしていること。道具が作る）・WHY（Claude の説明。今は作業の名前＝hands start の題名。無いと「説明がありません」）・IF ALLOWED（許可すると。窓の側が種類ごとに持つ）。［DENY／断る］［ALLOW／許可する］、残りの秒、ほかに待っている頼みの数
- 頼みのファイル：`devkit\requests\<種類>.txt` に JSON（kind・what・why・tool・from・at）。前の形（理由 1 行）も読む。返事は `<種類>.answer.json`（allow・deny・timeout）。**断る・時間切れだと、道具は待たずに「カオルが断りました。同じ操作をやり直さず、チャットで聞いて」と Claude に返す**。ALLOW を許可しても UAC で「いいえ」なら deny
- 記録：許可・断る・時間切れは `devkit\hands\asks.jsonl`（時刻・種類・WHAT・WHY・道具・返事）
- 予備：今までの PowerShell の窓（`devkit\switch-app\devkit-switch.vbs /old`）。新しい exe があると、vbs をふつうに開いても新しい方に任せて終わる。2 つが同時に頼みを取らないよう、pc-setup.bat は動いている PowerShell の窓を止める
- **WHAT（言い換えの表・pc-36 から）**：常駐役が止めた理由から、ふつうの言葉で作る（tools/pc-mcp/what.go）。命令の名前・ファイルの場所・URL・ページの題名はそのまま出さない。例：Invoke-WebRequest →「インターネットとやりとりする（落とす・送る）」、Login Data →「ブラウザのパスワード・クッキーのファイルを読む・変える」、HKLM: →「Windows 全体の設定（レジストリ）を読む・変える」、銀行のページ →「「銀行」にかかわりそうなページを操作する」。スイッチの頼みは道具ごとの言い方（「PC の窓の部品を押す・文字を入れる」など）。表に無いものは「よく分からない操作（道具の名前）」
- **WHY（Claude の説明・pc-36・mcp-22 から）**：どの道具にも引数 `why` がある（pc_status・iphone_status を除く）。窓が出るとき（スイッチが足りない・ALLOW が要る）だけ要る。**無い・100 字を超える・2 行以上・URL・記号が 3 つ以上**なら、道具は窓を出さずに「why を書いて呼び直して」と Claude に返す。窓には why を紫で大きく、今の作業の名前（hands start の題名）を小さく出す。記録 asks.jsonl にも why と作業の名前が残る
- Claude 側の決まり：窓が出そうな操作（スイッチがオフ・見るだけのときの操作・パスワードや外への通信など）には、はじめから why を付ける。why は「この許可で何をするか」を正直に書く（窓の WHAT と合っていなければ、カオルが断る目安になる）
- **頼みの署名（pc-37 から）**：道具（PC・iPhone）は頼みに合言葉で署名する（HMAC-SHA256。合言葉は `%ProgramData%\claude-hands\keys\request.key`。pc-setup.bat の守りの場所づくりで、管理者が 1 回だけ作る。カオルの権限では読むだけ。文字の守りで Claude の台本からは読ませない）。窓は合言葉があれば、署名の合わない頼み・書き換えた頼み・2 分より古い頼み・名前と中身の種類が違う頼みを捨て、asks.jsonl に「捨てた」と残し、道具には「署名が確かめられず捨てた」と返す。合言葉がまだ無いときは受け付けて、窓に「署名なし」と出す。pc_status に「頼みの署名：あり／まだ無い」
- **Claude の道具はカオルの窓を押さない（pc-37 から）**：pc_ui_do・pc_keys は、窓の名前が「claude-hands SWITCH／ASK／LIVE」「Claude 操作スイッチ」「Claude からの頼み」なら止める（許可があっても）。click_xy などの位置で押す操作は、その位置にある窓を調べて止める。computer use（Claude アプリの画面操作）は止められないので、Claude の決まりとして押さない

### 前の窓（2026-11-25〜27。予備として残す）

- `%USERPROFILE%\devkit\switch-app\devkit-switch.vbs` を開くと、iPhone・PC・ブラウザのスイッチを 1 つの窓で切り替えられる（「パスワード・セキュリティ・お金」の 10 分 許可は PC とブラウザで共通）（中身は .bat と同じ：mode.txt を書く。PC の許可は UAC を通って allow.txt を書く）。右下の通知領域に常駐し、窓を閉じても残る。右上の「—」で最小化（タスクバーへ）、ボタン以外のどこをつかんでも窓を動かせる（2026-11-26）。終わるのは通知領域の右クリック →「終わる」。デスクトップのショートカット「Claude 操作スイッチ」（アイコンは devkit-switch.ico）から開ける。もう動いているときは、2 つ目は起動せず、動いている方の窓を前に出す（名前つきの合図 Local\DevKitSwitchShow）
- 道具がスイッチ不足で止まるとき、道具は `%USERPROFILE%\devkit\requests\<種類>.txt`（iphone-view・iphone-operate・pc-view・pc-operate・browser-view・browser-operate・pc-allow。中身は理由 1 行）を置き、窓を `/tray` で起こす。窓が「○○にしますか？」を出し、カオルが「許可する」か「断る」を押す
- 道具は返事を待つ：60 秒（PC の許可は 90 秒）。許可されたらそのまま続ける。断られた・時間切れなら、今までどおり「スイッチが足りません」を返す
- 窓は 55 秒（許可は 80 秒）で自動的に断る。2 分より古い頼みは捨てる。既定のボタンは「断る」（Enter・Esc で断る）
- Claude 側の決まりは変わらない：mode.txt・allow.txt を書かない、窓のボタンを押さない。requests に置くのは道具だけ（Claude が手で置いてよいのは、カオルに見た目を見せる試しのときだけ）
- 許可の窓は、許可が要る命令（Remove-Item・HKLM のレジストリ。読むだけでも）で自動で出る。カオルの手間になるので、ごみ箱（pc_trash）・アプリの一覧（pc_apps）など許可の要らない道具で済むならそちらを使う。どうしても要るときは、先にカオルに一言伝える
- `DEVKIT_NO_ASK` があるとき（CI の試験）や USERPROFILE が無いときは頼まない

### 許可が要るもの（道具が止める）

| 種類 | 例 |
| --- | --- |
| パスワード・鍵・資格情報 | ブラウザのパスワード・クッキー・カード（Login Data・Web Data・Cookies など）、Windows の資格情報・証明書、.ssh・.aws・.git-credentials・.npmrc・gh の hosts.yml、iPhone のトンネルの鍵（devkit\go-ios）・iloader のサインイン、.kdbx・.pfx・.pem・.key・.env、パスワード管理アプリ |
| お金・本人確認 | 名前に bank・銀行・証券・wallet・給与・源泉・確定申告・e-Tax・マイナンバー・パスポート・免許証・password・secret などを含む場所。窓の題名に銀行・証券・PayPay・カード・サインイン・ログイン・アカウントなどを含む窓 |
| セキュリティ・Windows の設定 | cmdkey・certutil・netsh・Defender・ファイアウォール・BitLocker・ユーザー・レジストリ・タスクの予約・サービス・実行の決まり・アクセス権・管理者で開く（runas） |
| 戻せない・外に出る | 完全に消す（Remove-Item・.Delete()。ごみ箱は pc_trash でよい）・ごみ箱を空にする・ドライブの初期化・外への通信（Invoke-WebRequest など）・別の PowerShell や cmd・中身が後で決まる実行（Invoke-Expression・& $x） |
| Windows 本体を変える | C:\Windows・Program Files・ProgramData・ドライブの直下・スタートアップ |

- スイッチ・許可の置き場所は、許可があっても触らせない
- **箱（Sandboxie）の設定も、許可があっても触らせない**（pc-52〜）：SbieIni・SandMan・SbieCtrl・Sandboxie.ini の名前と、Start.exe の設定の窓を開く使い方（run_sbie_ctrl・open_agent）を止める。pc_open で SandMan を開く・SandMan の窓を押すのも止める。Start.exe で箱の中で動かす・中身を空にする（delete_sandbox_silent）は通す。本当の守りは Sandboxie の「設定のパスワード」（EditPassword）：設定を書けるのは Sandboxie のサービスだけで、パスワードが無いと管理者でも断る。パスワードはカオルだけが知る（iCloud パスワードの「Sandboxie 設定（PC）」）。pc-setup.bat の最後の box-check.ps1 が、管理者なしで書けないことを毎回確かめる（下の「箱」）
- 調べ方：PowerShell 自身に台本を読ませて（実行はしない）、呼ぶ命令・メソッド・書き出しを見る。別名（del・rm・iwr など）は元の名前に直して判断する
- 限り：台本の書き方をわざと工夫すれば、文字で探す守りは抜けられる。止めているのは「うっかり」。抜けられないのは、UAC を通す許可のファイルだけ。だから Claude 側の決まり（CLAUDE.md）と記録（audit.log）を組み合わせる

## 道具

| 道具 | スイッチ | 中身 |
| --- | --- | --- |
| pc_status | いつでも | 常駐役・スイッチ・許可・C の空き（100 GB・50 GB の決まり）・取り置き（件数・大きさ・新しい 3 つ）・毎日の点検（最後の復元ポイント）・OneDrive への最後の写し・最近の記録 8 行 |
| pc_disk | 見るだけ〜 | ドライブの空き。`path` の中を大きい順（`files` で 50 MB 以上のファイルも）。膨らみやすい場所の大きさ（path を省いたとき） |
| pc_problems | 見るだけ〜 | イベントログのエラー（`warnings` で警告も）、落ちた・固まったアプリの回数、エラー報告（WER）、クラッシュのダンプ、`reliability` で信頼性モニター。`app` で絞る |
| pc_processes | 見るだけ〜 | メモリの重い順（同じ名前はまとめる）か、`sort: cpu` で CPU（1 秒測る） |
| pc_apps | 見るだけ〜 | 入っているアプリ（`sort: size/date`、`store` でストアのアプリも）か、`kind: startup` で起動時に動くもの |
| pc_files | 見るだけ〜 | list（一覧）・read（中身。`head`・`tail`・`pattern` で行を絞る。UTF-8・Shift_JIS・UTF-16 を見分ける）・find（名前の形で探す。`sort: new/big`）・info |
| pc_screenshot | 見るだけ〜 | 画面全体か窓（`window`）。返事に「画像の座標 → 画面の座標」の換算が付く |
| pc_windows | 見るだけ〜 | 開いている窓（題名・プログラム・pid・位置。いま前の窓に印） |
| pc_ui | 見るだけ〜 | 窓の中の部品（種類・名前・id・値・位置）。文書の欄は中身の先頭も出る。`filter` で絞る |
| pc_powershell | 見るだけ〜 | 読むだけの台本（Get- / Test- / Measure- / Select- など）は本物の PC でそのまま。**何かを変える台本は既定で箱（ClaudeBox）の中**（pc-61〜。道 B。下の「Claude の命令は最初から箱の中で」）。`real: true` で本物を変える（道 C。許可が要るものを除く）。見るだけでは読む台本だけ |
| pc_open | 操作も可 | アプリ・ファイル・フォルダ・URL を開く |
| pc_ui_do | 操作も可 | 部品を名前か id で探して click・set_value・toggle・expand・select など。位置で押す click_xy（`from_image` で画像の座標のまま） |
| pc_keys | 操作も可 | 打つ（`text`、日本語は `paste: true` が確実）・特別なキー（`keys`：^s・%{F4}・{TAB} など）・`enter`。`window` を前に出せなければ打たない |
| pc_try | 操作も可 | 箱で試す（Claude の作業場。pc-57〜）：ClaudeBox の中でファイル（`path`）か PowerShell の台本（`script`）を動かす。返事に終わりのコード・画面の文字・増えた／変わった（前 → 後）／消えたファイル。続けて run できる。`action`：run（既定。`args`・`seconds` 既定 60 最大 110・`offer`・`lines`）・show・offer（TRY の窓に出す）・discard。本物に入れるのは TRY の窓（カオル）だけ（下の「箱で試す」） |
| hands | 見るだけ〜（undo・save は操作も可） | 作業のまとまり：start・done（DONE の報告）・log（TIMELINE）・undo（まとめて戻す）。下の「作業のまとまり」。**台本（RECIPE）**：recipes・recipe・save・run（下の「台本」） |
| pc_trash | 操作も可 | 消す（完全には消さない）：**取り置き**（30 日）に写して中身が合うのを確かめてから、元を消す（ごみ箱は通さない。2026-11-28 から。C を 2 回使わないため）。取り置きに入らない大きいもの・C の空きが少ないときは止まる → カオルに聞いてから `recycle: true`（ごみ箱だけ）。消す前にカオルに確かめる決まり（下の「取り置き」） |

## ブラウザ（2026-11-26 から。browser_ で始まる道具）

### Claude 用の Chrome（2026-11-27 から。pc-42〜。ASK-DESIGN.md の 4）

- ブラウザの道具は**既定で Claude 用の Chrome**（`profile=claude`）を使う。カオルのいつもの Chrome（ログインしたまま）は、**カオルに頼まれたときだけ** `profile=kaoru`
- Claude 用の Chrome：道具が自分で起動する（User Data は `%LOCALAPPDATA%\claude-hands\chrome-claude`。カオルの Chrome とは別で、ログイン・履歴・パスワードは入っていない）。`--remote-debugging-port=0`（PC の中 127.0.0.1 だけ）なので「許可しますか？」は出ない。ブラウザのスイッチがオフ・30 分使わないと閉じる
- **Claude 用の Chrome ではログインしない**：ログイン・お金・パスワードにかかわるページ（銀行・決済・サインインなど）は、ALLOW があっても開かない・動かさない（いつの間にかログインがたまるのを防ぐ）。要るときはカオルに聞いてから `profile=kaoru`
- Claude 用の Chrome のタブは全部 Claude のものなので、閉じてよい
- pc_status の「Chrome：つながっている（Claude 用の Chrome・カオルの Chrome）」で、どちらにつないでいるかが分かる
- 見た目：**ダーク＋紫**（カオルの Chrome とひと目で見分ける）。名前は「Claude」。翻訳の吹き出しは出さない。起動の前に Preferences の browser.theme.user_color2・color_variant2・color_scheme2 を入れる（Chrome 158 で確かめた名前）
- 出し方：**フォーカスを取らずに起動し、ほかの窓の後ろに置く**（窓なしで起動 --no-startup-window → つないでから Target.createTarget の newWindow・background で窓を作る → SW_SHOWNOACTIVATE と HWND_BOTTOM。タブも background で開く。ふつうに起動すると、最小化の指定でも Chrome は前を取った）。見たいときはタスクバーから開く。閉じるときは Browser.close（「ページを復元しますか？」は出さない）

カオルのいつもの Chrome（ログイン済み）を、画面を撮らずに**文字と番号**で読んで、**何手もまとめて**動かす。1 回の呼び出しは PC の中で 0.01〜0.1 秒（手元の試験）。往復が減るので、Claude in Chrome や内蔵ブラウザより速い。

### つなぎ方（カオル・最初に 1 回）

1. Chrome で `chrome://inspect/#remote-debugging` を開き、リモートデバッグを許可する（オン）。Chrome が User Data の `DevToolsActivePort` に番号を書く
2. 道具を初めて使うと、Chrome が「リモートデバッグを許可しますか？」を出す。カオルが「許可」を押す（押さなければ 75 秒で止まる）
3. つないでいる間、Chrome の上に「自動テスト ソフトウェアによって制御されています」の帯が出る。ブラウザのスイッチをオフにする・30 分使わないと切れる。次に使うとき、また「許可」が出る
- カオルの PC で確かめた（2026-11-26）：User Data の DevToolsActivePort で見つかり、Chrome の「許可」1 回でつながった。ポーカーのゲームで 1 手 1.7〜4 秒（道具の往復は 0.1 秒ほど。残りはゲームの待ち）
- つなぐのは PC の中（127.0.0.1）だけ。Chrome 144 からの公式の仕組み（Chrome DevTools MCP の autoConnect と同じ）。2026-11-26 の PC の Chrome は 158

### スイッチと許可（ブラウザは PC と別のスイッチ・許可は共通。2026-11-26 から）

スイッチの窓の「ブラウザ」の段で切り替える（置き場所 `%USERPROFILE%\devkit\browser\mode.txt`。書くのはカオルの窓だけ）。PC がオフでもブラウザだけ使える（常駐役はどちらかがオンなら動き、両方オフで終わる）。ブラウザをオフにする・30 分使わないと、Chrome とのつながりをすぐ切る（上の帯も消える）。足りないときは道具が「ブラウザ（Chrome）を〜にしますか？」の窓を出して 60 秒待つ（頼みの種類 browser-view・browser-operate）。PC かブラウザのスイッチを切り替えると、許可（10 分）は取り消し。

| ブラウザのスイッチ | できること |
| --- | --- |
| 見るだけ | browser_tabs（一覧）・browser_read・browser_screenshot |
| 操作も可 | 上に加えて browser_do・browser_js・タブを開く／前に出す／Claude が開いたタブを閉じる |
| 許可（PC-ALLOW・10 分） | 銀行・証券・決済・ログイン・パスワード・2 段階認証などのページの操作、クッキー・保存領域・パスワードにかかわる JavaScript、file:// のページ、カオルが開いたタブを閉じる |

- **許可があっても打たない**：パスワード欄・カード番号・セキュリティコード・ワンタイムコードの欄（type="password"・autocomplete の cc- など）。その欄では Tab・Enter・Esc・矢印以外のキー（貼り付けの Ctrl+V も）を押さない。値は読んでも「（入力あり・伏せる）」になる
- **行かない**：chrome://・chrome-extension://・devtools:// などの設定・拡張機能のページ（パスワードの設定を含む）
- 決まりは文字で見る守り（「うっかり」止め）。Claude 側の決まり（CLAUDE.md：パスワード・2 段階認証のコードはカオル、お金は先に確認）と組み合わせる

### 道具

| 道具 | スイッチ | 中身 |
| --- | --- | --- |
| browser_tabs | 見るだけ〜 | タブの一覧（id・★＝道具がいま使うタブ・＋＝Claude が開いたタブ）。`action`：new（url）・select・close |
| browser_read | 見るだけ〜 | 押せる部品の一覧（`[番号] 役「名前」 = 値`・画面の外・押せない・パスワード欄の印）。`mode`：elements（既定）・text・all。`filter` で絞る。canvas・iframe があれば数を出す |
| browser_do | 操作も可 | `steps` を順に：click（番号・`text=…`・`css=…`）・double・click_xy・type（ref・text・clear・enter）・select・key（Enter・Ctrl+A など）・scroll・wait・wait_for（文字か css）・goto・back・js。**分かれ道・繰り返し・確かめ**（pc-65 予定〜）：`if {text か css, wait, then:[手], else:[手]}`（いまページにあるか）・`repeat {times（50 まで）, until:{text か css}, steps:[手]}`（until にならなければ止まる）・`check {text か css, absent}`（あるはず・ないはずが違えば止まる。台本でページの形が変わったのに気づく）。中の手も同じ守り（E・許可）を通る。繰り返しを含めて 300 手まで。終わったら部品の一覧を返す（`read`：elements・text・all・none）。`front: true` でタブを前に出す（見えていないと止まるゲームなど） |
| browser_js | 操作も可 | ページの中で JavaScript（コンソールと同じ書き方・await 可・最後の値を返す） |
| browser_screenshot | 見るだけ〜 | 見えている所か部品 1 つ（`target`）を JPEG で。画像の座標 → click_xy の座標の換算が付く。文字で読めないときだけ |

- `tab` は一覧の id の頭（4 文字以上）か、題名・URL の一部。省くと最後に使ったタブ（無ければ見えているタブ）。**並び順の番号は使えない**（カオルがタブを開け閉めすると変わり、2026-11-26 に別のタブへ移ろうとしてしまった）
- 番号は次に読むまで有効。ページが変わったら読み直す。番号の代わりに `text=決める` のように名前で押してもよい
- 押す・打つは本物のマウス・キーの合図（CDP の Input）なので、canvas のゲームも押せる（isTrusted=true を試験で確かめた）

### Claude 側の決まり（ブラウザ）

- **カオルのタブ（＋が付いていないタブ）は、頼まれたときだけ読む・動かす**。ふだんの作業は browser_tabs の new で自分のタブを開いてそこでやり、終わったら閉じる。カオルのタブにはカオルが見ているもの（個人的なページを含む）があるので、題名や中身を返事・repo・記録に写さない
- 一覧（browser_tabs）にはカオルのタブの題名・URL も出る。必要な行（自分のタブ）だけを見て、ほかには触れない
- 常駐役の記録（audit.log）には、道具の返事の 1 行目（タブの題名など）が残る。PC の中だけの記録なので repo には写さない
- ダウンロードは、頼まれたものだけ。ファイルはダウンロードのフォルダに入るので、使ったら片付ける（消す前に確認）
- ダウンロードは browser_do の goto でファイルの URL へ行く（GitHub の Releases の一覧の a を押す方法は、一覧があとから読み込まれるので押せないことがある）。goto は「net::ERR_ABORTED」で止まったと返すが、ダウンロードは始まっている。pc_powershell でダウンロードのフォルダを見て確かめる（1 つの goto ごとに 1 回の browser_do に分ける）

### canvas（絵で描かれた）ページ・ゲーム

- 部品の一覧に出ない。まず browser_js でページの中の作り（どのライブラリか：Phaser・PixiJS など）を見て、**画面に出ている文字と絵だけ**を読む。相手の伏せたカードなど、画面に出ていないものは読めても使わない
- Phaser 3 の例（2026-11-26 のポーカー）：`Phaser.Scenes.SceneManager.prototype.update` を包んで game を拾い、見えているシーンの Text と、カードの絵の番号（frame）を読む。押すのは canvas への pointerdown／pointerup（ページの中から送る）か、browser_do の click_xy
- browser_js は「コンソールと同じ書き方」なので、一番上の `const`・`let` は次の呼び出しまで残り、同じ名前を 2 回書くとエラーになる。続けて使う関数は `window.名前 = …` に置く

### 速く使うコツ

- 次の手が読めるときは 1 回の browser_do にまとめる（開く → 打つ → 押す → 待つ → 読む）
- 読む量を絞る（`filter`・`max`）。画像は最後
- 毎回同じ手順なら、browser_js に台本をまとめるか、PC に置くプログラムにする

## 作業のまとまり（hands。2026-11-27 から）

- **TIMELINE**：PC・ブラウザ・iPhone の道具の呼び出しを、`%USERPROFILE%\devkit\hands\timeline\日付.jsonl` に 1 本で残す（時刻・どの手か・道具・引数の頭・結果・作業の番号。pc_trash は取り置きの場所も）。iPhone の拡張機能も同じ所に書く
- **Claude の決まり**：PC やファイルを変える作業は、始める前に `hands action=start title=…`（カオルが読んで分かる題名）、終わったら `hands action=done`。done の報告（DONE）は Claude の説明ではなく記録から作る：変えたファイル（Git）・消したもの（取り置き）・そのほかの操作の数。報告は `devkit\hands\done\<番号>.txt` にも残る
- **まとめて戻す**：`hands action=undo task=<番号>`（操作も可）。その作業で Claude が変えた文字のファイルは始めの中身へ、作ったファイルは取り置きへ、消したものは取り置きから元の場所へ写す。あとで変わっているもの・元の場所に別のものがあるものは戻さずに知らせる。決まったフォルダの外の変更・送ったもの・設定は対象外
- **Git**：記録するフォルダは `devkit\hands\git-folders.txt`（ユーザーフォルダからの場所。既定 `devkit/setup`）。記録の置き場所は `%ProgramData%\claude-hands\git`（pc-setup.bat が作る。GitHub には上げない）。画像・zip・exe・ログ・plist などは記録しない
- `hands action=log` で、今の作業・終わった作業（新しい 5 つ）・最近の TIMELINE

## 作業の道 A〜E（2026-11-27 から。pc-38・mcp-24）

道具が呼び出しごとに道を決め、TIMELINE（lane）と DONE の報告（「道：A 読む 5・C 変更（控えあり）1・D 小さな変更 3・E 止めた 1」）に残す。設計は ASK-DESIGN.md の 2。

| 道 | 何が入るか | 道具がすること |
| --- | --- | --- |
| A 読む | 見るだけの道具、読むだけの PowerShell | そのまま |
| B 箱で試す | （箱は 5 番で作る。それまでは C） | |
| C 変更（控えあり） | 何かを変える PowerShell | 先に決まったフォルダ（git-folders.txt）を Git に控える → 本物でやる → 確かめる：画面に出たエラー（FullyQualifiedErrorId）・台本が止まった／時間切れ、Windows のエラーの記録（Application・System のエラー以上）がこの間に増えたか、決まったフォルダの変更。**エラーが出て決まったフォルダが変わっていたら、自動で控えに戻す**（作ったファイルは取り置きへ）。-ErrorAction SilentlyContinue で黙らせたエラーは数えない。返事の最後に「C（変更）の確かめ」 |
| D 小さな変更 | ごみ箱へ（取り置き）・開く・窓の部品を押す・打つ・ページの操作・タブ・iPhone の操作 | そのまま（記録と取り置き） |
| E 止めた | 戻せない操作の最後の一押し | Claude は押さずに止まり、「E（戻せない操作の最後の一押し）」と返す。押すのはカオル |

- E の見分け方：ボタンの名前（30 字まで）に「送信・送る・購入・注文する・支払う・決済・ログイン・サインイン・投稿・公開する・ツイート・send・submit・buy・purchase・place order・pay・checkout・log in・sign in・post・publish・tweet」がある（lane.go の finalRe）。pc_ui_do（名前で・位置で）、browser_do（番号・文字・位置で押す）、iphone_tap_text で見る
- Enter：ブラウザ・チャット・メール・Claude の窓（chrome・edge・LINE・Slack・Teams・Discord・Outlook など）では、pc_keys で Enter（送信に当たるキー・改行入りの文字）を打たない。ブラウザの中では、チャット・文章の欄（textarea・contenteditable）か、「送信」などのボタンがある入力欄では Enter を押さない（検索の欄などは押す）
- E で止まったら、Claude はチャットで「〜を押してください」とカオルに伝える。ALLOW があっても E は押さない

## LIVE の窓（2026-11-27 から。pc-26）

- 形（pc-33〜）：幅 340・高さ 84。1 行目に「LIVE」と、入っているスイッチの色の札（VIEW 灰・OPERATE 緑）。2 行目に作業の名前、3 行目に最後の操作。右に［STOP］。スイッチの窓（claude-hands SWITCH）が見えている間は隠れる（DONE は閉じるまで取っておく）

- `%USERPROFILE%\devkit\hands\claude-hands-live.exe`（Go ＋ Wails。スタートアップから起動。1 つだけ動く）。Claude が道具を使っている間だけ右下に出る。フォーカスを取らない（全画面の動画の上でも）
- 出す中身は記録から（スイッチ・task.json・TIMELINE の最後）。作業の題名は `hands action=start` の title がそのまま出るので、カオルが読んで分かる言葉にする
- ［STOP／止める］はカオルだけが押す（全部のスイッチをオフ）。Claude は LIVE の窓を押さない・閉じない
- **STOP で、動いている処理も止まる**（pc-64〜。それまではスイッチがオフになっても、動いている台本は最大 10 分終わるまで動いた）：常駐役の見回り（2 秒ごと）が PC のオフを見つけると、動いている PC の道具を切る。PowerShell の台本は途中で止まり（常駐の PowerShell を起動し直す。台本の中で起動した別のプログラムは動いたままのことがある）、箱の中で動いているものも止める（箱の中身と試しの記録は残る）。ブラウザの道具はブラウザのスイッチで同じ
  - **止めないもの**（控え・戻しにかかわる短い処理。途中で切ると、元が消えたのに取り置きの記録が返らない・半分だけ入る、になる）：pc_trash・hands（まとめて戻す）・TRY の窓の［本物に入れる］［捨てる］。本物で変える台本（道 C）を止めたときも、後の確かめ（エラーなら決まったフォルダを控えに戻す）は最後までやる
- **Claude が画面を撮る（pc_screenshot）・位置で押す（pc_ui_do の click_xy など）間は、LIVE の窓を一瞬隠す**（pc-65 予定〜。前に出さず・フォーカスを取らずに戻す）。位置で押すときは、隠したあとの窓で「押してよい窓か」を確かめる（LIVE の下にあるものを押す。スイッチ・許可の窓は今までどおり押さない）
- **DONE の報告に「箱で試したこと」も出る**（pc-65 予定〜）：作業の間に動きのあった試し（ClaudeBox）ごとに、動かした回数・増えた／変わった／消えた数・どうなったか（まだ試している・TRY の窓で返事待ち・本物に入れた・捨てた）。試しの控えは `devkit\hands\try\history`（新しい 100 件）
- **DONE の窓**：`hands action=done` のあと、右下に報告が出る（3 分か［閉じる］で消える）。［まとめて戻す］はカオルが 2 回押す（Claude は押さない）。押すと常駐役に `hands undo` が届く（PC が「操作も可」のときだけ）
- LIVE の exe だけを新しくするとき：Releases の pc-switch.zip から claude-hands-live.exe を取り、動いている claude-hands-live を止めて `devkit\hands` に写し、起動し直す（pc-setup.bat は要らない）

## 台本（RECIPE。pc-65 予定〜）

- くり返す作業は台本にして、`hands action=run` の 1 回の呼び出しで最後まで流す。Claude は途中で考えないので速く、トークンも少ない（LINK-DESIGN.md の 3・RESEARCH-DESIGN.md）
- 保存：`hands action=save name=… about=（何をする台本か 1 行）params={言葉: 説明} steps=[{tool, args, note, show}, …]`。手の中の `{{言葉}}` を run の `values` で入れ替える。40 手まで。直して保存すると版が上がり、前の版は `devkit\hands\recipes\history` に残る
- 流す：`hands action=run name=… values={…}`。返事は手ごとの結果と秒数、最後の手（と show: true の手）の返事
- 一覧：`hands action=recipes`（流した回数・うまくいった回数・最後に止まった所）。中身：`hands action=recipe name=…`
- 守り（台本を守りの抜け道にしない）：
  - 手は 1 つずつ、ふつうの呼び出しと同じ道（スイッチ・道 A〜E・許可・E の止め）を通る。TIMELINE にも 1 手ずつ残る
  - 許可（ALLOW）は使わない：許可が出ている間は流さない。許可が要る手に当たったら止まる
  - 使えない手：hands（入れ子）・pc_trash（消す）・本物で変える台本（pc_powershell の real）・画面を撮る道具。pc_try は run・show だけ（変える台本は箱の中で動く）
  - 失敗した手で止まる（前の手は済んでいる）。E（最後の一押し）もそこで止まる。止まった所は台本に残る
  - 同時に 2 つは流さない。STOP で途中でも止まる。1 回 120 秒まで（リモートの道具は 140 秒で切れるため）
  - 置き場所は PC の中（`devkit\hands\recipes`。個人の情報が入りうるので repo には入れない）
- まだ無いもの（あとで）：iPhone の手・前の手の結果を次の手に渡す・WATCH（見張り）・記録から自動で台本を作る・似た台本を道具が自動で添える（操作研究）

## 取り置き（2026-11-27 から）

- 場所：`%ProgramData%\claude-hands\holding\<日時>\`（中に元の名前のファイル・フォルダと「元の場所.txt」）。作るのは pc-setup.bat（UAC。`hands-protect.ps1`）
- **Windows の権限で守る**：カオルの普通の権限（＝常駐役・Claude の PowerShell）は「入れる」ことだけできる。消す・書き換える・名前を変える・取り出す（移す）はできない。文字の守りでも、この場所は読むのも止める
- **戻すのはカオル**：エクスプローラーで取り置きを開き、元の場所へコピーする（UAC の「続行」が出る）。Claude は場所と「元の場所」を伝えるだけ（pc_status に新しい 3 つが出る）
- 上限 10 GB（C の空きが 100 GB を切ったら 5 GB、50 GB を切ったら止める）。入らないものは pc_trash が止まるので、カオルに聞いてから `recycle: true`
- 30 日たったものは、毎日 12:00 の点検（予定のタスク `\claude-hands\RestorePoint`。SYSTEM。`%ProgramData%\claude-hands\bin\hands-daily.ps1`）が消す。同じ点検で復元ポイントを作り、結果を `status\daily.txt` に書く（pc_status に出る）
- OneDrive のクラウドだけのファイルを消すと、写すために一度落とす
- 写し方：フォルダを作って、ファイルの中身だけを 1 つずつ写す（[IO.File]::Copy は「ネットから来た」印などの付属データも写そうとして拒否される。印は「元の場所.txt」に ZoneId とサイトの名前だけ書く。URL には一時的な鍵が入るので書かない）。時刻も付けられないので、元の更新日時は「元の場所.txt」へ（robocopy は作ったフォルダの属性を書こうとして拒否され、しかも成功と返すので使わない）。つなぎ目（ジャンクション）の先は写さない
- 確かめたこと（2026-11-27・カオルの PC）：Claude の権限では、取り置きのファイルを消す・書き換える・付け足す・名前を変える・取り出す、フォルダごと消す・名前を変える、権限・持ち主を書き換える、取り置きごと消す、の 10 通りが全部「アクセス拒否」。10 分 許可があっても同じ（許可は文字の守りを外すだけ）
- 写すのに失敗したら、元は消さない。途中でできた空の日時フォルダは消せないまま残る（30 日で点検が消す）
- **ごみ箱は通さない**（2026-11-28。pc-49 から）：取り置きに写して大きさが合ったら、元はその場で消す（取り置きとごみ箱で C を 2 回使わないため）。元を消せなかったときだけごみ箱へ送る。ごみ箱はカオルが自分で消したものだけになり、ストレージ センサーが 30 日で空にする。`recycle: true`（取り置きなし）は今までどおりごみ箱へ

## 箱（Sandboxie-Plus。2026-11-28 から。ASK-DESIGN.md の 2 の「箱」）

- Sandboxie-Plus 1.18.6（`C:\Program Files\Sandboxie-Plus`）。設定は `C:\Windows\Sandboxie.ini`（カオルの普通の権限では読むだけ。書くのは Sandboxie のサービス SbieSvc だけ）。箱の中身は `C:\Sandbox\<ユーザー>\<箱>\`
- Claude 用の箱は **ClaudeBox**（pc-setup.bat が毎回作り直す。中身のファイルは残る）：ネットは中継だけ（pc-59〜。下の「箱のネット」。前は ClosedFilePath=InternetAccessDevices で全部切っていた）・管理者なし（DropAdminRights）・ネットワークのファイル共有なし（BlockNetworkFiles）・見せない場所＝守りの場所（%ProgramData%\claude-hands・pc-agent）・devkit\pc-agent・iphone-mcp・hands・backup・browser・go-ios・.ssh・カオルの Chrome と Edge の User Data・OneDrive。「すぐ取り出す（RecoverFolder）」は入れない（本物に入れるのは TRY の窓）
- **設定のパスワード（EditPassword。2026-11-28 カオルが入れた）**：設定を書けるのは Sandboxie のサービスだけ（Sandboxie.ini はカオルの普通の権限では読むだけ）。サービスは、パスワードが無い頼みを管理者でも断る（Sandboxie のソースの SbieIniServer::IsCallerAuthorized）。パスワードは iCloud パスワードの「Sandboxie 設定（PC）」にだけあり、Claude は知らない
  - **EditAdminOnly=y だけでは効かなかった**：カオルのアカウントは管理者のグループに入っていて、Sandboxie は UAC の「はい」の前（制限つきの権限）でも管理者として扱う（TokenIsAdmin が TokenElevationTypeLimited を管理者とみなす）。pc-52 の box-check が「書けてしまった」で見つけた。EditAdminOnly=y は入れたまま（害は無い）
  - SbieIni は断られても何も言わずに 0 を返す。書いたら必ず読み直して確かめる。UserSettings_（SandMan の見た目の設定）はパスワードなしで書ける
  - pc-setup.bat（hands-protect.ps1）は、箱の設定が決まりどおりなら何も書かない（パスワードも聞かない）。違うときだけ、黒い窓でパスワードを 1 回聞いて書き直す
  - カオルが SandMan で設定を変えるときもパスワードが要る（全体設定 → 詳細設定 → Sandboxie.ini タブの「設定の保護」で変える・外す）
  - **パスワードをなくしたとき**：管理者で（UAC の「はい」）メモ帳を開き、C:\Windows\Sandboxie.ini の [GlobalSettings] の EditPassword= の行を消して保存 → SandMan のオプション →［設定を再読み込み］（未検証。Sandboxie の手で ini を直す使い方どおりなら戻る）。Claude は UAC を押せないので、この戻し方は使えない
- Claude がするのは：箱の中で動かす（`Start.exe /box:ClaudeBox /wait …`）・中身を空にする（`Start.exe /box:ClaudeBox delete_sandbox_silent`）・箱の中身のファイルを読む。設定を読む・変えるのは道具が止める
- Claude 用の Chrome は箱に入れない（2026-11-28 カオル。今の別の Chrome のまま）

### 箱で試す（B の道・TRY の窓。pc-55〜。2026-11-28 カオルの PC で確かめた）

**箱のネット：許可したサイトだけ（次の段 1。pc-59〜）**：
- 箱の設定（pc-setup.bat が入れる・設定のパスワードが要る）：`NetworkAccess=*,Block` と `NetworkAccess=*,Allow;Address=127.0.0.1;Port=60230`、全体に `NetworkEnableWFP=y`（Windows の中核 WFP で強制。Sandboxie のソースで確かめた：有料の証明書は要らない・設定を読み直せば効く）。前の `ClosedFilePath=InternetAccessDevices`（ネットを全部切る）はやめた（それだと 127.0.0.1 の中継にもつながらないため）
- 中継（常駐役の中。boxnet.go）：127.0.0.1:60230。HTTPS（CONNECT・443 番）で、一覧のサイト（とその下の名前）だけ通す。IP の数字では通さない。常駐役が動いている間だけ（スイッチがオフなら箱のネットも無い）。箱から常駐役の口（60210）など、ほかの 127.0.0.1 にはつながらない
- 一覧：`%ProgramData%\claude-hands\net-allow.txt`（pc-setup.bat が無いときだけ作る。カオルの権限では読むだけ＝Claude は足せない。足すときはカオルが管理者のメモ帳で直す）。最初の中身：github.com・githubusercontent.com・pypi.org・pythonhosted.org・npmjs.org・proxy.golang.org・sum.golang.org・powershellgallery.com（2026-11-28 カオルが了承）。一覧が無い・読めないときは何も通さない
- 箱の中の設定（包みの台本 wrap.ps1 が毎回入れる。箱の中だけ）：HTTP(S)_PROXY・PowerShell の既定の中継・Internet Settings の中継
- 記録：`devkit\hands\try\net.log`（通した・止めた）。pc_try の返事に「この回に中継が通したサイト・止めたもの」が出る
- pc-setup.bat の最後（box-check.ps1）が、箱から直接 example.com・1.1.1.1 につながらないことを毎回確かめる
- **画面を持つ道具**（2026-11-28 に試しの HTA で確かめた）：pc_try の返事に「箱の中の窓」（題名に [#]）が出る。文書（.hta・.html・.xlsx など）は包みの台本が窓を開くだけにする（PowerShell は文書を中継の形で動かせない。.hta は mshta.exe、.txt・.log・.csv は notepad.exe を名指し）
  - **pc_ui は箱の中の窓の中身が見えない**：外から読めたのは題名の帯と最小化・閉じるだけ（HTML の入力欄・ボタンは出ない）。→ **pc_screenshot で見て、pc_ui_do の click_xy（from_image）で押す**
  - **打つのは pc_keys の text（キーを打つ）で届いた。paste（クリップボードから貼る）は箱の中に届かなかった**（日本語を打つときは要注意）
  - 確かめた流れ：入力欄を押す → kaoru と打つ →［作る］を押す → 画面に「できました：kaoru」・箱の中に gui-result.txt（名前：kaoru）。HTA のまちがい（ボタンの言葉の指定が無く「MakeIt は定義されていません」）も画面で見つけて直した
- 包みの台本は、進み具合の表示を切る（$ProgressPreference。箱の中の見えない窓に書けず Invoke-WebRequest が「コンソール出力バッファー…アクセスが拒否」で失敗した）

**Claude の命令は最初から箱の中で（pc-61〜。2026-11-28 カオルが決めた B）**：
- pc_powershell の台本は、道具が「読むだけか・何かを変えるか」を見分ける（「見るだけ」のスイッチと同じ見分け方）
  - 読むだけ（Get- / Test- / Measure- / Select- など）：今までどおり本物の PC で（道 A）
  - 何かを変える（書く・消す・入れる・外への通信・別の PowerShell など）：**既定で箱の中**（道 B）。中身は pc_try の作業場と同じ（同じ箱で続けて試せる・返事に終わりのコード・画面の文字・増えた／変わったファイル・本物に入れたいものは `pc_try action=offer` でカオルの TRY の窓へ）。TRY の窓が返事待ちのときは動かさない
  - `real: true`：本物の PC で変える（道 C）。頼まれた作業で本物を変える必要があるときだけ。今までどおり許可（ALLOW）・控え（Git）・確かめ（エラー・Windows のエラーの記録）を通る。**控え（Git の記録）が取れなければ、本物では何も変えずに止める**（pc-64〜。それまでは「取れなかった」と書いて進んでいた。控えが守るのは決まったフォルダ（devkit\setup など）だけで、ほかの場所は毎日の復元ポイントと OneDrive の写しが守り）
- 箱の中では、消す・レジストリ・外への通信（許可したサイトだけ）・別の PowerShell などは本物に届かないので許可は要らない。**パスワード・鍵・証明書にかかわるもの**（cmdkey などの資格情報の命令・証明書・Protect／Unprotect・鍵などの場所）は、箱からでも本物の中身を読めてしまう場所があるので**箱でも許可が要る**（pc_try の script も同じ決まり）。スイッチ・許可・取り置きの置き場所・箱の設定・箱の中身（C:\Sandbox）は箱からでも触れない
- 待つのは 110 秒まで（過ぎても箱の中では動いたまま。`pc_try action=show`）
- 箱が入っていない PC では変える台本は動かさない（本物で黙ってやらない）。そのときは real: true
- **箱から見えない場所（pc-64〜。一覧は `switch/box-closed.ps1`。pc-setup が書き、box-check が確かめる）**：守りの場所・道具の置き場所・.ssh・ブラウザ（Chrome・Edge・Brave・Firefox）のデータ・OneDrive に加えて、Windows の資格情報（Credentials・Vault）・DPAPI の鍵（Protect）・証明書の鍵（Crypto・個人の証明書）・GitHub CLI・gcloud・.aws・.azure・.kube・.gnupg・.docker・.git-credentials・.netrc・.npmrc・.pypirc・Bitwarden・1Password
  - 確かめ方（box-check.ps1。pc-setup の最後）：本物にある場所だけを、箱の中から「開けるか」だけ試す（中身は読まない・記録しない。返事は名前だけ）。Windows の資格情報の一覧も、箱の中から見えるかを数だけで比べる
  - 箱の中で困ること：GitHub への push・gcloud・資格情報を使うアプリは箱では動かない（もともと本物で許可を取ってやる作業）
  - **残るリスク**（2026-11-28。カオルと決めた。個人で使うので、全面的な分け方（Claude 専用の Windows ユーザー）や有料の Sandboxie の Privacy Mode（支援者の証明書が要る）までは今はしない）：
    1. 知っている場所を並べるやり方なので、**あとで入れたソフトの鍵の置き場所は漏れる**。鍵・トークンを置くソフトを入れたら box-closed.ps1 に足して pc-setup をやり直す
    2. ふつうのファイル（書類・プロジェクトの中の .env やパスワードを書いたメモなど）は箱から読める（作業場なので）。台本の文字に出た鍵などの場所は許可で止めるが、組み立てた名前・pc_try path で動かすファイルの中身は見えない
    3. 箱は許可したサイト（GitHub・PyPI など）にはつながるので、箱が読めたものを外へ出す道は残る
    4. レジストリ（HKCU）に鍵を置くアプリのものは箱から読める（Sandboxie のレジストリの見せない設定はまだ使っていない）

**Claude の作業場（pc-57〜）**：
- `pc_try path=…`（どこのファイルでもよい。鍵などの場所のものは止める）か `script=…`（PowerShell の台本）。devkit\downloads のものは Defender で調べてから。台本の中に devkit\downloads の動くものが書いてあれば、それも調べる
- 動かし方：作業用のフォルダ `%USERPROFILE%\devkit\boxwork`（箱から見える。包みの台本 wrap.ps1・Claude の台本 run-N.ps1）に置き、`Start.exe /box:ClaudeBox /wait powershell -File wrap.ps1 <対象> <out> <code>`。包みが画面の文字とエラーを UTF-8 で run-N.out に、終わりのコードを run-N.code に書く（箱の中にだけできる。Claude は箱の中を読む）。Start.exe /wait の終わりのコードも対象のものと同じ（2026-11-28 に記録で確かめた：bat の exit 3・ps1 の exit 5 がそのまま返った。bat の日本語も読めた。箱 1 回の立ち上がりは約 1 秒）
- 状態：working（Claude が試している。窓は出ない。続けて run すると箱はそのまま）→ offer で waiting（TRY の窓）→ applied／discarded。devkit\downloads のファイルは既定ですぐ waiting（`offer:false` で続けて試せる）
- 文字のファイル（256 KB まで・UTF-8／UTF-16／Shift_JIS）は、変わったものに「- 前・+ 後」の行、増えたものに先頭の行を付ける
- 捨てる・本物に入れるのあとで、作業用のフォルダは消す（道具の作業用。カオルのファイルではない）
- **箱の中ではストアのアプリが動かない**（2026-11-28 の記録）：カオルの PC は .txt・.ps1 の既定のアプリがストアの新しいメモ帳。既定のアプリで開くと Start.exe が止まったまま残り、窓は出ない（カオルの［見る］［箱の中で開く］が開かなかった）。昔からの `C:\Windows\System32\notepad.exe` を名指しすると箱の中で開く（窓の題名に [#]）。→ 窓は既定のアプリに頼らず：文字のファイルは notepad.exe、.ps1 は powershell -NoExit、.bat／.cmd は cmd /k、.exe はそのまま、.msi は msiexec /i。既定がストアのアプリのものは「箱では開けない」と出す（pc-58）
- TRY の窓：一覧の行ごとに［VIEW 見る］（そのファイルを箱の中で開く。箱の中では本物の場所が箱の中身に見える）・変わった行（+ 青緑・- 赤）・「Claude が箱で N 回試した（最後の回は成功）」。台本の試しのときは［箱の中で開く］を出さない

- **devkit\downloads のもの（Claude が落としたもの。印が無くてもネットから来た扱い）の動くもの**（exe・msi・bat・ps1・vbs・js・lnk・マクロ入りの文書など）は、**箱の外では動かさない**。道具の守りが pc_powershell（Start-Process・Invoke-Item・msiexec・cmd などで動かす）と pc_open を止めて「pc_try を使って」と返す。Defender の検査（MpCmdRun）・zip を開く・印を読むのは通す。Start.exe /box:ClaudeBox で箱の中で動かすのも通す
- **pc_try**（Claude）：Defender で検査（見つかったら止める。検査できなければ動かさない）→ 箱が残っていれば空にしてから → ClaudeBox の中で動かして seconds 秒待つ（過ぎたら箱の中で動いたまま）→ 箱の中身をまとめて `devkit\hands\try\current.json` に書く → TRY の窓が出る（通知音 1 回）。返事待ちの間は次の pc_try はできない（discard で捨てれば次へ）
  - 箱の中身の読み方（2026-11-28 に記録で確かめた）：`drive\C\…`＝C:\…・`user\current\…`＝%USERPROFILE%\…・`user\all\…`＝%ProgramData%\…。本物を変えると写しが箱にでき、消すと大きさ 0・作成日時 1986-05-24 の印になる。根の直下の RegHive などはレジストリ（入れない）。CatRoot2・Recent・PowerShell の起動の記録・Temp などは Windows が勝手に作るもの（数えるだけ）。本物と同じ中身のもの（開いただけ）は数えない。Windows・Program Files・ProgramData の中は「管理者の場所」（入れない。本物でもう一度入れ直す）
- **TRY の窓**（LIVE の窓の中。色は青緑）：ファイル名・Defender の結果・増えた／変わった／消えた・一覧（8 つまで）・「ネットは切ってある（ネットありでは試していない）」。ボタンはカオルだけ：
  - ［箱の中で開く］：窓が自分で Start.exe /box:ClaudeBox を動かす（ネットなしの箱で使ってみる。窓のふちが水色）
  - ［本物に入れる］（2 回押す）：窓が合言葉（request.key）で署名して常駐役に頼む（Claude は合言葉を読めないので頼めない）。**全部入れるか、何も入れないか**（pc-64〜）：
    1. 箱の中を読み直し、窓に出したものと違えば（名前・大きさ・**中身の SHA-256**。大きさが同じでも中身が変わっていれば）一覧を新しくして止める（もう一度押す）
    2. 先に全部を確かめる：試しのあとで本物が変わったもの・同じ名前のものが本物にできた・消すはずのものが本物で変わった／もう無い、が 1 つでもあれば、何も入れずに止める（一部だけ入れることはしない）
    3. 全部を一時ファイルに写し、窓に出した中身（SHA-256）と同じかを確かめる
    4. 変わるものの元と箱の中で消えたものを取り置きへ。1 つでも送れなければ、送った分を取り置きから戻して止める
    5. 置き換える。途中で失敗したら、置いた分を外して取り置きから元を戻し、戻せたか（中身が同じか）を返事に出す。戻せなかったものはカオルが取り置きから戻す
    - 管理者の場所のものは入れない（窓に出ている）。終わったら箱を空にする
  - ［捨てる］（2 回押す）：箱を空にする（本物は何も変わらない）。Claude も pc_try action=discard で捨てられる
- 箱の中身（C:\Sandbox）は台本から触らない（読むのは pc_files でよい。開く・消す・写すは止める）
- 容量の点検：箱が 5 GB を超えたら空にする（返事待ちのときは知らせるだけ）
- まだ：ネットありで試したいとき（カオルに聞く）・TRY の結果を DONE の報告に入れる

## 容量の点検（2026-11-28 から。pc-49〜。ASK-DESIGN.md の「容量の決まり」）

- 常駐役が、動き出して 1 分後と、その後 1 時間ごとに「前の点検から 20 時間たったか」を見て、たっていれば点検する。結果は `devkit\hands\status\capacity.txt`。pc_status の「容量の点検」に出て、知らせがあれば **pc_status のいちばん上に【知らせ】** が出る（Claude はチャットでもカオルに伝える）
- 自動で片付けるのは claude-hands が自分で作ったものだけ：
  - 記録（TIMELINE・DONE の報告・asks.jsonl）：90 日より古いものを消す。合わせて 200 MB を超えたら古いものから
  - Claude 用の Chrome：閉じるたびと点検で、キャッシュ（Cache・Code Cache・GPUCache など）を消す（Chrome が動いている間は消さない。lockfile が消せるかで見る）。それでも 1 GB を超えたら中身を作り直す（ログインは入っていない。見た目の設定は次の起動で入れ直す）
  - 窓の中身（SWITCH・LIVE の WebView2）：200 MB を超えたらキャッシュを消す
  - 道具の一時ファイル（%TEMP% の名前に claude-hands を含むもの）：1 日
  - Claude が落としたもの（`devkit\downloads`）：14 日たったら取り置きへ
  - Git の控え：週 1 回まとめる（git gc）。500 MB を超えたら知らせる
  - iPhone のログ：100 MB で回して 2 つまで（合わせて 200 MB。mcp-25 から）
- 知らせるのは：C の空きが 100 GB・50 GB を切った／OneDrive への写しが 1 日止まっている／復元ポイントが 3 日より古い／claude-hands の合計が 20 GB を超えた／自動で片付けられなかった
- **CHECK の窓**（pc-51〜）：知らせがあると、LIVE の窓が右下に CHECK（黄色）を出す。Claude が動いている間は LIVE を優先。カオルが［閉じる］を押した日はもう出さず、知らせが続いていれば次の日にまた出る（閉じた日は `devkit\hands\status\check-closed.txt`）
- **OneDrive の契約**（pc-51〜）：使用量は PC の中に無い（取るには OneDrive の設定の窓を開いて読むしかない）ので、80% の知らせの代わりに、レジストリの契約の状態（有料か・有効か・期限・自動更新）を見る。知らせるのは：有効でない／自動更新なしで期限まで 14 日を切った／無料（5 GB）で PC-Backup の写しが 4 GB を超えた。pc_status の「OneDrive の契約」に、状態と次の期限と PC-Backup の写しの大きさ（クラウドの分も含む）が出る
- pc_trash で中身を取り置きに写して消したあと、空のフォルダだけ使用中で残ることがある（エクスプローラーで開いている・そこから起動したものが動いている）。返事は「（空のフォルダだけ使用中で残った。中身は取り置きにある）」。pc-setup.bat は LIVE の窓を devkit\hands で起動するように直した（pc-51。zip のフォルダを握ったままにしない）
- 管理者の権限が要るもの（復元ポイント・取り置きの期限）は今までどおり SYSTEM の点検（毎日 12:00）
- Windows の設定（2026-11-28。カオルの頼みで Claude が設定の画面から）：ストレージ センサー＝毎日・一時ファイルの片付けオン・ごみ箱 30 日・ダウンロードは触らない・OneDrive は 1 日開かなければクラウドだけに。OneDrive の PC に出すフォルダは PC-Backup だけ。PC-Backup の写しは「クラウドだけ」

## よく使う流れ

| したいこと | やり方 |
| --- | --- |
| 容量が減った原因 | `pc_disk`（引数なし）でドライブと膨らみやすい場所 → 大きい所を `pc_disk path=... files=true` で掘る |
| アプリの不具合 | `pc_problems app=アプリ名 hours=72` → 落ちた回数・エラーの中身・WER の控えの場所 → そのアプリのログを `pc_files find`（`*.log`・`sort: new`）→ `pc_files read tail=200` |
| 重い | `pc_processes`（メモリ）と `pc_processes sort=cpu` |
| アプリを操作 | `pc_open` → `pc_windows` → `pc_ui window=... filter=...` で名前を見る → `pc_ui_do` → 結果を `pc_ui` か `pc_screenshot window=...` で確かめる |
| 画像を見て押す | `pc_screenshot` → 画像の座標を読む → `pc_ui_do action=click_xy from_image=true x=.. y=..` |
| ブラウザで作業 | `browser_tabs` → `browser_read`（部品の番号）→ `browser_do`（何手もまとめて。終わりに部品の一覧が返る）→ 足りなければ `browser_read mode=text filter=…` |

## 気をつけること（試験で分かったこと）

- 窓を前に出すのに Alt キーを使ってはいけない。メニューが開いて、そのあと打った文字がメニューの操作になる（メモ帳が「閉じる」になった）。今は空のキーと UI オートメーションで前に出し、出せなければ打たない
- メモ帳などの文書の欄は「値」ではなく「文書」の部品。`set_value` は、全部選んで貼る形で入れる
- 最初の画面系の呼び出し（pc_windows など）は、部品の読み込みで 5 秒ほどかかる。2 回目からは 0.1 秒前後
- 道具の返事には、アプリの一覧・ファイル名・窓の題名など個人の情報が出る。repo や記録に写すときは伏せる

## うまくいかないとき

| 症状 | まずやること |
| --- | --- |
| 「スイッチがオフ」 | 道具が「許可しますか？」の窓を出して 60 秒待つ。それでもだめなら、カオルに PC-VIEW か PC-OPERATE を押してもらう |
| 「カオルの許可が要ります」 | 道具が理由つきで「許可しますか？」の窓を出して 90 秒待つ。本当に要る操作のときだけ呼ぶ |
| 「常駐役が断りました」 | 常駐役を起動し直す（カオルに PC-OFF → PC-VIEW） |
| 道具がおかしい | `pc_status` の最近の記録。全部は `%USERPROFILE%\devkit\pc-agent\audit.log`・`agent.log` |

## 中の作り

- Go（`tools/pc-mcp`）。同じソースから、拡張機能（コンソール）と常駐役（窓なし）の 2 つの exe を作る
- PowerShell（Windows PowerShell 5.1）を開きっぱなしにして、台本を base64 の 1 行で送り、終わりの印まで読む。時間切れなら起動し直す
- 容量の計算とファイルは Go（速い）。つなぎ目（ジャンクション）はたどらない。OneDrive のクラウドのみのファイルは分けて数える
- 台本は `tools/pc-mcp/scripts/*.ps1`（exe に埋め込む）
- 試験：`tools/pc-mcp/test/scenario.py`（GitHub の Windows で毎回。手元では Linux＋pwsh でも動く）。ブラウザは headless の Chrome（一時のプロフィール・`--remote-debugging-port=0`）と `test/site/` の試験のページで試す（`PC_BROWSER_USERDATA` で User Data の場所を変える）
- ブラウザ：`browser*.go`。WebSocket と CDP は外の部品を使わず手で書いた（127.0.0.1 だけ）。ページの中に置く小さな道具箱（dkLib）が、部品の番号・名前・伏せる欄の見分けをする
- ビルド：`.github/workflows/pc-mcp.yml`（Linux でビルド → Windows で通しの試験 → Releases `pc-N`）

## カオルの PC で確かめたこと（2026-11-25）

- ストア版の Claude の中の拡張機能から、常駐役（127.0.0.1）と token.txt に届く：OK
- PC-ALLOW.bat：UAC の「はい」→ 許可 10 分 → 鍵の場所（selfIdentity.plist）をごみ箱へ送れた：OK
- 容量・不具合の記録・PowerShell・ごみ箱（73 個）：OK。1 回 1〜2 秒
- .bat の日本語の表示（CP932）：OK（カオルに見てもらった）
- まだ：画面の拡大率（DPI）が 100% でないときの座標、画面・部品の操作

## 守りの細かいところ

- 文字で見る守りは、レジストリの HKLM: という文字があると、読むだけ（Get-ItemProperty）でも許可が要る。読むだけで要るなら、powercfg などの別の命令で代わりに調べる
