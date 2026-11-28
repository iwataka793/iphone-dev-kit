package main

// PowerShell の台本で動く道具。台本は scripts/*.ps1（exe に埋め込む）。

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

//go:embed scripts/*.ps1
var scriptFS embed.FS

func script(name string) string {
	b, err := scriptFS.ReadFile("scripts/" + name)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// 画面や入力を使う台本は、共通の部品（common.ps1）を先に読む
func uiScript(name string) string { return script("common.ps1") + "\n" + script(name) }

var scriptDrives = script("drives.ps1")

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// psReply は台本の出力を答えにする（"ERROR: " で始まる行があればエラー扱い）。
func psReply(out string, err error) Reply {
	if err != nil {
		if out != "" {
			return errText("%v\n%s", err, out)
		}
		return errText("%v", err)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "ERROR: ") {
		return errText("%s", strings.TrimPrefix(strings.TrimSpace(out), "ERROR: "))
	}
	if strings.TrimSpace(out) == "" {
		out = "（出力なし）"
	}
	return okText(out)
}

func (a *Agent) problems(ctx context.Context, raw json.RawMessage) Reply {
	var in map[string]any
	_ = json.Unmarshal(raw, &in)
	return psReply(a.ps.Run(ctx, script("problems.ps1"), in, 180*time.Second))
}

func (a *Agent) processes(ctx context.Context, raw json.RawMessage) Reply {
	var in map[string]any
	_ = json.Unmarshal(raw, &in)
	return psReply(a.ps.Run(ctx, script("processes.ps1"), in, 60*time.Second))
}

func (a *Agent) apps(ctx context.Context, raw json.RawMessage) Reply {
	var in map[string]any
	_ = json.Unmarshal(raw, &in)
	return psReply(a.ps.Run(ctx, script("apps.ps1"), in, 90*time.Second))
}

func (a *Agent) windows(ctx context.Context, raw json.RawMessage) Reply {
	var in map[string]any
	_ = json.Unmarshal(raw, &in)
	return psReply(a.ps.Run(ctx, uiScript("windows.ps1"), in, 60*time.Second))
}

func (a *Agent) ui(ctx context.Context, raw json.RawMessage) Reply {
	var in map[string]any
	_ = json.Unmarshal(raw, &in)
	if w, _ := in["window"].(string); w != "" && sensitiveWindowRe.MatchString(w) {
		if ok, _ := a.st.Allowed(); !ok {
			return errText("窓「%s」：%s", w, allowMsg("パスワード・セキュリティ・お金にかかわりそうな窓")).needs(windowWhat(sensitiveWindowRe.FindString(w), "読む"))
		}
	}
	return psReply(a.ps.Run(ctx, uiScript("ui.ps1"), in, 90*time.Second))
}

// ---- 画面 ----

type shotInfo struct {
	Left, Top, Width, Height int
	Factor                   int
}

var (
	lastShotMu sync.Mutex
	lastShot   shotInfo
)

func (a *Agent) screenshot(ctx context.Context, raw json.RawMessage) Reply {
	var in struct {
		Window   string `json:"window"`
		Activate bool   `json:"activate"`
		Full     bool   `json:"full"`
		MaxWidth int    `json:"max_width"`
	}
	_ = json.Unmarshal(raw, &in)
	restore := hideLive(150 * time.Millisecond) // LIVE の窓は撮った画面に写さない
	out, err := a.ps.Run(ctx, uiScript("screenshot.ps1"), in, 60*time.Second)
	restore()
	if err != nil || strings.HasPrefix(strings.TrimSpace(out), "ERROR: ") {
		return psReply(out, err)
	}
	var s struct {
		Left, Top, Width, Height int
		Title                    string
		PNG                      string `json:"png"`
	}
	if err := json.Unmarshal([]byte(strings.ReplaceAll(strings.ReplaceAll(out, "\n", ""), "\r", "")), &s); err != nil {
		return errText("画面の結果が読めません：%v", err)
	}
	png, err := base64.StdEncoding.DecodeString(s.PNG)
	if err != nil {
		return errText("画面の結果が読めません：%v", err)
	}
	if in.MaxWidth <= 0 {
		in.MaxWidth = 1280
	}
	factor := 1
	for s.Width/factor > in.MaxWidth {
		factor++
	}
	if in.Full {
		factor = 1
	}
	img, mime := png, "image/png"
	if factor > 1 || !in.Full {
		if j, _, _, err := shrink(png, factor, 80); err == nil {
			img, mime = j, "image/jpeg"
		}
	}
	lastShotMu.Lock()
	lastShot = shotInfo{s.Left, s.Top, s.Width, s.Height, factor}
	lastShotMu.Unlock()
	what := "画面全体"
	if s.Title != "" {
		what = "窓「" + s.Title + "」"
	}
	msg := fmt.Sprintf("%s（%dx%d を 1/%d に縮めた）。画像の (x, y) は画面の (%d + x×%d, %d + y×%d)。pc_ui_do の click_xy で from_image: true にすると、画像の座標のまま押せる",
		what, s.Width, s.Height, factor, s.Left, factor, s.Top, factor)
	return okText(msg).withImage(img, mime)
}

// ---- PowerShell ----

type psIn struct {
	Script  string `json:"script"`
	Seconds int    `json:"seconds"`
	Max     int    `json:"max_chars"`
	Real    bool   `json:"real"` // 本物の PC で（何かを変える台本も）。既定は箱の中（2026-11-28 カオルが決めた B）
}

func (a *Agent) inspect(ctx context.Context, src string) (scriptInfo, error) {
	if a.insp != nil {
		return a.insp(src)
	}
	out, err := a.ps.Run(ctx, script("astinfo.ps1"), map[string]string{"src": src}, 30*time.Second)
	if err != nil {
		return scriptInfo{}, err
	}
	var info scriptInfo
	if err := json.Unmarshal([]byte(strings.TrimSpace(strings.ReplaceAll(out, "\n", ""))), &info); err != nil {
		return scriptInfo{}, fmt.Errorf("台本を調べた結果が読めません：%v（%s）", err, clip(out, 200))
	}
	return info, nil
}

func (a *Agent) powershell(ctx context.Context, raw json.RawMessage) Reply {
	in, err := decode[psIn](raw)
	if err != nil || strings.TrimSpace(in.Script) == "" {
		return errText("script（PowerShell の台本）を指定してください")
	}
	info, err := a.inspect(ctx, in.Script)
	if err != nil {
		return errText("%v", err)
	}
	block, needAllow, viewNG, what := checkScriptWhat(info, in.Script)
	// 何かを変える台本は、既定で箱（ClaudeBox）の中で動かす。本物を変えるのは real: true のときだけ（守りは今までどおり）
	if !in.Real && (viewNG != "" || block == tryMsg) {
		if a.st.Mode() != ModeOperate {
			if viewNG == "" {
				viewNG = "落としたものを動かす"
			}
			return errText("スイッチが「見るだけ」なので、この台本は使えません（%s）。見るだけで使えるのは Get- / Test- / Measure- / Select- などの読む命令だけ。要るならカオルに PC-OPERATE.bat を押してもらう", viewNG)
		}
		return a.psInBox(ctx, in, info, viewNG)
	}
	if block != "" {
		return errText("%s", block)
	}
	if a.st.Mode() != ModeOperate && viewNG != "" {
		return errText("スイッチが「見るだけ」なので、この台本は使えません（%s）。見るだけで使えるのは Get- / Test- / Measure- / Select- などの読む命令だけ。要るならカオルに PC-OPERATE.bat を押してもらう", viewNG)
	}
	if needAllow != "" {
		if ok, _ := a.st.Allowed(); !ok {
			return errText("%s", allowMsg(needAllow)).needs(what)
		}
	}
	if in.Seconds <= 0 {
		in.Seconds = 60
	}
	if in.Seconds > 600 {
		in.Seconds = 600
	}
	if in.Max <= 0 {
		in.Max = 30000
	}
	// 道：読むだけの台本は A。何かを変える台本は C（先に控え → 本物でやる → 確かめる → だめなら戻す）
	lane := "A"
	if viewNG != "" {
		lane = "C"
	}
	var c *cRun
	if lane == "C" {
		var err error
		if c, err = a.beginC(ctx, in.Script); err != nil {
			// 控えが取れないと、エラーのときに戻せない。戻せない形では本物を変えない（2026-11-28）
			r := errText("控え（Git の記録）が取れなかったので、本物では何も変えずに止めました：%v\n（Git の記録の場所か Git の具合を直してから。急ぐならカオルに聞く）", err)
			r.Lane = lane
			return r
		}
	}
	out, err := a.ps.Run(ctx, in.Script, nil, time.Duration(in.Seconds)*time.Second)
	if err != nil && stoppedBySwitch(ctx) {
		err = errors.New(stopMsg + "（台本の中で起動した別のプログラムは動いたままのことがある）")
	}
	if r := []rune(out); len(r) > in.Max {
		out = string(r[:in.Max]) + fmt.Sprintf("\n（出力が長いので %d 文字で切った。全部で %d 文字）", in.Max, len(r))
	}
	var tail string
	reverted := false
	if c != nil {
		// 後の確かめ（エラーなら決まったフォルダを控えに戻す）は、STOP で止めたあとも切らずにやる
		cc, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		tail, reverted = a.endC(cc, c, out, err)
		cancel()
	}
	if err != nil {
		r := errText("%v\n%s%s", err, out, tail)
		r.Lane = lane
		return r
	}
	if strings.TrimSpace(out) == "" {
		out = "（出力なし）"
	}
	r := okText(out + tail)
	r.Lane = lane
	r.IsError = reverted
	return r
}

// psInBox：何かを変える台本を箱の中で動かす（道 B）。pc_try の作業場と同じ箱（続けて試せる・offer で TRY の窓へ）
func (a *Agent) psInBox(ctx context.Context, in psIn, info scriptInfo, why string) Reply {
	block, needAllow, what := boxScriptCheck(info, in.Script)
	if block != "" {
		return errText("%s", block)
	}
	if needAllow != "" {
		if ok, _ := a.st.Allowed(); !ok {
			return errText("%s（箱の中で動かすときも）", allowMsg(needAllow)).needs(what)
		}
	}
	if in.Max <= 0 {
		in.Max = 30000
	}
	note := ""
	if in.Seconds > 110 {
		note = fmt.Sprintf("（待つのは 110 秒まで。%d 秒は箱では待てない。過ぎても箱の中では動いたまま。pc_try action=show で続きを見る）\n", in.Seconds)
	}
	tryMu.Lock()
	r := a.tryRun(ctx, tryArgs{Script: in.Script, Seconds: in.Seconds})
	tryMu.Unlock()
	head := "箱（ClaudeBox）の中で動かした（" + why + "があるので。本物の PC は変わらない）。\n" + note
	if r.IsError {
		r.Text = head + r.Text + "\n（本物の PC で変える必要がある作業なら real: true。そのときは今までどおりの守り（許可・控え・確かめ）を通る）"
	} else {
		r.Text = head + r.Text + "\n本物に入れたいものは pc_try action=offer でカオルの TRY の窓へ。頼まれた作業で本物を変える必要があるときだけ real: true"
	}
	if rs := []rune(r.Text); len(rs) > in.Max {
		r.Text = string(rs[:in.Max]) + fmt.Sprintf("\n（出力が長いので %d 文字で切った。全部は pc_try action=show）", in.Max)
	}
	r.Lane = "B"
	return r
}

// ---- C：控えを取ってから変える ----

type cRun struct {
	snap  string
	start time.Time
}

// beginC：控え（Git の記録）を取る。取れなければ err（本物は変えない）
func (a *Agent) beginC(ctx context.Context, script string) (*cRun, error) {
	c := &cRun{start: time.Now()}
	s, err := gitSnapshot(ctx, "C の前："+oneLine(script, 80))
	if err != nil {
		return nil, err
	}
	if s == "" {
		return nil, errors.New("記録の番号が返らなかった")
	}
	c.snap = s
	_, _ = a.ps.Run(ctx, "$Error.Clear()", nil, 10*time.Second)
	return c, nil
}

// endC：確かめる（エラーが出たか・Windows のエラーの記録が増えたか・決まったフォルダの変更）。エラーが出て決まったフォルダが変わっていたら戻す。
// 「エラーが出た」は、台本が止まった・時間切れ、か、画面に出たエラー（FullyQualifiedErrorId の行）があるとき。
// -ErrorAction SilentlyContinue で黙らせたエラーは数えない（わざと無視したもの）。
func (a *Agent) endC(ctx context.Context, c *cRun, out string, runErr error) (string, bool) {
	var ls []string
	shown := strings.Count(out, "FullyQualifiedErrorId")
	all := 0
	if o, err := a.ps.Run(ctx, "$Error.Count", nil, 10*time.Second); err == nil {
		fmt.Sscanf(strings.TrimSpace(o), "%d", &all)
	}
	failed := runErr != nil || shown > 0
	switch {
	case runErr != nil:
		ls = append(ls, "エラー：台本が止まった（"+oneLine(runErr.Error(), 80)+"）")
	case shown > 0:
		ls = append(ls, fmt.Sprintf("エラー：%d 件（画面に出たもの）", shown))
	case all > 0:
		ls = append(ls, fmt.Sprintf("エラー：なし（黙らせたもの %d 件は数えない）", all))
	default:
		ls = append(ls, "エラー：なし")
	}
	ev, _ := a.ps.Run(ctx, `$s = [datetime]::Parse($A.since); $e = @(Get-WinEvent -FilterHashtable @{LogName='Application','System'; Level=1,2; StartTime=$s} -ErrorAction SilentlyContinue); if ($e.Count) { "{0} 件（{1}）" -f $e.Count, (($e | Select-Object -ExpandProperty ProviderName -Unique | Select-Object -First 3) -join '・') } else { 'なし' }`,
		map[string]any{"since": c.start.Add(-time.Second).Format(time.RFC3339)}, 20*time.Second)
	if ev = strings.TrimSpace(ev); ev != "" {
		ls = append(ls, "Windows のエラーの記録（この間に増えた分）："+ev)
	}
	reverted := false
	if c.snap == "" {
		ls = append(ls, "控え：取れなかった（決まったフォルダを Git で記録できない）")
	} else if after, err := gitSnapshot(ctx, "C の後"); err == nil {
		cs, _ := gitChanges(ctx, c.snap, after)
		switch {
		case len(cs) == 0:
			ls = append(ls, "決まったフォルダ（"+strings.Join(gitPaths(), "・")+"）の変更：なし")
		case failed:
			ls = append(ls, fmt.Sprintf("エラーが出たので、決まったフォルダの変更 %d 件を自動で戻した：", len(cs)))
			ls = append(ls, a.restoreGit(ctx, c.snap, after)...)
			_, _ = gitSnapshot(ctx, "C を戻した")
			reverted = true
		default:
			ls = append(ls, fmt.Sprintf("決まったフォルダの変更：%d 件（DONE の［まとめて戻す］で戻せる）", len(cs)))
		}
	} else {
		ls = append(ls, "後の記録が取れなかった（"+oneLine(err.Error(), 80)+"）。前の控え "+shortID(c.snap)+" は残っている（戻すのは hands action=undo か Git の記録から）")
	}
	return "\n—— C（変更）の確かめ ——\n" + strings.Join(ls, "\n"), reverted
}

// ---- 動かす ----

func (a *Agent) open(ctx context.Context, raw json.RawMessage) Reply {
	var in struct {
		Target string `json:"target"`
		Args   string `json:"args"`
	}
	_ = json.Unmarshal(raw, &in)
	if in.Target == "" {
		return errText("target（開くもの：アプリ名・ファイル・フォルダ・URL）を指定してください")
	}
	if msg := openRuns(in.Target, in.Args); msg != "" && !strings.Contains(in.Target, "://") {
		return errText("%s", msg)
	}
	if !strings.Contains(in.Target, "://") {
		if r := a.needAllowFor(in.Target, false); r != nil {
			return *r
		}
	}
	if m := sensitiveWindowRe.FindString(in.Target + " " + in.Args); m != "" || strings.Contains(strings.ToLower(in.Args), "runas") {
		if ok, _ := a.st.Allowed(); !ok {
			w := "「" + m + "」にかかわりそうなものを開く"
			if m == "" {
				w = "アプリを管理者として開く（UAC）"
			}
			return errText("%s", allowMsg("パスワード・セキュリティ・お金にかかわりそうなもの（"+m+"）")).needs(w)
		}
	}
	return psReply(a.ps.Run(ctx, script("open.ps1"), in, 60*time.Second))
}

func (a *Agent) checkWindow(ctx context.Context, window string) *Reply {
	// 指定が無いときは、いま前にある窓を調べる
	name := window
	if name == "" {
		out, err := a.ps.Run(ctx, uiScript("windows.ps1"), map[string]any{}, 30*time.Second)
		if err == nil {
			for _, l := range strings.Split(out, "\n") {
				if strings.Contains(l, "← いま前") {
					name = l
				}
			}
		}
	}
	if m := handsWindowRe.FindString(name); m != "" {
		r := errText("窓「%s」はカオルの窓です。スイッチ・許可・LIVE・DONE の窓は Claude からは押せない・打てない（押すのはカオルだけ。許可があっても）", strings.TrimSpace(name))
		return &r
	}
	if isBoxWindow(name) {
		r := errText("窓「%s」は%sの窓です。Claude からは押せない・打てない（許可があっても）", strings.TrimSpace(name), boxConfWhy)
		return &r
	}
	if m := sensitiveWindowRe.FindString(name); m != "" {
		if ok, _ := a.st.Allowed(); !ok {
			r := errText("窓「%s」：%s", strings.TrimSpace(name), allowMsg("パスワード・セキュリティ・お金にかかわりそうな窓（"+m+"）")).needs(windowWhat(m, "押す・打つ"))
			return &r
		}
	}
	return nil
}

func (a *Agent) uiDo(ctx context.Context, raw json.RawMessage) Reply {
	var in map[string]any
	_ = json.Unmarshal(raw, &in)
	w, _ := in["window"].(string)
	if r := a.checkWindow(ctx, w); r != nil {
		return *r
	}
	if fi, _ := in["from_image"].(bool); fi {
		lastShotMu.Lock()
		s := lastShot
		lastShotMu.Unlock()
		if s.Factor == 0 {
			return errText("先に pc_screenshot で画面を撮ってください（from_image は最後に撮った画像の座標）")
		}
		x, _ := in["x"].(float64)
		y, _ := in["y"].(float64)
		in["x"] = float64(s.Left) + x*float64(s.Factor)
		in["y"] = float64(s.Top) + y*float64(s.Factor)
	}
	act, _ := in["action"].(string)
	act = strings.ToLower(act)
	if strings.HasSuffix(act, "_xy") {
		x, _ := in["x"].(float64)
		y, _ := in["y"].(float64)
		// 位置で押す間は LIVE の窓を隠す（LIVE の下にあるものを押す。押せるかの確かめも、隠したあとの窓で）
		restore := hideLive(60 * time.Millisecond)
		defer restore()
		if t := windowTitleAt(int(x), int(y)); handsWindowRe.MatchString(t) {
			return errText("そこはカオルの窓（%s）です。スイッチ・許可・LIVE・DONE の窓は Claude からは押せません（押すのはカオルだけ）", t)
		} else if isBoxWindow(t) {
			return errText("そこは%sの窓（%s）です。Claude からは押せません", boxConfWhy, t)
		}
		// E：その位置の部品の名前が「送信」などなら押さない
		if name := a.nameAt(ctx, x, y); finalWord(name) != "" {
			return stopE("その位置のボタン「"+oneLine(name, 30)+"」", finalWord(name))
		}
	}
	if act == "click" || act == "double" || act == "toggle" || act == "select" {
		if t, _ := in["target"].(string); finalWord(t) != "" {
			return stopE("ボタン「"+oneLine(t, 30)+"」", finalWord(t))
		}
	}
	return psReply(a.ps.Run(ctx, uiScript("uido.ps1"), in, 60*time.Second))
}

func (a *Agent) keys(ctx context.Context, raw json.RawMessage) Reply {
	var in map[string]any
	_ = json.Unmarshal(raw, &in)
	w, _ := in["window"].(string)
	if r := a.checkWindow(ctx, w); r != nil {
		return *r
	}
	// E：ブラウザ・チャット・メールの窓で、送信に当たるキー（Enter）を打たない（Enter だけで送られる）
	k, _ := in["keys"].(string)
	tx, _ := in["text"].(string)
	en, _ := in["enter"].(bool)
	if sendKeys(k, tx, en) {
		if line := a.windowLine(ctx, w); sendApp(line) {
			return stopE("この窓（"+oneLine(strings.TrimSpace(strings.TrimSuffix(line, "← いま前")), 60)+"）での Enter（送信に当たるキー）", "Enter")
		}
	}
	return psReply(a.ps.Run(ctx, uiScript("keys.ps1"), in, 60*time.Second))
}

func (a *Agent) trash(ctx context.Context, raw json.RawMessage) Reply {
	var in struct {
		Paths   []string `json:"paths"`
		Recycle bool     `json:"recycle"`
	}
	_ = json.Unmarshal(raw, &in)
	if len(in.Paths) == 0 {
		return errText("paths（消すファイル・フォルダ）を指定してください")
	}
	var ps []string
	for _, p := range in.Paths {
		n := normPath(p)
		if r := a.needAllowFor(n, true); r != nil {
			return *r
		}
		ps = append(ps, n)
	}
	// 取り置き（30 日・Claude には消せない場所）に写してから、元をごみ箱へ。
	// 取り置きに入らない（大きい・C の空きが少ない）ものは止める。カオルに聞いてから recycle: true でごみ箱だけに送る
	hi := scanHolding()
	capB, why := holdCap(cFree())
	return psReply(a.ps.Run(ctx, script("trash.ps1"), map[string]any{
		"paths": ps, "recycle": in.Recycle, "ready": hi.ready, "holding": holdingDir(),
		"used": hi.bytes, "cap": capB, "capWhy": why,
	}, 600*time.Second))
}

// nameAt：画面の (x, y) にある部品の名前（UI オートメーション）
func (a *Agent) nameAt(ctx context.Context, x, y float64) string {
	out, err := a.ps.Run(ctx, `Add-Type -AssemblyName UIAutomationClient, UIAutomationTypes, WindowsBase
try { $e = [System.Windows.Automation.AutomationElement]::FromPoint((New-Object System.Windows.Point ([double]$A.x), ([double]$A.y))); $e.Current.Name } catch { '' }`,
		map[string]any{"x": x, "y": y}, 20*time.Second)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// windowLine：pc_windows の一覧から、spec（題名の一部・プログラム名・pid）に合う窓の行。spec が空ならいま前の窓
func (a *Agent) windowLine(ctx context.Context, spec string) string {
	out, err := a.ps.Run(ctx, uiScript("windows.ps1"), map[string]any{}, 30*time.Second)
	if err != nil {
		return ""
	}
	spec = strings.ToLower(strings.TrimSpace(spec))
	for _, l := range strings.Split(out, "\n") {
		if spec == "" && strings.Contains(l, "← いま前") {
			return l
		}
		if spec != "" && strings.Contains(strings.ToLower(l), spec) {
			return l
		}
	}
	return ""
}

// shortID：Git の記録の番号の先頭 8 文字
func shortID(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
