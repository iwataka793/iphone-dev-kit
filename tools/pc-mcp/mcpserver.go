package main

// 拡張機能の役：MCP の道具を並べ、中身は常駐役（127.0.0.1:60210）に頼む。
// 常駐役が動いていない＝スイッチがオフ。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var errAgentDown = errors.New("agent down")

func callAgent(ctx context.Context, tool string, args any, timeout time.Duration) (Reply, error) {
	tok, err := os.ReadFile(filepath.Join(stateDir(), "token.txt"))
	if err != nil {
		return Reply{}, errAgentDown
	}
	body, _ := json.Marshal(map[string]any{"tool": tool, "args": args})
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+agentAddr+"/call", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(tok)))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return Reply{}, fmt.Errorf("常駐役の返事が %0.f 秒で来ませんでした", timeout.Seconds())
		}
		return Reply{}, errAgentDown
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Reply{}, fmt.Errorf("常駐役が断りました（%d）。常駐役を起動し直す（PC-OFF → PC-VIEW）とよいかも", res.StatusCode)
	}
	var rep Reply
	if err := json.NewDecoder(res.Body).Decode(&rep); err != nil {
		return Reply{}, err
	}
	return rep, nil
}

func toResult(rep Reply) *mcp.CallToolResult {
	r := &mcp.CallToolResult{IsError: rep.IsError}
	for _, im := range rep.Images {
		r.Content = append(r.Content, &mcp.ImageContent{Data: im.Data, MIMEType: im.MIME})
	}
	r.Content = append(r.Content, &mcp.TextContent{Text: rep.Text})
	return r
}

func errResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

const offMsg = "スイッチがオフです（PC の常駐役が動いていない）。カオルに PC-VIEW.bat（見るだけ）か PC-OPERATE.bat（操作も可）を押してもらってください。切り替えはカオルだけ"

// forward は、道具 tool を常駐役に頼む関数を作る。
func forward[T any](tool string, timeout time.Duration) func(context.Context, *mcp.CallToolRequest, T) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in T) (*mcp.CallToolResult, any, error) {
		rep, err := callAgent(ctx, tool, in, timeout)
		if isBrowserTool(tool) {
			return browserForward(ctx, tool, in, timeout, rep, err)
		}
		if errors.Is(err, errAgentDown) {
			if tool == "pc_status" {
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "常駐役：動いていない\nスイッチ：オフ\n拡張機能の版：" + version + "\n使うときはカオルに PC-VIEW.bat か PC-OPERATE.bat を押してもらう"}}}, nil, nil
			}
			// カオルの PC に「許可しますか？」の窓を出してもらい、60 秒だけ待つ（決めるのはカオル）
			kind := "pc-view"
			if toolNeed[tool] == ModeOperate {
				kind = "pc-operate"
			}
			asked, need := askFor(kind, tool, actionOf(tool), in)
			if need != "" {
				return errResult(offMsg + "。" + need), nil, nil
			}
			if !asked {
				return errResult(offMsg), nil, nil
			}
			if ok, why := waitKaoru(kind, 60*time.Second, func() bool { _, e := callAgent(ctx, "pc_status", noIn{}, 5*time.Second); return e == nil }); !ok {
				return errResult(offMsg + "。" + why), nil, nil
			}
			rep, err = callAgent(ctx, tool, in, timeout)
		}
		if err == nil && rep.IsError && strings.HasPrefix(rep.Text, "PC のスイッチがオフ") {
			// 常駐役はブラウザのために動いているが、PC はオフ
			kind, want := "pc-view", ModeView
			if toolNeed[tool] == ModeOperate {
				kind, want = "pc-operate", ModeOperate
			}
			st := NewState(stateDir(), 0)
			if asked, need := askFor(kind, tool, actionOf(tool), in); need != "" {
				rep.Text += "\n（" + need + "）"
			} else if asked {
				if ok, why := waitKaoru(kind, 60*time.Second, func() bool { return st.Mode() >= want }); ok {
					rep, err = callAgent(ctx, tool, in, timeout)
				} else {
					rep.Text += "\n（" + why + "）"
				}
			}
		}
		if err == nil && rep.IsError && strings.HasPrefix(rep.Text, "スイッチが「見るだけ」") {
			st := NewState(stateDir(), 0)
			if asked, need := askFor("pc-operate", tool, actionOf(tool), in); need != "" {
				rep.Text += "\n（" + need + "）"
			} else if asked {
				if ok, why := waitKaoru("pc-operate", 60*time.Second, func() bool { return st.Mode() == ModeOperate }); ok {
					rep, err = callAgent(ctx, tool, in, timeout)
				} else {
					rep.Text += "\n（" + why + "）"
				}
			}
		}
		if err == nil && rep.IsError && strings.Contains(rep.Text, "カオルの許可が要ります") {
			st := NewState(stateDir(), 0)
			if asked, need := askFor("pc-allow", tool, allowWhat(tool, rep), in); need != "" {
				rep.Text += "\n（" + need + "）"
			} else if asked {
				if ok, w := waitKaoru("pc-allow", 90*time.Second, func() bool { ok, _ := st.Allowed(); return ok }); ok {
					rep, err = callAgent(ctx, tool, in, timeout)
				} else {
					rep.Text += "\n（" + w + "）"
				}
			}
		}
		if err != nil {
			return errResult(err.Error()), nil, nil
		}
		return toResult(rep), nil, nil
	}
}

// browserForward：ブラウザの道具のスイッチが足りないとき、ブラウザの「許可しますか？」を出して待つ。
func browserForward(ctx context.Context, tool string, in any, timeout time.Duration, rep Reply, err error) (*mcp.CallToolResult, any, error) {
	need := toolNeed[tool]
	if tool == "browser_tabs" {
		if m, ok := in.(brTabsIn); ok && m.Action != "" && strings.ToLower(m.Action) != "list" {
			need = ModeOperate
		}
	}
	kind, want := "browser-view", ModeView
	if need == ModeOperate {
		kind, want = "browser-operate", ModeOperate
	}
	bst := NewState(browserDir(), 0)
	short := errors.Is(err, errAgentDown) ||
		(err == nil && rep.IsError && (strings.HasPrefix(rep.Text, "ブラウザのスイッチがオフ") || strings.HasPrefix(rep.Text, "ブラウザのスイッチが「見るだけ」")))
	if short {
		asked, need := askFor(kind, tool, actionOf(tool), in)
		if need != "" {
			return errResult("ブラウザのスイッチが足りません。" + need), nil, nil
		}
		if !asked {
			if errors.Is(err, errAgentDown) {
				return errResult("ブラウザのスイッチがオフです（常駐役が動いていない）。カオルにスイッチの窓で「ブラウザ」を「見るだけ」か「操作も可」にしてもらってください。切り替えはカオルだけ"), nil, nil
			}
			return toResult(rep), nil, nil
		}
		ok, why := waitKaoru(kind, 60*time.Second, func() bool {
			if bst.Mode() < want {
				return false
			}
			_, e := callAgent(ctx, "pc_status", noIn{}, 5*time.Second)
			return e == nil
		})
		if !ok {
			return errResult("ブラウザのスイッチが足りません。" + why), nil, nil
		}
		rep, err = callAgent(ctx, tool, in, timeout)
	}
	if err == nil && rep.IsError && strings.Contains(rep.Text, "カオルの許可が要ります") {
		st := NewState(stateDir(), 0)
		st.revokers = []string{bst.path("mode.txt")}
		if asked, need := askFor("pc-allow", tool, allowWhat(tool, rep), in); need != "" {
			rep.Text += "\n（" + need + "）"
		} else if asked {
			if ok, w := waitKaoru("pc-allow", 90*time.Second, func() bool { ok, _ := st.Allowed(); return ok }); ok {
				rep, err = callAgent(ctx, tool, in, timeout)
			} else {
				rep.Text += "\n（" + w + "）"
			}
		}
	}
	if err != nil {
		return errResult(err.Error()), nil, nil
	}
	return toResult(rep), nil, nil
}

type noIn struct{}

type diskIn2 struct {
	whyArg
	Path    string `json:"path,omitempty" jsonschema:"大きさを調べるフォルダ（例 %USERPROFILE% や C:\\）。省くとドライブの空きと、膨らみやすい場所だけ"`
	Top     int    `json:"top,omitempty" jsonschema:"大きい順に何個出すか（既定 20）"`
	Files   bool   `json:"files,omitempty" jsonschema:"true なら、50 MB 以上の大きいファイルも出す"`
	Known   *bool  `json:"known,omitempty" jsonschema:"膨らみやすい場所（一時ファイル・キャッシュ・iPhone のバックアップなど）も調べるか。path を省いたときは既定で調べる"`
	Seconds int    `json:"seconds,omitempty" jsonschema:"調べる時間の上限（既定 120 秒）"`
}

type problemsIn struct {
	whyArg
	Hours       int    `json:"hours,omitempty" jsonschema:"何時間前から見るか（既定 24）"`
	App         string `json:"app,omitempty" jsonschema:"アプリの名前の一部で絞る（例 chrome、Claude）"`
	Warnings    bool   `json:"warnings,omitempty" jsonschema:"true なら警告も出す（既定はエラーだけ）"`
	Reliability bool   `json:"reliability,omitempty" jsonschema:"true なら信頼性モニターの記録も出す（少し遅い）"`
	Max         int    `json:"max,omitempty" jsonschema:"出す件数（既定 40）"`
}

type processesIn struct {
	whyArg
	Sort string `json:"sort,omitempty" jsonschema:"memory（既定・同じ名前はまとめる）か cpu（1 秒測る）"`
	Name string `json:"name,omitempty" jsonschema:"名前の一部で絞る"`
	Top  int    `json:"top,omitempty" jsonschema:"何個出すか（既定 20）"`
}

type appsIn struct {
	whyArg
	Kind   string `json:"kind,omitempty" jsonschema:"installed（入っているアプリ・既定）か startup（起動時に動くもの）"`
	Filter string `json:"filter,omitempty" jsonschema:"名前・作った会社の一部で絞る"`
	Sort   string `json:"sort,omitempty" jsonschema:"name（既定）・size・date"`
	Store  bool   `json:"store,omitempty" jsonschema:"true ならストアのアプリも出す"`
	Max    int    `json:"max,omitempty"`
}

type filesIn2 struct {
	whyArg
	Action   string `json:"action,omitempty" jsonschema:"list（一覧・既定）・read（中身）・find（名前で探す）・info（詳しく）"`
	Path     string `json:"path" jsonschema:"ファイルかフォルダ（%LOCALAPPDATA% などの書き方も使える）"`
	Pattern  string `json:"pattern,omitempty" jsonschema:"read：この正規表現に合う行だけ／find：名前の形（*.log など）"`
	Head     int    `json:"head,omitempty" jsonschema:"read：先頭から何行（既定 200）"`
	Tail     int    `json:"tail,omitempty" jsonschema:"read：末尾から何行（ログを見るとき）"`
	Depth    int    `json:"depth,omitempty" jsonschema:"find：何段下まで（既定 6）"`
	Sort     string `json:"sort,omitempty" jsonschema:"list / find：name・new（新しい順）・big（大きい順）"`
	Max      int    `json:"max,omitempty" jsonschema:"出す数の上限（既定 200）"`
	MaxBytes int    `json:"max_bytes,omitempty" jsonschema:"read：読む量の上限（既定 20 MB）"`
}

type shotIn struct {
	whyArg
	Window   string `json:"window,omitempty" jsonschema:"窓の題名の一部・プログラム名・pid。省くと画面全体"`
	Activate bool   `json:"activate,omitempty" jsonschema:"true なら、撮る前にその窓を前に出す"`
	Full     bool   `json:"full,omitempty" jsonschema:"true なら縮めない"`
	MaxWidth int    `json:"max_width,omitempty" jsonschema:"縮めたあとの幅の上限（既定 1280）"`
}

type windowsIn struct {
	whyArg
	All bool `json:"all,omitempty" jsonschema:"true なら隠れている窓も"`
}

type uiIn struct {
	whyArg
	Window string `json:"window,omitempty" jsonschema:"窓の題名の一部・プログラム名・pid。省くといま前の窓"`
	Filter string `json:"filter,omitempty" jsonschema:"名前・id・値の一部で絞る"`
	Depth  int    `json:"depth,omitempty" jsonschema:"何段下まで（既定 8）"`
	Max    int    `json:"max,omitempty" jsonschema:"出す数（既定 300）"`
	All    bool   `json:"all,omitempty" jsonschema:"true なら画面の外の部品も"`
}

type psIn2 struct {
	whyArg
	Script   string `json:"script" jsonschema:"PowerShell の台本（Windows PowerShell 5.1）"`
	Seconds  int    `json:"seconds,omitempty" jsonschema:"時間の上限（既定 60・最大 600）"`
	MaxChars int    `json:"max_chars,omitempty" jsonschema:"出力の文字数の上限（既定 30000）"`
	Real     bool   `json:"real,omitempty" jsonschema:"true で、何かを変える台本も本物の PC で動かす（頼まれた作業で本物を変える必要があるときだけ。許可・控え・確かめは今までどおり）。既定 false：変える台本は箱（ClaudeBox）の中で動かす"`
}

type openIn struct {
	whyArg
	Target string `json:"target" jsonschema:"開くもの：アプリ（notepad、explorer など）・ファイル・フォルダ・URL"`
	Args   string `json:"args,omitempty" jsonschema:"アプリに渡す引数"`
}

type uiDoIn struct {
	whyArg
	Window    string  `json:"window,omitempty" jsonschema:"窓の題名の一部・プログラム名・pid（前に出してから操作する）"`
	Target    string  `json:"target,omitempty" jsonschema:"部品の名前か id（pc_ui の「」の中か id=）"`
	Type      string  `json:"type,omitempty" jsonschema:"部品の種類で絞る（Button、Edit など）"`
	Partial   bool    `json:"partial,omitempty" jsonschema:"true なら名前の一部が合えばよい"`
	Index     int     `json:"index,omitempty" jsonschema:"同じ名前が複数あるときの番号（0 から）"`
	Action    string  `json:"action" jsonschema:"click・double・right・set_value・focus・toggle・expand・collapse・select・scroll_into_view・click_xy・double_xy・right_xy"`
	Value     string  `json:"value,omitempty" jsonschema:"set_value のときに入れる文字"`
	X         float64 `json:"x,omitempty" jsonschema:"click_xy などの横の位置（画面の画素。from_image なら画像の座標）"`
	Y         float64 `json:"y,omitempty"`
	FromImage bool    `json:"from_image,omitempty" jsonschema:"true なら x, y は最後に撮った pc_screenshot の画像の座標"`
}

type keysIn struct {
	whyArg
	Window string `json:"window,omitempty" jsonschema:"先に前に出す窓（省くといま前の窓に打つ）"`
	Text   string `json:"text,omitempty" jsonschema:"そのまま打つ文字"`
	Paste  bool   `json:"paste,omitempty" jsonschema:"true なら text をクリップボード経由で貼る（日本語はこちらが確実。前のクリップボードは戻す）"`
	Keys   string `json:"keys,omitempty" jsonschema:"特別なキー（SendKeys の書き方：^s＝Ctrl+S、%{F4}＝Alt+F4、{TAB}、{ESC} など）"`
	Enter  bool   `json:"enter,omitempty" jsonschema:"true なら最後に Enter"`
}

type trashIn struct {
	whyArg
	Paths   []string `json:"paths" jsonschema:"消すファイル・フォルダ（取り置きに 30 日写してから元を消す。ごみ箱は通さない）"`
	Recycle bool     `json:"recycle,omitempty" jsonschema:"true なら取り置きに写さず、ごみ箱だけに送る（取り置きに入らない大きいもの。先にカオルに聞いてから）"`
}

type tryIn struct {
	whyArg
	Action  string   `json:"action,omitempty" jsonschema:"run（既定：箱で動かす）／show（いまの試し・変わった行・最後の画面の文字）／offer（カオルの TRY の窓に出す）／discard（捨てる：箱を空にする）。本物に入れるのは TRY の窓（カオル）だけ"`
	Path    string   `json:"path,omitempty" jsonschema:"run：動かすファイル（exe・bat・ps1 など。devkit\\downloads のものは Defender で調べてから。本物の PC のどこのものでもよいが、本物は変わらない）"`
	Script  string   `json:"script,omitempty" jsonschema:"run：箱の中で動かす PowerShell の台本（path の代わり。Claude が作った台本を試すとき）"`
	Args    []string `json:"args,omitempty" jsonschema:"run：ファイル・台本に渡す引数"`
	Seconds int      `json:"seconds,omitempty" jsonschema:"run：終わるのを待つ秒数（既定 60・最大 110。過ぎても箱の中では動いたまま）"`
	Offer   *bool    `json:"offer,omitempty" jsonschema:"run：終わったらすぐ TRY の窓に出すか（既定：devkit\\downloads のファイルは true・ほかは false＝Claude が続けて試す）"`
	Lines   int      `json:"lines,omitempty" jsonschema:"run・show：画面の文字を何行出すか（run 既定 40・show 既定 200）"`
	ID      string   `json:"id,omitempty" jsonschema:"discard：試しの番号（省くといまの試し）"`
}

type handsIn struct {
	whyArg
	Action string `json:"action" jsonschema:"start（作業を始める。title が要る）／done（作業を終えて DONE の報告）／log（TIMELINE と作業の一覧。既定）／undo（task の作業で Claude が変えた・消した分を戻す。操作も可）"`
	Title  string `json:"title,omitempty" jsonschema:"start の題名（何をする作業か。カオルが読んで分かる言葉で）"`
	Task   string `json:"task,omitempty" jsonschema:"作業の番号（log・undo）"`
	Max    int    `json:"max,omitempty" jsonschema:"log の行数（既定 40）"`
}

func runMCP() {
	s := mcp.NewServer(&mcp.Implementation{Name: "pc", Version: version}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "pc_status", Description: "PC の道具の様子：常駐役が動いているか・スイッチ（オフ・見るだけ・操作も可）・許可（ALLOW）・C の空き・取り置き・毎日の点検（復元ポイント）・OneDrive への写し・最近の記録。いつでも使える"}, forward[noIn]("pc_status", 15*time.Second))
	mcp.AddTool(s, &mcp.Tool{Name: "pc_disk", Description: "容量を調べる：ドライブの空き、フォルダの中の大きい順、大きいファイル、膨らみやすい場所（一時ファイル・キャッシュ・Windows Update・iPhone のバックアップ・Docker/WSL など）。見るだけで使える"}, forward[diskIn2]("pc_disk", 11*time.Minute))
	mcp.AddTool(s, &mcp.Tool{Name: "pc_problems", Description: "アプリや Windows の不具合の記録を集める：イベントログのエラー、落ちた・固まったアプリの回数、エラー報告（WER）、クラッシュのダンプ、信頼性モニター。見るだけで使える"}, forward[problemsIn]("pc_problems", 4*time.Minute))
	mcp.AddTool(s, &mcp.Tool{Name: "pc_processes", Description: "動いているプログラムをメモリか CPU の重い順に。見るだけで使える"}, forward[processesIn]("pc_processes", 2*time.Minute))
	mcp.AddTool(s, &mcp.Tool{Name: "pc_apps", Description: "入っているアプリ（版・会社・入れた日・大きさ）か、PC の起動時に動くもの。見るだけで使える"}, forward[appsIn]("pc_apps", 2*time.Minute))
	mcp.AddTool(s, &mcp.Tool{Name: "pc_files", Description: "ファイルを見る：一覧・中身（先頭／末尾／行を正規表現で絞る。UTF-8・Shift_JIS・UTF-16 を見分ける）・名前で探す・詳しく。パスワード・セキュリティ・お金にかかわる場所の中身はカオルの許可が要る。見るだけで使える"}, forward[filesIn2]("pc_files", 2*time.Minute))
	mcp.AddTool(s, &mcp.Tool{Name: "pc_screenshot", Description: "画面全体か 1 つの窓を撮る（縮めた JPEG）。返事に、画像の座標と画面の座標の換算が付く。見るだけで使える"}, forward[shotIn]("pc_screenshot", 90*time.Second))
	mcp.AddTool(s, &mcp.Tool{Name: "pc_windows", Description: "開いている窓の一覧（題名・プログラム・pid・位置。いま前の窓に印）。見るだけで使える"}, forward[windowsIn]("pc_windows", 90*time.Second))
	mcp.AddTool(s, &mcp.Tool{Name: "pc_ui", Description: "窓の中の部品（ボタン・入力欄・文字など）を名前・id・値・位置で一覧にする（Windows の UI オートメーション）。押す前に使う。見るだけで使える"}, forward[uiIn]("pc_ui", 2*time.Minute))
	mcp.AddTool(s, &mcp.Tool{Name: "pc_powershell", Description: "PowerShell の台本を実行する。読むだけの台本（Get- など）は本物の PC でそのまま（開きっぱなしなので速い）。何かを変える台本（書く・消す・入れる・外への通信など）は、既定で箱（ClaudeBox）の中で動かす：本物は変わらず、返事に終わりのコード・画面の文字・増えた／変わったファイルが出る（pc_try の作業場と同じ箱。続けて試せる・本物に入れたいものは pc_try action=offer でカオルの TRY の窓へ）。箱ではパスワード・鍵・証明書にかかわるものだけカオルの許可が要る。real: true で本物の PC を変える（頼まれた作業で本物を変える必要があるときだけ。パスワード・セキュリティ・お金・完全に消す（Remove-Item）・外への通信・レジストリなどはカオルの許可が要る。ごみ箱は pc_trash）。「見るだけ」では読む台本だけ"}, forward[psIn2]("pc_powershell", 11*time.Minute))
	mcp.AddTool(s, &mcp.Tool{Name: "pc_open", Description: "アプリ・ファイル・フォルダ・URL を開く。操作も可のときだけ"}, forward[openIn]("pc_open", 90*time.Second))
	mcp.AddTool(s, &mcp.Tool{Name: "pc_ui_do", Description: "窓の部品を名前で探して押す・文字を入れる・切り替える・開く（UI オートメーション）。位置で押す（click_xy、pc_screenshot の画像の座標でもよい）もできる。操作も可のときだけ"}, forward[uiDoIn]("pc_ui_do", 90*time.Second))
	mcp.AddTool(s, &mcp.Tool{Name: "pc_keys", Description: "キーボードで打つ（文字・特別なキー・Enter）。window で先に窓を前に出せる。操作も可のときだけ"}, forward[keysIn]("pc_keys", 90*time.Second))
	mcp.AddTool(s, &mcp.Tool{Name: "pc_trash", Description: "ファイル・フォルダを消す（完全には消さない）：取り置き（30 日。Claude には取り出せない・消せない。戻すのはカオル）に写して確かめてから、元を消す（ごみ箱は通さない。C を 2 回使わない）。取り置きに入らない大きいものは止まるので、カオルに聞いてから recycle: true。Windows 本体・パスワードなどの場所はカオルの許可が要る。消す前にカオルに確かめる決まり。操作も可のときだけ"}, forward[trashIn]("pc_trash", 6*time.Minute))
	mcp.AddTool(s, &mcp.Tool{Name: "pc_try", Description: "箱で試す（Claude の作業場）：ClaudeBox（ネットは許可したサイトだけ・管理者なし・鍵などは見えない）の中で、ファイルか PowerShell の台本を動かす。本物の PC は何も変わらない。返事に、終わりのコード・画面の文字とエラー・増えた／変わった（前 → 後の行）／消えたファイルが出る。だめなら直して、もう一度 run（箱はそのまま続く）。本物に入れたいものができたら action=offer でカオルの TRY の窓に出す（本物に入れるかはカオルが決める。Claude からは入れられない）。試しだけなら action=discard。devkit\\downloads のもの（ネットから来たもの）は Defender で調べてから動かし、すぐ TRY の窓に出す。それらを箱の外で動かすのは止まるので、これを使う。操作も可のときだけ"}, forward[tryIn]("pc_try", 4*time.Minute))
	mcp.AddTool(s, &mcp.Tool{Name: "hands", Description: "作業のまとまり：PC・ブラウザ・iPhone の操作を 1 本の記録（TIMELINE）に残し、作業ごとに区切る。PC やファイルを変える作業は、始める前に action=start（題名）、終わったら action=done（記録から「何を変えたか」の報告）。action=undo で、その作業で Claude が変えた文字のファイル（決まったフォルダ）と消したもの（取り置き）だけを戻す（あとで変わったものは戻さない）。action=log で最近の記録"}, forward[handsIn]("hands", 10*time.Minute))
	// ブラウザ（カオルのいつもの Chrome。画面を撮らずに文字と番号で読んで動かす）
	mcp.AddTool(s, &mcp.Tool{Name: "browser_tabs", Description: "Chrome のタブの一覧（番号・題名・URL）。action=new で URL を新しいタブで開く・select で前に出す・close で Claude が開いたタブを閉じる（一覧は見るだけで使える、ほかは操作も可）。ブラウザの道具は既定で Claude 用の Chrome（profile=claude。道具が起動する・カオルのログインは無い・ログインやお金のページは開かない）を使う。カオルのいつもの Chrome は、カオルに頼まれたときだけ profile=kaoru（初めてつなぐとき Chrome が「許可しますか？」を出す）"}, forward[brTabsIn]("browser_tabs", 2*time.Minute))
	mcp.AddTool(s, &mcp.Tool{Name: "browser_read", Description: "Chrome のページを文字で読む（画像より速く正確）：押せる部品（ボタン・リンク・入力欄）に番号を付けた一覧と、ページの文字。filter で絞れる。番号は browser_do で使う。見るだけで使える"}, forward[brReadIn]("browser_read", 2*time.Minute))
	mcp.AddTool(s, &mcp.Tool{Name: "browser_do", Description: "Chrome のページで何手もまとめて動かす（1 回の呼び出しで）：click（番号・text=…・css=…）・double・click_xy・type・select・key・scroll・wait・wait_for・goto・back・js、分かれ道（if）・繰り返し（repeat）・確かめ（check）。押すのは本物のマウスの合図なので canvas も押せる。終わったら部品の一覧を返す（read で変えられる）。パスワード・カードの欄には打たない。銀行・決済・ログインなどのページはカオルの許可が要る。操作も可のときだけ"}, forward[brDoIn]("browser_do", 6*time.Minute))
	mcp.AddTool(s, &mcp.Tool{Name: "browser_js", Description: "Chrome のページの中で JavaScript を動かして、値を返す（コンソールと同じ書き方・await 可）。表を全部読む・canvas のゲームの中の見えている数字を読む、など。クッキー・保存領域・パスワードにかかわるものはカオルの許可が要る。操作も可のときだけ"}, forward[brJSIn]("browser_js", 3*time.Minute))
	mcp.AddTool(s, &mcp.Tool{Name: "browser_screenshot", Description: "Chrome のページの見えている所（か部品 1 つ）を撮る（縮めた JPEG）。文字で読めない所（canvas・画像）を見るときだけ使う。返事に画像の座標 → click_xy の座標の換算が付く。見るだけで使える"}, forward[brShotIn]("browser_screenshot", 2*time.Minute))
	_ = s.Run(context.Background(), &mcp.StdioTransport{})
}
