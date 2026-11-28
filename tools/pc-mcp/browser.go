package main

// ブラウザの道具（browser_*）：カオルのいつもの Chrome を、画面を撮らずに文字と番号で読んで動かす。
//   - 読む：ページの部品（ボタン・リンク・入力欄）に番号を付けた一覧と、ページの文字
//   - 動かす：何手もまとめて 1 回で送る（押す・打つ・選ぶ・キー・待つ・移る）。押すのは本物のマウスの合図なので canvas も押せる
// スイッチは PC と別の「ブラウザ」（見るだけ＝読む・撮る・タブの一覧、操作も可＝押す・打つ・JavaScript・タブを開く）。
// 許可（PC-ALLOW・10 分）は PC と共通。
// 守り：パスワード欄・カード欄には許可があっても打たない。銀行・決済・ログインなどのページの操作と、
// クッキーや保存領域を触る JavaScript はカオルの許可（PC-ALLOW）が要る。chrome:// などの設定のページには行かない。

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 操作に許可が要るページ（題名・URL で判断）
var sensitivePageRe = regexp.MustCompile(`(?i)(bank|銀行|証券|信用金庫|信金|paypay|paypal|楽天ペイ|ペイディ|クレジット|credit ?card|wallet|ウォレット|crypto|bitcoin|決済|お支払い|支払い方法|checkout|購入手続き|注文を確定|password|パスワード|\bsign ?in\b|サインイン|ログイン|\blog ?in\b|2段階|二段階|認証コード|two-factor|2fa|accounts\.google\.com|appleid\.apple\.com|account\.apple\.com|myaccount|マイナ|e-?tax|確定申告|chrome web store)`)

// 行かない・動かさない場所
var blockedURLRe = regexp.MustCompile(`(?i)^(chrome|edge|brave|opera|vivaldi|chrome-extension|devtools|chrome-search|chrome-untrusted|view-source|javascript):`)

// 許可が要る場所（PC の中のファイルを開く）
var fileURLRe = regexp.MustCompile(`(?i)^file:`)

// 許可が要る JavaScript（クッキー・保存領域・パスワード）
var jsAllowRe = regexp.MustCompile(`(?i)(document\.cookie|cookiestore|localstorage|sessionstorage|indexeddb|caches\.|navigator\.credentials|passwordcredential|\.password\b|type\s*=\s*["']?password|\[type=["']?password)`)

const dkLibVersion = "3"

// ページの中に置く小さな道具箱（番号・名前・役・値・伏せる欄の見分け）
const dkLib = `var DK=(window.__DKL&&window.__DKL.v==='` + dkLibVersion + `')?window.__DKL:(window.__DKL=(()=>{
const SEL='a[href],button,input,select,textarea,summary,[role=button],[role=link],[role=checkbox],[role=radio],[role=tab],[role=menuitem],[role=menuitemcheckbox],[role=menuitemradio],[role=option],[role=switch],[role=combobox],[role=textbox],[role=searchbox],[role=slider],[role=spinbutton],[contenteditable=""],[contenteditable="true"],[onclick],[tabindex]:not([tabindex="-1"])';
const clean=s=>String(s==null?'':s).replace(/\s+/g,' ').trim();
const vis=e=>{const r=e.getBoundingClientRect(); if(r.width<1||r.height<1) return false; const s=getComputedStyle(e); return s.visibility!=='hidden'&&s.display!=='none'&&+s.opacity!==0;};
const sens=e=>{if(!e||!e.getAttribute) return false; const t=(e.type||'').toLowerCase(),ac=(e.getAttribute('autocomplete')||'').toLowerCase(),n=((e.name||'')+' '+(e.id||'')+' '+(e.getAttribute('aria-label')||'')).toLowerCase(); return t==='password'||/password|cc-|one-time-code/.test(ac)||/passw|パスワード|暗証|cvc|cvv|cardnum|card-?number|creditcard|カード番号|セキュリティコード|securitycode|pincode/.test(n);};
const role=e=>{const r=e.getAttribute('role'); if(r) return r; const t=e.tagName.toLowerCase(); if(t==='a') return 'link'; if(t==='input'){const ty=(e.type||'text').toLowerCase(); return ({checkbox:'checkbox',radio:'radio',submit:'button',button:'button',reset:'button',image:'button',range:'slider',file:'file'})[ty]||('input:'+ty);} if(t==='select') return 'select'; if(t==='textarea') return 'textarea'; if(e.isContentEditable) return 'textbox'; return t;};
const nm=e=>{const t=e.tagName.toLowerCase(); let s=e.getAttribute('aria-label')||''; if(!s&&e.labels&&e.labels.length) s=e.labels[0].innerText; if(!s&&e.getAttribute('aria-labelledby')){const l=document.getElementById(e.getAttribute('aria-labelledby').split(' ')[0]); if(l) s=l.innerText;} if(!s&&!(t==='input'||t==='textarea'||t==='select')) s=e.innerText; if(!s&&t==='input'&&/submit|button|reset/.test(e.type)) s=e.value; if(!s) s=e.getAttribute('placeholder')||e.getAttribute('title')||''; if(!s){const i=e.querySelector&&e.querySelector('img[alt]'); if(i) s=i.alt;} if(!s&&t==='a') s=(e.getAttribute('href')||'').slice(0,60); return clean(s).slice(0,80);};
const val=e=>{const t=e.tagName.toLowerCase(); if(sens(e)) return e.value?'（入力あり・伏せる）':'（空）'; if(t==='select') return clean(e.options[e.selectedIndex]&&e.options[e.selectedIndex].text); if(e.type==='checkbox'||e.type==='radio') return e.checked?'✓':'□'; if(t==='input'||t==='textarea') return clean(e.value).slice(0,60); if(e.getAttribute('aria-checked')) return e.getAttribute('aria-checked')==='true'?'✓':'□'; if(e.getAttribute('aria-selected')==='true') return '選択中'; return '';};
const resolve=t=>{ if(typeof t==='number'){const e=(window.__dk||[])[t]; if(!e||!e.isConnected) throw new Error('番号 '+t+' の部品がもうありません。browser_read で読み直してください'); return e;}
 t=String(t); if(t.startsWith('css=')){const e=document.querySelector(t.slice(4)); if(!e) throw new Error('見つかりません：'+t); return e;}
 const q=clean(t.startsWith('text=')?t.slice(5):t).toLowerCase(); let best=null,bl=1e9;
 for(const e of document.querySelectorAll(SEL)){ if(!vis(e)) continue; const n=nm(e).toLowerCase(); if(!n) continue; if(n===q) return e; if(n.includes(q)&&n.length<bl){best=e;bl=n.length;} }
 if(best) return best; throw new Error('見つかりません：'+t+'（browser_read で番号を見てください）'); };
return {v:'` + dkLibVersion + `',SEL,clean,vis,sens,role,nm,val,resolve};})());
`

type brTabsIn struct {
	whyArg
	browserArg
	Action string `json:"action,omitempty" jsonschema:"list（タブの一覧・既定）・new（url を新しいタブで開く）・select（tab を前に出す）・close（Claude が開いたタブを閉じる）"`
	Tab    string `json:"tab,omitempty" jsonschema:"タブ：一覧の id の頭（4 文字以上）か、題名・URL の一部。並び順の番号は使えない"`
	URL    string `json:"url,omitempty" jsonschema:"new で開く URL"`
}

type brReadIn struct {
	whyArg
	browserArg
	Tab      string `json:"tab,omitempty" jsonschema:"タブ（省くと最後に使ったタブか、いま見えているタブ）"`
	Mode     string `json:"mode,omitempty" jsonschema:"elements（押せる部品の一覧・既定）・text（ページの文字）・all（両方）"`
	Filter   string `json:"filter,omitempty" jsonschema:"この文字を含む行だけ（大文字小文字は区別しない）"`
	Max      int    `json:"max,omitempty" jsonschema:"部品の数の上限（既定 200。画面の中のものが先）"`
	MaxChars int    `json:"max_chars,omitempty" jsonschema:"文字の上限（既定 8000）"`
}

type brDoIn struct {
	whyArg
	browserArg
	Tab   string           `json:"tab,omitempty" jsonschema:"タブ（省くと最後に使ったタブか、いま見えているタブ）"`
	Steps []map[string]any `json:"steps" jsonschema:"順にやる手。1 つずつ {\"click\": 12} のように書く。click（番号・\"text=文字\"・\"css=セレクタ\"）・double（同じ）・click_xy [x,y]（ページの見えている所の座標）・type {\"ref\":3,\"text\":\"…\",\"clear\":true,\"enter\":true}（ref を省くと今の欄）・select {\"ref\":5,\"value\":\"文字か値\"}・key \"Enter\"・\"Ctrl+A\"・scroll 600（下へ・負で上へ・\"top\"・\"bottom\"）・wait 500（ミリ秒・10000 まで）・wait_for \"文字\" か {\"text\":…,\"css\":…,\"timeout\":ms}・goto \"URL\"・back true・js \"式\"（最後の値を返す）。分かれ道・繰り返し・確かめ：if {\"text\"か\"css\":…,\"wait\":ms,\"then\":[手…],\"else\":[手…]}（いまページにあるか）・repeat {\"times\":N（50 まで）,\"until\":{\"text\"か\"css\":…},\"steps\":[手…]}（until になるまで。ならなければ止まる）・check {\"text\"か\"css\":…,\"absent\":true}（あるはず・ないはずを確かめ、違えば止まる）。繰り返しを含めて手は 300 まで"`
	Read  string           `json:"read,omitempty" jsonschema:"終わったあとに読む：elements（既定）・text・all・none"`
	Front bool             `json:"front,omitempty" jsonschema:"true なら先にタブを前に出す（ゲームなど、見えていないと止まるページ）"`
	Max   int              `json:"max,omitempty" jsonschema:"終わったあとの部品の数の上限（既定 120）"`
}

type brJSIn struct {
	whyArg
	browserArg
	Tab     string `json:"tab,omitempty"`
	Code    string `json:"code" jsonschema:"ページの中で動かす JavaScript（式の値を返す。await も使える。JSON にできる値で返す）"`
	Seconds int    `json:"seconds,omitempty" jsonschema:"時間の上限（既定 20・最大 120）"`
}

type brShotIn struct {
	whyArg
	browserArg
	Tab      string `json:"tab,omitempty"`
	Target   string `json:"target,omitempty" jsonschema:"部品 1 つだけ撮るとき：番号・text=…・css=…（省くと見えている所全体）"`
	MaxWidth int    `json:"max_width,omitempty" jsonschema:"縮めたあとの幅の上限（既定 1000）"`
	Quality  int    `json:"quality,omitempty" jsonschema:"JPEG の質（既定 70）"`
}

// browser：この呼び出しの Chrome（ctx の profile。claude＝Claude 用・kaoru＝カオルの）
func (a *Agent) browser(ctx context.Context) *Browser {
	a.brMu.Lock()
	defer a.brMu.Unlock()
	if profileOf(ctx) == profKaoru {
		if a.br == nil {
			a.br = &Browser{profile: profKaoru, opened: map[string]bool{}}
		}
		return a.br
	}
	if a.brc == nil {
		a.brc = &Browser{profile: profClaude, opened: map[string]bool{}}
	}
	return a.brc
}

// browsers：つないでいる Chrome（閉じる・様子を見るとき）
func (a *Agent) browsers() []*Browser {
	a.brMu.Lock()
	defer a.brMu.Unlock()
	var out []*Browser
	for _, b := range []*Browser{a.brc, a.br} {
		if b != nil {
			out = append(out, b)
		}
	}
	return out
}

// pickTab は、指定（番号・文字・id）からタブを選ぶ。省くと最後に使ったタブ、無ければ見えているタブ。
func (b *Browser) pickTab(ctx context.Context, spec string) (targetInfo, error) {
	pages, err := b.pages(ctx)
	if err != nil {
		return targetInfo{}, err
	}
	if len(pages) == 0 {
		return targetInfo{}, fmt.Errorf("Chrome にタブがありません")
	}
	spec = strings.TrimSpace(spec)
	if spec == "" {
		b.mu.Lock()
		cur := b.current
		b.mu.Unlock()
		for _, p := range pages {
			if p.TargetID == cur {
				return p, nil
			}
		}
		// いま見えているタブを探す
		for _, p := range pages {
			c, sid, err := b.session(ctx, p.TargetID)
			if err != nil {
				continue
			}
			var v bool
			cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			_ = evalIn(cctx, c, sid, `document.visibilityState==='visible'`, &v)
			cancel()
			if v {
				b.setCurrent(p.TargetID)
				return p, nil
			}
		}
		b.setCurrent(pages[0].TargetID)
		return pages[0], nil
	}
	// 並び順の番号は使わない：カオルがタブを開け閉めすると変わり、別のタブを動かしてしまう（2026-11-26 に一度あった）
	if _, err := strconv.Atoi(spec); err == nil && len(spec) <= 3 {
		return targetInfo{}, fmt.Errorf("タブは番号ではなく id の頭（一覧の id）か、題名・URL の一部で指定してください（並び順はカオルがタブを開け閉めすると変わるため）")
	}
	if len(spec) >= 4 {
		for _, p := range pages {
			if strings.HasPrefix(strings.ToLower(p.TargetID), strings.ToLower(spec)) {
				b.setCurrent(p.TargetID)
				return p, nil
			}
		}
	}
	ls := strings.ToLower(spec)
	for _, p := range pages {
		if strings.Contains(strings.ToLower(p.Title), ls) || strings.Contains(strings.ToLower(p.URL), ls) {
			b.setCurrent(p.TargetID)
			return p, nil
		}
	}
	return targetInfo{}, fmt.Errorf("タブ「%s」が見つかりません（browser_tabs で一覧を見てください）", spec)
}

func (b *Browser) setCurrent(id string) {
	b.mu.Lock()
	b.current = id
	b.mu.Unlock()
}

func shortTitle(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return string(r)
}

// pageNeedsAllow は、そのページを操作するのに許可が要るか（要るなら理由）。
func pageNeedsAllow(t targetInfo) string {
	if blockedURLRe.MatchString(t.URL) {
		return ""
	}
	if m := sensitivePageRe.FindString(t.Title + " " + t.URL); m != "" {
		return "パスワード・お金・本人確認にかかわりそうなページ（" + m + "）"
	}
	if fileURLRe.MatchString(t.URL) {
		return "PC の中のファイルのページ"
	}
	return ""
}

func (a *Agent) allowed() bool { ok, _ := a.st.Allowed(); return ok }

// needAllowErr：許可が要って止めた（窓の WHAT 付き）
type needAllowErr struct{ msg, what string }

func (e needAllowErr) Error() string { return e.msg }

// pageWhat：許可が要るページの WHAT（URL や題名はそのまま出さず、合った言葉だけ）
func pageWhat(t targetInfo, verb string) string {
	if m := sensitivePageRe.FindString(t.Title + " " + t.URL); m != "" {
		return "「" + m + "」にかかわりそうなページ" + verb
	}
	if fileURLRe.MatchString(t.URL) {
		return "PC の中のファイルのページ" + verb
	}
	return "許可が要るページ" + verb
}

func (a *Agent) brCheckOperate(ctx context.Context, t targetInfo) *Reply {
	if blockedURLRe.MatchString(t.URL) {
		r := errText("このページ（%s）は Chrome の設定・拡張機能などのページなので、操作しません", shortTitle(t.URL, 60))
		return &r
	}
	if r := claudeBlocked(ctx, t); r != nil {
		return r
	}
	if why := pageNeedsAllow(t); why != "" && !a.allowed() {
		r := errText("%s", allowMsg(why+"の操作。ページ："+shortTitle(t.Title, 40))).needs(pageWhat(t, "を操作する（押す・打つ）"))
		return &r
	}
	return nil
}

func (a *Agent) brTabs(ctx context.Context, raw json.RawMessage) Reply {
	in, err := decode[brTabsIn](raw)
	if err != nil {
		return errText("引数がおかしい：%v", err)
	}
	if ctx, err = withProfile(ctx, in.Profile); err != nil {
		return errText("%v", err)
	}
	b := a.browser(ctx)
	act := strings.ToLower(strings.TrimSpace(in.Action))
	if act == "" {
		act = "list"
	}
	if act != "list" && a.bst.Mode() != ModeOperate {
		return errText("ブラウザのスイッチが「見るだけ」なので操作できません。要るならカオルにスイッチの窓で「ブラウザ」を「操作も可」にしてもらってください")
	}
	switch act {
	case "new":
		u := strings.TrimSpace(in.URL)
		if u == "" {
			u = "about:blank"
		}
		if !strings.Contains(u, ":") {
			u = "https://" + u
		}
		if blockedURLRe.MatchString(u) {
			return errText("%s は Chrome の設定・拡張機能などのページなので開きません", u)
		}
		if r := claudeBlocked(ctx, targetInfo{URL: u}); r != nil {
			return *r
		}
		if why := pageNeedsAllow(targetInfo{URL: u}); why != "" && !a.allowed() {
			return errText("%s", allowMsg(why)).needs(pageWhat(targetInfo{URL: u}, "を開く"))
		}
		c, err := b.conn(ctx)
		if err != nil {
			return errText("%v", err)
		}
		var r struct {
			TargetID string `json:"targetId"`
		}
		// 空のタブを作ってから、つないで移る（作るときに URL を渡すと、読み込みが始まらないことがあった：Windows の headless）
		params := map[string]any{"url": "about:blank"}
		if b.profile == profClaude {
			params["background"] = true // Claude 用の Chrome は、タブを開いても窓を前に出さない（カオルの窓のフォーカスを取らない）
			ps, _ := b.pages(ctx)
			if len(ps) == 0 {
				params["newWindow"] = true // 窓を全部閉じたあと（Chrome は窓なしで動き続ける）
				defer func() { go sendBack(b.pid) }()
			}
			// 窓にまだ空のページ 1 枚しかないなら、それを使う（後ろで開いたタブは表に出ないので、空のページが表のままになる。2026-11-27）
			if len(ps) == 1 && ps[0].URL == "about:blank" {
				b.mu.Lock()
				mine := b.opened[ps[0].TargetID]
				b.mu.Unlock()
				if !mine {
					r.TargetID = ps[0].TargetID
				}
			}
		}
		if r.TargetID == "" {
			if err := c.Call(ctx, "", "Target.createTarget", params, &r); err != nil {
				return errText("タブを開けません：%v", err)
			}
		}
		b.mu.Lock()
		b.opened[r.TargetID] = true
		b.mu.Unlock()
		b.setCurrent(r.TargetID)
		t := targetInfo{TargetID: r.TargetID, URL: u}
		if u != "about:blank" {
			cs, sid, err := b.session(ctx, r.TargetID)
			if err != nil {
				return errText("%v", err)
			}
			var nr struct {
				ErrorText string `json:"errorText"`
			}
			if err := cs.Call(ctx, sid, "Page.navigate", map[string]any{"url": u}, &nr); err != nil {
				return errText("開けません：%v", err)
			}
			if nr.ErrorText != "" {
				return errText("開けません：%s", nr.ErrorText)
			}
			_ = a.brWaitLoad(ctx, t, 20*time.Second)
		}
		return a.brList(ctx, "開きました："+u)
	case "select":
		t, err := b.pickTab(ctx, in.Tab)
		if err != nil {
			return errText("%v", err)
		}
		c, _ := b.conn(ctx)
		if err := c.Call(ctx, "", "Target.activateTarget", map[string]any{"targetId": t.TargetID}, nil); err != nil {
			return errText("前に出せません：%v", err)
		}
		return a.brList(ctx, "前に出しました："+shortTitle(t.Title, 40))
	case "close":
		t, err := b.pickTab(ctx, in.Tab)
		if err != nil {
			return errText("%v", err)
		}
		b.mu.Lock()
		mine := b.opened[t.TargetID] || b.profile == profClaude // Claude 用の Chrome のタブは全部 Claude のもの
		b.mu.Unlock()
		if !mine && !a.allowed() {
			return errText("このタブ（%s）はカオルが開いたものなので閉じません（Claude が開いたタブだけ閉じる）。%s", shortTitle(t.Title, 40), "要るならカオルに閉じてもらうか、PC-ALLOW の間に頼んでください")
		}
		c, _ := b.conn(ctx)
		if err := c.Call(ctx, "", "Target.closeTarget", map[string]any{"targetId": t.TargetID}, nil); err != nil {
			return errText("閉じられません：%v", err)
		}
		b.mu.Lock()
		delete(b.opened, t.TargetID)
		delete(b.sessions, t.TargetID)
		if b.current == t.TargetID {
			b.current = ""
		}
		b.mu.Unlock()
		return a.brList(ctx, "閉じました："+shortTitle(t.Title, 40))
	case "list":
		return a.brList(ctx, "")
	}
	return errText("action は list・new・select・close のどれか")
}

func (a *Agent) brList(ctx context.Context, head string) Reply {
	b := a.browser(ctx)
	pages, err := b.pages(ctx)
	if err != nil {
		return errText("%v", err)
	}
	b.mu.Lock()
	cur := b.current
	b.mu.Unlock()
	var lines []string
	if head != "" {
		lines = append(lines, head)
	}
	lines = append(lines, fmt.Sprintf("Chrome のタブ（%d 個。★＝道具がいま使うタブ、＋＝Claude が開いたタブ。指定は id か題名で）：", len(pages)))
	for _, p := range pages {
		mark := "  "
		if p.TargetID == cur {
			mark = "★ "
		}
		b.mu.Lock()
		if b.opened[p.TargetID] {
			mark += "＋"
		}
		b.mu.Unlock()
		lines = append(lines, fmt.Sprintf("・id %s %s%s — %s", p.TargetID[:8], mark, shortTitle(p.Title, 50), shortTitle(p.URL, 90)))
	}
	return okText(strings.Join(lines, "\n"))
}

type readResult struct {
	Title  string   `json:"title"`
	URL    string   `json:"url"`
	SX     float64  `json:"sx"`
	SY     float64  `json:"sy"`
	H      float64  `json:"h"`
	VW     float64  `json:"vw"`
	VH     float64  `json:"vh"`
	Lines  []string `json:"lines"`
	More   int      `json:"more"`
	Text   string   `json:"text"`
	Frames int      `json:"frames"`
	Canvas int      `json:"canvas"`
}

func readJS(withEls, withText bool, filter string, max int) string {
	o, _ := json.Marshal(map[string]any{"els": withEls, "text": withText, "filter": strings.ToLower(filter), "max": max})
	return dkLib + `((o)=>{const D=DK; const vw=innerWidth,vh=innerHeight; let lines=[],more=0,text='';
if(o.els){ const items=[]; for(const e of document.querySelectorAll(D.SEL)){ if(e.type==='hidden'||!D.vis(e)) continue; if(e.closest('[aria-hidden="true"]')) continue; const r=e.getBoundingClientRect(); items.push({e,r,inv:(r.bottom>0&&r.right>0&&r.top<vh&&r.left<vw)?1:0}); }
 items.sort((a,b)=>(b.inv-a.inv)||(Math.round(a.r.top)-Math.round(b.r.top))||(a.r.left-b.r.left));
 const keep=[]; for(const it of items){ const e=it.e; const ro=D.role(e), n=D.nm(e), v=D.val(e); let l=ro+' 「'+n+'」'; if(v) l+=' = '+v; if(e.disabled||e.getAttribute('aria-disabled')==='true') l+=' （押せない）'; if(D.sens(e)) l+=' （パスワード・カードの欄：Claude は打たない）'; if(!it.inv) l+=' （画面の外）'; if(ro==='link'){const h=e.getAttribute('href')||''; if(h&&!/^javascript:/i.test(h)) l+=' → '+h.slice(0,90);} if(o.filter&&!l.toLowerCase().includes(o.filter)) continue; if(keep.length>=o.max){more++; continue;} lines.push('['+keep.length+'] '+l); keep.push(e);} window.__dk=keep; }
if(o.text){ let raw=(document.body?document.body.innerText:'').split('\n').map(D.clean).filter(Boolean); if(o.filter) raw=raw.filter(x=>x.toLowerCase().includes(o.filter)); const out=[]; for(const x of raw){ if(out[out.length-1]!==x) out.push(x);} text=out.join('\n'); }
return {title:document.title,url:location.href,sx:scrollX,sy:scrollY,h:document.documentElement.scrollHeight,vw,vh,lines,more,text,frames:document.querySelectorAll('iframe').length,canvas:[...document.querySelectorAll('canvas')].filter(c=>{const r=c.getBoundingClientRect(); return r.width>50&&r.height>50;}).length};})(` + string(o) + `)`
}

func formatRead(r readResult, withEls, withText bool, maxChars int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "ページ：%s\nURL：%s\n見えている所：横 %.0f×縦 %.0f、上から %.0f／全体の高さ %.0f\n", shortTitle(r.Title, 80), r.URL, r.VW, r.VH, r.SY, r.H)
	if r.Canvas > 0 {
		fmt.Fprintf(&sb, "絵で描かれた所（canvas）が %d 個：中身は文字では読めない（browser_screenshot か browser_js で）\n", r.Canvas)
	}
	if r.Frames > 0 {
		fmt.Fprintf(&sb, "iframe が %d 個（中の部品は一覧に出ない）\n", r.Frames)
	}
	if withEls {
		fmt.Fprintf(&sb, "部品（番号で押す・打つ。番号は次に読むまで有効）：\n")
		sb.WriteString(strings.Join(r.Lines, "\n"))
		if len(r.Lines) == 0 {
			sb.WriteString("（なし）")
		}
		if r.More > 0 {
			fmt.Fprintf(&sb, "\n…ほかに %d 個（filter で絞るか max を増やす）", r.More)
		}
		sb.WriteString("\n")
	}
	if withText {
		t := r.Text
		rs := []rune(t)
		if len(rs) > maxChars {
			t = string(rs[:maxChars]) + fmt.Sprintf("\n…（ここまで %d 文字。全部で %d 文字。filter で絞るか max_chars を増やす）", maxChars, len(rs))
		}
		sb.WriteString("文字：\n" + t + "\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (a *Agent) brRead(ctx context.Context, raw json.RawMessage) Reply {
	in, err := decode[brReadIn](raw)
	if err != nil {
		return errText("引数がおかしい：%v", err)
	}
	if ctx, err = withProfile(ctx, in.Profile); err != nil {
		return errText("%v", err)
	}
	b := a.browser(ctx)
	t, err := b.pickTab(ctx, in.Tab)
	if err != nil {
		return errText("%v", err)
	}
	if blockedURLRe.MatchString(t.URL) {
		return errText("このページ（%s）は Chrome の設定・拡張機能などのページなので読みません", shortTitle(t.URL, 60))
	}
	c, sid, err := b.session(ctx, t.TargetID)
	if err != nil {
		return errText("%v", err)
	}
	mode := strings.ToLower(in.Mode)
	withEls, withText := true, false
	switch mode {
	case "text":
		withEls, withText = false, true
	case "all":
		withText = true
	}
	if in.Max <= 0 {
		in.Max = 200
	}
	if in.MaxChars <= 0 {
		in.MaxChars = 8000
	}
	var r readResult
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := evalIn(cctx, c, sid, readJS(withEls, withText, in.Filter, in.Max), &r); err != nil {
		return errText("読めません：%v", err)
	}
	return okText(formatRead(r, withEls, withText, in.MaxChars))
}

func (a *Agent) brJS(ctx context.Context, raw json.RawMessage) Reply {
	in, err := decode[brJSIn](raw)
	if err != nil {
		return errText("引数がおかしい：%v", err)
	}
	if ctx, err = withProfile(ctx, in.Profile); err != nil {
		return errText("%v", err)
	}
	if strings.TrimSpace(in.Code) == "" {
		return errText("code が空です")
	}
	if m := jsAllowRe.FindString(in.Code); m != "" && !a.allowed() {
		return errText("%s", allowMsg("クッキー・保存領域・パスワードにかかわる JavaScript（"+m+"）")).needs("ページのクッキー・保存領域・パスワードを読む・変える（JavaScript）")
	}
	b := a.browser(ctx)
	t, err := b.pickTab(ctx, in.Tab)
	if err != nil {
		return errText("%v", err)
	}
	if r := a.brCheckOperate(ctx, t); r != nil {
		return *r
	}
	c, sid, err := b.session(ctx, t.TargetID)
	if err != nil {
		return errText("%v", err)
	}
	sec := in.Seconds
	if sec <= 0 {
		sec = 20
	}
	if sec > 120 {
		sec = 120
	}
	cctx, cancel := context.WithTimeout(ctx, time.Duration(sec)*time.Second)
	defer cancel()
	var v any
	start := time.Now()
	if err := evalRepl(cctx, c, sid, in.Code, &v); err != nil {
		return errText("%v", err)
	}
	out, _ := json.MarshalIndent(v, "", " ")
	s := string(out)
	if len(s) > 30000 {
		s = s[:30000] + "\n…（長いので切りました）"
	}
	return okTextf("%s\n（%s・%.2f 秒）", s, shortTitle(t.Title, 40), time.Since(start).Seconds())
}

func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// --- マウス・キー（本物の合図として Chrome に送る）---

func mouseClick(ctx context.Context, c *CDP, sid string, x, y float64, count int) error {
	if err := c.Call(ctx, sid, "Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": x, "y": y}, nil); err != nil {
		return err
	}
	for i := 1; i <= count; i++ {
		p := map[string]any{"x": x, "y": y, "button": "left", "clickCount": i, "buttons": 1}
		p["type"] = "mousePressed"
		if err := c.Call(ctx, sid, "Input.dispatchMouseEvent", p, nil); err != nil {
			return err
		}
		p["type"] = "mouseReleased"
		p["buttons"] = 0
		if err := c.Call(ctx, sid, "Input.dispatchMouseEvent", p, nil); err != nil {
			return err
		}
	}
	return nil
}

var keyCodes = map[string]struct {
	code int
	key  string
	text string
}{
	"enter": {13, "Enter", "\r"}, "tab": {9, "Tab", ""}, "escape": {27, "Escape", ""}, "esc": {27, "Escape", ""},
	"backspace": {8, "Backspace", ""}, "delete": {46, "Delete", ""}, "space": {32, " ", " "},
	"arrowup": {38, "ArrowUp", ""}, "arrowdown": {40, "ArrowDown", ""}, "arrowleft": {37, "ArrowLeft", ""}, "arrowright": {39, "ArrowRight", ""},
	"up": {38, "ArrowUp", ""}, "down": {40, "ArrowDown", ""}, "left": {37, "ArrowLeft", ""}, "right": {39, "ArrowRight", ""},
	"home": {36, "Home", ""}, "end": {35, "End", ""}, "pageup": {33, "PageUp", ""}, "pagedown": {34, "PageDown", ""},
	"f5": {116, "F5", ""},
}

func pressKey(ctx context.Context, c *CDP, sid, spec string) error {
	parts := strings.Split(spec, "+")
	mods := 0
	for _, m := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(m)) {
		case "alt":
			mods |= 1
		case "ctrl", "control":
			mods |= 2
		case "meta", "cmd", "win":
			mods |= 4
		case "shift":
			mods |= 8
		default:
			return fmt.Errorf("知らない修飾キー：%s", m)
		}
	}
	k := strings.TrimSpace(parts[len(parts)-1])
	code, key, text := 0, k, ""
	if kc, ok := keyCodes[strings.ToLower(k)]; ok {
		code, key, text = kc.code, kc.key, kc.text
	} else if r := []rune(k); len(r) == 1 {
		u := strings.ToUpper(k)
		code = int([]rune(u)[0])
		key = k
		text = k
		if mods&8 != 0 {
			text = u
		}
	} else {
		return fmt.Errorf("知らないキー：%s", k)
	}
	if mods&(1|2|4) != 0 {
		text = "" // 修飾キーつきは文字を入れない
	}
	down := map[string]any{"type": "keyDown", "modifiers": mods, "windowsVirtualKeyCode": code, "key": key}
	if text != "" {
		down["text"] = text
		down["unmodifiedText"] = text
	} else {
		down["type"] = "rawKeyDown"
	}
	if err := c.Call(ctx, sid, "Input.dispatchKeyEvent", down, nil); err != nil {
		return err
	}
	return c.Call(ctx, sid, "Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "modifiers": mods, "windowsVirtualKeyCode": code, "key": key}, nil)
}

// パスワード・カードの欄にいるときでも押してよいキー（文字を入れない・貼らない）
var safeKeyRe = regexp.MustCompile(`(?i)^(shift\+)?(tab|enter|escape|esc|arrowup|arrowdown|arrowleft|arrowright|up|down|left|right|home|end|pageup|pagedown)$`)

// --- まとめて動かす ---

type elemPos struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Name string  `json:"name"`
	Role string  `json:"role"`
	Sens bool    `json:"sens"`
}

func refArg(v any) (string, error) {
	switch x := v.(type) {
	case float64:
		return strconv.Itoa(int(x)), nil
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(x)); err == nil {
			return strconv.Itoa(n), nil
		}
		return jsString(x), nil
	case nil:
		return "null", nil
	}
	return "", fmt.Errorf("部品の指定は番号か文字（text=…・css=…）")
}

func refLabel(v any) string {
	switch x := v.(type) {
	case float64:
		return fmt.Sprintf("[%d]", int(x))
	case string:
		return "「" + shortTitle(x, 30) + "」"
	}
	return ""
}

func (a *Agent) brWaitLoad(ctx context.Context, t targetInfo, d time.Duration) error {
	b := a.browser(ctx)
	c, sid, err := b.session(ctx, t.TargetID)
	if err != nil {
		return err
	}
	end := time.Now().Add(d)
	time.Sleep(150 * time.Millisecond)
	for time.Now().Before(end) {
		var s string
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := evalIn(cctx, c, sid, `document.readyState+' '+location.href`, &s)
		cancel()
		if err == nil && strings.HasPrefix(s, "complete ") && (t.URL == "about:blank" || !strings.HasSuffix(s, " about:blank")) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("読み込みが %0.f 秒で終わりませんでした（そのまま続けます）", d.Seconds())
}

func (b *Browser) refresh(ctx context.Context, t targetInfo) targetInfo {
	c, err := b.conn(ctx)
	if err != nil {
		return t
	}
	var r struct {
		TargetInfo targetInfo `json:"targetInfo"`
	}
	if c.Call(ctx, "", "Target.getTargetInfo", map[string]any{"targetId": t.TargetID}, &r) == nil && r.TargetInfo.TargetID != "" {
		return r.TargetInfo
	}
	return t
}

func (a *Agent) brDo(ctx context.Context, raw json.RawMessage) Reply {
	in, err := decode[brDoIn](raw)
	if err != nil {
		return errText("引数がおかしい：%v", err)
	}
	if ctx, err = withProfile(ctx, in.Profile); err != nil {
		return errText("%v", err)
	}
	if len(in.Steps) == 0 {
		return errText("steps が空です")
	}
	if len(in.Steps) > 60 {
		return errText("steps は 60 個まで")
	}
	b := a.browser(ctx)
	t, err := b.pickTab(ctx, in.Tab)
	if err != nil {
		return errText("%v", err)
	}
	if r := a.brCheckOperate(ctx, t); r != nil {
		return *r
	}
	c, sid, err := b.session(ctx, t.TargetID)
	if err != nil {
		return errText("%v", err)
	}
	start := time.Now()
	var log []string
	if in.Front {
		_ = c.Call(ctx, "", "Target.activateTarget", map[string]any{"targetId": t.TargetID}, nil)
		_ = c.Call(ctx, sid, "Page.bringToFront", nil, nil)
	}
	r := &brRun{a: a, b: b, c: c, sid: sid, t: t}
	if rep := r.run(ctx, in.Steps, ""); rep != nil {
		rep.Text = a.brDoTail(ctx, r.t, r.log, start, "none", 0)
		return *rep
	}
	t, log = r.t, r.log
	if in.Max <= 0 {
		in.Max = 120
	}
	return okText(a.brDoTail(ctx, t, log, start, in.Read, in.Max))
}

// brRun：browser_do の手を順に動かす（if・repeat・check の中の手も同じ道を通る）
type brRun struct {
	a     *Agent
	b     *Browser
	c     *CDP
	sid   string
	t     targetInfo
	log   []string
	count int
}

const brMaxSteps = 300 // 繰り返しを含めた手の合計の上限（止まらない台本を作らない）

func (r *brRun) fail(label, msg string, err error) *Reply {
	r.log = append(r.log, fmt.Sprintf("%s ✗ %s", label, msg))
	r.log = append(r.log, "（ここで止めました。前の手は済んでいます）")
	rep := &Reply{IsError: true}
	var na needAllowErr
	if errors.As(err, &na) {
		rep.What = na.what
	}
	var ee eErr
	if errors.As(err, &ee) {
		rep.Lane = "E"
	}
	return rep
}

func (r *brRun) run(ctx context.Context, steps []map[string]any, prefix string) *Reply {
	for i, st := range steps {
		label := fmt.Sprintf("%s%d.", prefix, i+1)
		if r.count > 0 {
			r.t = r.b.refresh(ctx, r.t)
			if rep := r.a.brCheckOperate(ctx, r.t); rep != nil {
				return r.fail(label, rep.Text, nil)
			}
		}
		r.count++
		if r.count > brMaxSteps {
			return r.fail(label, fmt.Sprintf("手が %d を超えました（繰り返しの数を見直す）", brMaxSteps), nil)
		}
		if len(st) != 1 {
			keys := make([]string, 0, len(st))
			for k := range st {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return r.fail(label, "1 つの手には 1 つの動きだけ書いてください："+strings.Join(keys, ","), nil)
		}
		var kind string
		var v any
		for k, x := range st {
			kind, v = strings.ToLower(k), x
		}
		switch kind {
		case "if":
			m, _ := v.(map[string]any)
			ok, what, err := r.cond(ctx, m)
			if err != nil {
				return r.fail(label, "if "+err.Error(), err)
			}
			branch, word := "then", "ある"
			if !ok {
				branch, word = "else", "ない"
			}
			sub, err := subSteps(m[branch])
			if err != nil {
				return r.fail(label, "if の "+branch+"："+err.Error(), err)
			}
			r.log = append(r.log, fmt.Sprintf("%s if 「%s」→ %s（%s の %d 手）", label, what, word, branch, len(sub)))
			if rep := r.run(ctx, sub, label); rep != nil {
				return rep
			}
		case "repeat":
			m, _ := v.(map[string]any)
			sub, err := subSteps(m["steps"])
			if err != nil || len(sub) == 0 {
				return r.fail(label, "repeat の steps（繰り返す手）が要ります", err)
			}
			times := 10.0
			if f, ok := m["times"].(float64); ok {
				times = f
			}
			times = math.Max(1, math.Min(times, 50))
			until, _ := m["until"].(map[string]any)
			if s, ok := m["until"].(string); ok {
				until = map[string]any{"text": s}
			}
			met := false
			n := 0
			for ; n < int(times); n++ {
				if until != nil {
					ok, _, err := r.cond(ctx, until)
					if err != nil {
						return r.fail(label, "repeat の until "+err.Error(), err)
					}
					if ok {
						met = true
						break
					}
				}
				if rep := r.run(ctx, sub, fmt.Sprintf("%s%d-", label, n+1)); rep != nil {
					return rep
				}
			}
			if until != nil && !met {
				if ok, _, _ := r.cond(ctx, until); ok {
					met = true
				}
			}
			if until != nil && !met {
				return r.fail(label, fmt.Sprintf("repeat：%d 回くり返しても「%s」になりませんでした", n, condWhat(until)), nil)
			}
			r.log = append(r.log, fmt.Sprintf("%s repeat %d 回%s", label, n, map[bool]string{true: "（" + condWhat(until) + " になった）", false: ""}[until != nil]))
		case "check":
			m, _ := v.(map[string]any)
			if s, ok := v.(string); ok {
				m = map[string]any{"text": s}
			}
			ok, what, err := r.cond(ctx, m)
			if err != nil {
				return r.fail(label, "check "+err.Error(), err)
			}
			absent, _ := m["absent"].(bool)
			if ok == absent {
				want := "ある"
				if absent {
					want = "ない"
				}
				return r.fail(label, fmt.Sprintf("check 「%s」が%sはずなのに違った（ページの形が変わったかもしれない）", what, want), nil)
			}
			r.log = append(r.log, fmt.Sprintf("%s check 「%s」→ OK", label, what))
		default:
			cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			msg, err := r.a.brStep(cctx, r.c, r.sid, &r.t, kind, v)
			cancel()
			if err != nil {
				return r.fail(label, kind+" "+err.Error(), err)
			}
			r.log = append(r.log, label+" "+msg)
		}
	}
	return nil
}

// cond：いまのページに文字・部品があるか（wait ミリ秒まで待ってよい。10 秒まで）
func (r *brRun) cond(ctx context.Context, m map[string]any) (bool, string, error) {
	text, _ := m["text"].(string)
	css, _ := m["css"].(string)
	if text == "" && css == "" {
		return false, "", fmt.Errorf("は {\"text\":…} か {\"css\":…} で書く")
	}
	wait, _ := m["wait"].(float64)
	end := time.Now().Add(time.Duration(math.Max(0, math.Min(wait, 10000))) * time.Millisecond)
	expr := fmt.Sprintf(`(()=>{const t=%s,c=%s; if(c&&document.querySelector(c)) return true; if(t&&document.body&&document.body.innerText.includes(t)) return true; return false;})()`, jsString(text), jsString(css))
	for {
		var ok bool
		err := evalIn(ctx, r.c, r.sid, expr, &ok)
		if err == nil && ok {
			return true, condWhat(m), nil
		}
		if !time.Now().Before(end) || ctx.Err() != nil {
			if err != nil {
				return false, condWhat(m), err
			}
			return false, condWhat(m), nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func condWhat(m map[string]any) string {
	text, _ := m["text"].(string)
	css, _ := m["css"].(string)
	if text != "" && css != "" {
		return text + "／" + css
	}
	return text + css
}

// subSteps：if・repeat の中の手（JSON の配列）を手の並びにする
func subSteps(v any) ([]map[string]any, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("手の並び（[{…},{…}]）で書く")
	}
	return out, nil
}

func (a *Agent) brDoTail(ctx context.Context, t targetInfo, log []string, start time.Time, read string, max int) string {
	b := a.browser(ctx)
	t = b.refresh(ctx, t)
	head := fmt.Sprintf("タブ：%s — %s\n%s\n（手の合計 %.2f 秒）", shortTitle(t.Title, 50), shortTitle(t.URL, 90), strings.Join(log, "\n"), time.Since(start).Seconds())
	read = strings.ToLower(read)
	if read == "" {
		read = "elements"
	}
	if read == "none" || max == 0 {
		return head
	}
	withEls, withText := read != "text", read == "text" || read == "all"
	c, sid, err := b.session(ctx, t.TargetID)
	if err != nil {
		return head
	}
	var r readResult
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := evalIn(cctx, c, sid, readJS(withEls, withText, "", max), &r); err != nil {
		return head + "\n（終わったあとに読めませんでした：" + err.Error() + "）"
	}
	return head + "\n\n" + formatRead(r, withEls, withText, 6000)
}

func (a *Agent) brStep(ctx context.Context, c *CDP, sid string, t *targetInfo, kind string, v any) (string, error) {
	switch kind {
	case "click", "double", "dblclick":
		ref, err := refArg(v)
		if err != nil {
			return "", err
		}
		var p elemPos
		if err := evalIn(ctx, c, sid, dkLib+`((t)=>{const e=DK.resolve(t); e.scrollIntoView({block:'center',inline:'center',behavior:'instant'}); const r=e.getBoundingClientRect(); return {x:r.left+r.width/2,y:r.top+r.height/2,name:DK.nm(e),role:DK.role(e),sens:DK.sens(e)};})(`+ref+`)`, &p); err != nil {
			return "", err
		}
		if w := finalWord(p.Name); w != "" {
			return "", eErr{"ページの「" + shortTitle(p.Name, 30) + "」", w}
		}
		n := 1
		if kind != "click" {
			n = 2
		}
		if err := mouseClick(ctx, c, sid, p.X, p.Y, n); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s %s %s「%s」", kind, refLabel(v), p.Role, shortTitle(p.Name, 40)), nil
	case "click_xy":
		xy, ok := v.([]any)
		if !ok || len(xy) != 2 {
			return "", fmt.Errorf("click_xy は [x, y]")
		}
		x, _ := xy[0].(float64)
		y, _ := xy[1].(float64)
		var nm string
		_ = evalIn(ctx, c, sid, dkLib+fmt.Sprintf(`(()=>{let e=document.elementFromPoint(%g,%g); while(e&&!/^(BUTTON|A|INPUT)$/.test(e.tagName)&&e.getAttribute&&e.getAttribute('role')!=='button') e=e.parentElement; return e?DK.nm(e):'';})()`, x, y), &nm)
		if w := finalWord(nm); w != "" {
			return "", eErr{"ページの「" + shortTitle(nm, 30) + "」", w}
		}
		if err := mouseClick(ctx, c, sid, x, y, 1); err != nil {
			return "", err
		}
		return fmt.Sprintf("click_xy (%.0f, %.0f)", x, y), nil
	case "type":
		var ref any
		text, clear, enter := "", false, false
		switch x := v.(type) {
		case string:
			text = x
		case map[string]any:
			ref = x["ref"]
			text, _ = x["text"].(string)
			clear, _ = x["clear"].(bool)
			enter, _ = x["enter"].(bool)
		default:
			return "", fmt.Errorf("type は \"文字\" か {\"ref\":…,\"text\":…}")
		}
		rs, err := refArg(ref)
		if err != nil {
			return "", err
		}
		var p struct {
			Sens bool   `json:"sens"`
			Name string `json:"name"`
		}
		if err := evalIn(ctx, c, sid, dkLib+`((t,clear)=>{const e=t===null?document.activeElement:DK.resolve(t); if(!e||e===document.body) throw new Error('入力欄が選ばれていません（ref を付けてください）'); if(DK.sens(e)) return {sens:true,name:DK.nm(e)}; e.scrollIntoView({block:'center',behavior:'instant'}); e.focus(); if(clear){ if(typeof e.select==='function') e.select(); else document.execCommand('selectAll'); } return {sens:false,name:DK.nm(e)};})(`+rs+`,`+strconv.FormatBool(clear)+`)`, &p); err != nil {
			return "", err
		}
		if p.Sens {
			return "", fmt.Errorf("「%s」はパスワード・カードの欄なので打ちません（カオルが入れる決まり）", p.Name)
		}
		if enter {
			if w := a.enterRisk(ctx, c, sid); w != "" {
				return "", eErr{"この欄での Enter（" + w + "）", "Enter"}
			}
		}
		if text == "" && clear {
			if err := pressKey(ctx, c, sid, "Backspace"); err != nil {
				return "", err
			}
		} else if err := c.Call(ctx, sid, "Input.insertText", map[string]any{"text": text}, nil); err != nil {
			return "", err
		}
		if enter {
			if err := pressKey(ctx, c, sid, "Enter"); err != nil {
				return "", err
			}
		}
		e := ""
		if enter {
			e = " ＋ Enter"
		}
		return fmt.Sprintf("type %s「%s」← %q%s", refLabel(ref), shortTitle(p.Name, 30), shortTitle(text, 40), e), nil
	case "select":
		x, ok := v.(map[string]any)
		if !ok {
			return "", fmt.Errorf("select は {\"ref\":…,\"value\":…}")
		}
		rs, err := refArg(x["ref"])
		if err != nil {
			return "", err
		}
		val := fmt.Sprint(x["value"])
		var got string
		if err := evalIn(ctx, c, sid, dkLib+`((t,v)=>{const e=DK.resolve(t); if(e.tagName!=='SELECT') throw new Error('選ぶ欄（select）ではありません'); const o=[...e.options].find(o=>o.value===v)||[...e.options].find(o=>DK.clean(o.text)===v)||[...e.options].find(o=>DK.clean(o.text).includes(v)); if(!o) throw new Error('選べるものに「'+v+'」がありません：'+[...e.options].slice(0,20).map(o=>DK.clean(o.text)).join(' / ')); e.value=o.value; e.dispatchEvent(new Event('input',{bubbles:true})); e.dispatchEvent(new Event('change',{bubbles:true})); return DK.clean(o.text);})(`+rs+`,`+jsString(val)+`)`, &got); err != nil {
			return "", err
		}
		return fmt.Sprintf("select %s ← 「%s」", refLabel(x["ref"]), got), nil
	case "key":
		k, _ := v.(string)
		var sens bool
		_ = evalIn(ctx, c, sid, dkLib+`DK.sens(document.activeElement)`, &sens)
		if sens && !safeKeyRe.MatchString(k) {
			return "", fmt.Errorf("いまの欄はパスワード・カードの欄なので、%s は押しません（Tab・Enter・Esc・矢印だけ）", k)
		}
		if strings.Contains(strings.ToLower(k), "enter") {
			if w := a.enterRisk(ctx, c, sid); w != "" {
				return "", eErr{"この欄での Enter（" + w + "）", "Enter"}
			}
		}
		if err := pressKey(ctx, c, sid, k); err != nil {
			return "", err
		}
		return "key " + k, nil
	case "scroll":
		var expr string
		switch x := v.(type) {
		case float64:
			expr = fmt.Sprintf("scrollBy(0,%d)", int(x))
		case string:
			switch strings.ToLower(x) {
			case "top":
				expr = "scrollTo(0,0)"
			case "bottom":
				expr = "scrollTo(0,document.documentElement.scrollHeight)"
			default:
				return "", fmt.Errorf("scroll は数・\"top\"・\"bottom\"")
			}
		default:
			return "", fmt.Errorf("scroll は数・\"top\"・\"bottom\"")
		}
		var y float64
		if err := evalIn(ctx, c, sid, expr+";scrollY", &y); err != nil {
			return "", err
		}
		return fmt.Sprintf("scroll → 上から %.0f", y), nil
	case "wait":
		ms, _ := v.(float64)
		ms = math.Max(0, math.Min(ms, 10000))
		select {
		case <-time.After(time.Duration(ms) * time.Millisecond):
		case <-ctx.Done():
		}
		return fmt.Sprintf("wait %.0f ms", ms), nil
	case "wait_for":
		text, css := "", ""
		to := 10000.0
		switch x := v.(type) {
		case string:
			text = x
		case map[string]any:
			text, _ = x["text"].(string)
			css, _ = x["css"].(string)
			if f, ok := x["timeout"].(float64); ok {
				to = math.Min(f, 25000)
			}
		}
		if text == "" && css == "" {
			return "", fmt.Errorf("wait_for は \"文字\" か {\"text\":…} か {\"css\":…}")
		}
		end := time.Now().Add(time.Duration(to) * time.Millisecond)
		expr := fmt.Sprintf(`(()=>{const t=%s,c=%s; if(c&&document.querySelector(c)) return true; if(t&&document.body&&document.body.innerText.includes(t)) return true; return false;})()`, jsString(text), jsString(css))
		for {
			var ok bool
			if err := evalIn(ctx, c, sid, expr, &ok); err == nil && ok {
				return fmt.Sprintf("wait_for 「%s%s」→ 出た", text, css), nil
			}
			if time.Now().After(end) {
				return "", fmt.Errorf("「%s%s」が %.0f 秒で出ませんでした", text, css, to/1000)
			}
			time.Sleep(200 * time.Millisecond)
		}
	case "goto":
		u, _ := v.(string)
		u = strings.TrimSpace(u)
		if u == "" {
			return "", fmt.Errorf("goto の URL が空")
		}
		if !strings.Contains(u, ":") {
			u = "https://" + u
		}
		if blockedURLRe.MatchString(u) {
			return "", fmt.Errorf("%s は Chrome の設定・拡張機能などのページなので行きません", u)
		}
		if r := claudeBlocked(ctx, targetInfo{URL: u}); r != nil {
			return "", errors.New(r.Text)
		}
		if why := pageNeedsAllow(targetInfo{URL: u}); why != "" && !a.allowed() {
			return "", needAllowErr{allowMsg(why), pageWhat(targetInfo{URL: u}, "を開く")}
		}
		var r struct {
			ErrorText string `json:"errorText"`
		}
		if err := c.Call(ctx, sid, "Page.navigate", map[string]any{"url": u}, &r); err != nil {
			return "", err
		}
		if r.ErrorText != "" {
			return "", fmt.Errorf("開けません：%s", r.ErrorText)
		}
		_ = a.brWaitLoad(ctx, targetInfo{TargetID: t.TargetID, URL: u}, 20*time.Second)
		return "goto " + shortTitle(u, 80), nil
	case "back":
		if err := evalIn(ctx, c, sid, `history.back()`, nil); err != nil {
			return "", err
		}
		time.Sleep(300 * time.Millisecond)
		_ = a.brWaitLoad(ctx, *t, 15*time.Second)
		return "back", nil
	case "js":
		code, _ := v.(string)
		if m := jsAllowRe.FindString(code); m != "" && !a.allowed() {
			return "", needAllowErr{allowMsg("クッキー・保存領域・パスワードにかかわる JavaScript（" + m + "）"), "ページのクッキー・保存領域・パスワードを読む・変える（JavaScript）"}
		}
		var out any
		if err := evalRepl(ctx, c, sid, code, &out); err != nil {
			return "", err
		}
		s, _ := json.Marshal(out)
		ss := string(s)
		if len(ss) > 4000 {
			ss = ss[:4000] + "…"
		}
		return "js → " + ss, nil
	}
	return "", fmt.Errorf("知らない動き：%s（click・double・click_xy・type・select・key・scroll・wait・wait_for・goto・back・js）", kind)
}

func (a *Agent) brShot(ctx context.Context, raw json.RawMessage) Reply {
	in, err := decode[brShotIn](raw)
	if err != nil {
		return errText("引数がおかしい：%v", err)
	}
	if ctx, err = withProfile(ctx, in.Profile); err != nil {
		return errText("%v", err)
	}
	b := a.browser(ctx)
	t, err := b.pickTab(ctx, in.Tab)
	if err != nil {
		return errText("%v", err)
	}
	if blockedURLRe.MatchString(t.URL) {
		return errText("このページ（%s）は Chrome の設定・拡張機能などのページなので撮りません", shortTitle(t.URL, 60))
	}
	c, sid, err := b.session(ctx, t.TargetID)
	if err != nil {
		return errText("%v", err)
	}
	if in.MaxWidth <= 0 {
		in.MaxWidth = 1000
	}
	if in.Quality <= 0 || in.Quality > 100 {
		in.Quality = 70
	}
	var box struct {
		X, Y, W, H, SX, SY float64
	}
	if in.Target != "" {
		ref, err := refArg(in.Target)
		if err != nil {
			return errText("%v", err)
		}
		if err := evalIn(ctx, c, sid, dkLib+`((t)=>{const e=DK.resolve(t); e.scrollIntoView({block:'center',behavior:'instant'}); const r=e.getBoundingClientRect(); return {X:r.left,Y:r.top,W:r.width,H:r.height,SX:scrollX,SY:scrollY};})(`+ref+`)`, &box); err != nil {
			return errText("%v", err)
		}
	} else {
		if err := evalIn(ctx, c, sid, `({X:0,Y:0,W:innerWidth,H:innerHeight,SX:scrollX,SY:scrollY})`, &box); err != nil {
			return errText("%v", err)
		}
	}
	scale := 1.0
	if box.W > float64(in.MaxWidth) {
		scale = float64(in.MaxWidth) / box.W
	}
	var r struct {
		Data string `json:"data"`
	}
	clip := map[string]any{"x": box.X + box.SX, "y": box.Y + box.SY, "width": box.W, "height": box.H, "scale": scale}
	if err := c.Call(ctx, sid, "Page.captureScreenshot", map[string]any{"format": "jpeg", "quality": in.Quality, "clip": clip, "captureBeyondViewport": false}, &r); err != nil {
		return errText("撮れません：%v", err)
	}
	img, err := base64.StdEncoding.DecodeString(r.Data)
	if err != nil {
		return errText("撮った画像が読めません：%v", err)
	}
	txt := fmt.Sprintf("%s — %s\n画像は %.0f×%.0f（縮め %.3f）。画像の座標 (ix, iy) → ページの見えている所の座標は (%.1f + ix ÷ %.3f, %.1f + iy ÷ %.3f)。click_xy にはこちらを使う",
		shortTitle(t.Title, 40), shortTitle(t.URL, 80), box.W*scale, box.H*scale, scale, box.X, scale, box.Y, scale)
	return okText(txt).withImage(img, "image/jpeg")
}

// enterRisk：いまの欄で Enter を押すと送ってしまいそうか（チャットの欄・「送信」などのボタンがある form）。そうなら理由
func (a *Agent) enterRisk(ctx context.Context, c *CDP, sid string) string {
	var w string
	js := dkLib + `(()=>{const F=` + finalJS() + `; const e=document.activeElement; if(!e||e===document.body) return ''; if(e.isContentEditable||e.tagName==='TEXTAREA') return 'チャット・文章の欄'; const f=e.form; if(f){ for(const b of f.querySelectorAll('button,input[type=submit],input[type=image],[role=button]')){ const n=DK.nm(b); if(n&&n.length<=30&&F.test(n)) return '「'+n+'」のボタンがある入力欄'; } } return '';})()`
	if err := evalIn(ctx, c, sid, js, &w); err != nil {
		return ""
	}
	return w
}
