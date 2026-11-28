package main

// 箱で試す（B の道。docs/ASK-DESIGN.md の 2「箱」・docs/PC.md「箱」）。
//
//   run     ：devkit\downloads のものを Defender で検査 → 見つかれば止める → ClaudeBox の中で動かす
//             → 箱の中で増えた・変わった・消えたファイルをまとめる → TRY の窓（LIVE の窓の中）に出す
//   status  ：いまの試しと、カオルの返事
//   discard ：捨てる（箱を空にする）。Claude もカオルも使える（戻せないものは何も無い）
//   apply   ：本物に入れる。TRY の窓（カオル）だけ：窓が合言葉（request.key）で署名して頼む。Claude からは呼べない
//
// 箱の中身の見方（2026-11-28 に記録で確かめた）：
//   箱の根 C:\Sandbox\<ユーザー>\ClaudeBox の下の drive\C\… が C:\…、user\current\… が %USERPROFILE%\…、user\all\… が %ProgramData%\…。
//   本物のファイルを変えると、写しが箱の中にできる。消すと、大きさ 0・作成日時 1986-05-24 の印のファイルになる。
//   根の直下の RegHive などはレジストリ（入れない）。PowerShell や Windows が勝手に作るもの（CatRoot2 など）は数えるだけ。

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const tryBox = "ClaudeBox"

var tryMu sync.Mutex

func sbieDir() string {
	pf := os.Getenv("ProgramFiles")
	if pf == "" {
		pf = `C:\Program Files`
	}
	return filepath.Join(pf, "Sandboxie-Plus")
}

func boxRoot() string {
	if v := os.Getenv("HANDS_BOX_ROOT"); v != "" {
		return v
	}
	return filepath.Join(systemDrive(), "Sandbox", os.Getenv("USERNAME"), tryBox)
}

func tryDir() string       { return filepath.Join(handsUserDir(), "try") }
func tryStateFile() string { return filepath.Join(tryDir(), "current.json") }

type tryItem struct {
	Kind  string   `json:"kind"` // new・changed・deleted・admin（管理者の場所。入れない）
	Real  string   `json:"real"`
	Size  int64    `json:"size"`
	Hash  string   `json:"hash,omitempty"`  // 箱の中のファイルの SHA-256（窓に出した中身。入れるときに照らす）
	Lines []string `json:"lines,omitempty"` // 文字のファイル：変わった行（- 前・+ 後）・増えたものの先頭
}

type tryState struct {
	ID       string    `json:"id"`
	Path     string    `json:"path"`
	Args     []string  `json:"args,omitempty"`
	Started  time.Time `json:"started"`
	Ended    time.Time `json:"ended"`
	Defender string    `json:"defender"`
	Running  bool      `json:"running"` // 待ち時間を過ぎても箱の中で動いている
	Exit     string    `json:"exit,omitempty"`
	Items    []tryItem `json:"items"`
	Noise    int       `json:"noise"`    // Windows・PowerShell が勝手に作るもの（入れない）
	Registry bool      `json:"registry"` // レジストリの変更（箱の中だけ。入れない）
	Runs     []tryRun  `json:"runs,omitempty"`
	Status   string    `json:"status"` // working（Claude が試している）・waiting（TRY の窓）・applied・discarded
	Result   string    `json:"result,omitempty"`
	By       string    `json:"by,omitempty"`
	At       time.Time `json:"at,omitempty"`
}

func loadTry() (*tryState, error) {
	b, err := os.ReadFile(tryStateFile())
	if err != nil {
		return nil, err
	}
	var s tryState
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func saveTry(s *tryState) error {
	_ = os.MkdirAll(tryDir(), 0o755)
	b, _ := json.MarshalIndent(s, "", "  ")
	tmp := tryStateFile() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	saveTryHistory(s.ID, b)
	return os.Rename(tmp, tryStateFile())
}

// 試しの控え（DONE の報告に「箱で試したこと」を出すため。新しい試しで current.json が上書きされても残る）
func tryHistoryDir() string { return filepath.Join(tryDir(), "history") }

const tryHistoryKeep = 100

func saveTryHistory(id string, b []byte) {
	if id == "" {
		return
	}
	d := tryHistoryDir()
	_ = os.MkdirAll(d, 0o755)
	_ = os.WriteFile(filepath.Join(d, id+".json"), b, 0o644)
	es, err := os.ReadDir(d)
	if err != nil || len(es) <= tryHistoryKeep {
		return
	}
	var names []string
	for _, e := range es {
		if strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // 番号は時刻なので名前の順＝古い順
	for _, n := range names[:len(names)-tryHistoryKeep] {
		_ = os.Remove(filepath.Join(d, n))
	}
}

// triesBetween：この間に動きのあった試し（古い順）
func triesBetween(from, to time.Time) []*tryState {
	es, err := os.ReadDir(tryHistoryDir())
	if err != nil {
		return nil
	}
	var out []*tryState
	for _, e := range es {
		b, err := os.ReadFile(filepath.Join(tryHistoryDir(), e.Name()))
		if err != nil {
			continue
		}
		var s tryState
		if json.Unmarshal(b, &s) != nil || s.Started.After(to) {
			continue
		}
		last := s.Started
		if s.At.After(last) {
			last = s.At
		}
		for _, r := range s.Runs {
			if r.At.After(last) {
				last = r.At
			}
		}
		if last.Before(from) {
			continue
		}
		out = append(out, &s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// tryDoneLines：DONE の報告に出す「箱で試したこと」
func tryDoneLines(from, to time.Time) []string {
	ts := triesBetween(from, to)
	if len(ts) == 0 {
		return nil
	}
	l := []string{fmt.Sprintf("箱で試したこと（ClaudeBox。本物は変わらない）：%d", len(ts))}
	for _, s := range ts {
		n := map[string]int{}
		for _, it := range s.Items {
			n[it.Kind]++
		}
		ok := 0
		for _, r := range s.Runs {
			if r.Exit != nil && *r.Exit == 0 {
				ok++
			}
		}
		st := map[string]string{
			"working":   "まだ Claude が試している（カオルにはまだ見せていない）",
			"waiting":   "TRY の窓でカオルの返事待ち",
			"applied":   "本物に入れた（" + s.By + "）：" + s.Result,
			"discarded": "捨てた（本物は何も変えていない）",
		}[s.Status]
		if st == "" {
			st = s.Status
		}
		l = append(l, fmt.Sprintf("  %s：%d 回動かした（うまく終わった %d）・増えた %d・変わった %d・消えた %d → %s",
			s.ID, len(s.Runs), ok, n["new"], n["changed"], n["deleted"], st))
	}
	return l
}

// ---- 署名（TRY の窓だけが「本物に入れる」を頼める） ----

func trySignText(id, action string) string {
	return strings.Join([]string{"claude-hands-try-v1", id, action}, "\n")
}

func trySigOK(id, action, sig string) bool {
	key, err := os.ReadFile(requestKeyFile())
	k := strings.TrimSpace(string(key))
	if err != nil || k == "" || sig == "" {
		return false
	}
	m := hmac.New(sha256.New, []byte(k))
	m.Write([]byte(trySignText(id, action)))
	return hmac.Equal([]byte(hex.EncodeToString(m.Sum(nil))), []byte(sig))
}

// ---- 箱の中身を本物の場所に読み替える ----

// 勝手にできるもの（入れない・数えるだけ）。箱の根からの場所（小文字・\ 区切り）
var tryNoise = []string{
	`drive\c\windows\system32\catroot2\`,
	`user\current\appdata\roaming\microsoft\windows\recent\`,
	`user\current\appdata\local\microsoft\windows\powershell\`,
	`user\current\appdata\local\microsoft\windows\inetcache\`,
	`user\current\appdata\local\microsoft\windows\webcache\`,
	`user\current\appdata\local\microsoft\windows\explorer\`,
	`user\current\appdata\local\microsoft\windows\caches\`,
	`user\current\appdata\local\temp\`,
	`user\current\devkit\boxwork\`, // 作業用（包みの台本・画面の文字の記録）
	`user\current\appdata\local\packages\`,
	`user\current\appdata\local\connecteddevicesplatform\`,
}

func boxToReal(rel string) string {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 2 {
		return ""
	}
	rest := filepath.Join(parts[min(2, len(parts)):]...)
	switch {
	case strings.EqualFold(parts[0], "drive") && len(parts[1]) == 1 && len(parts) > 2:
		return strings.ToUpper(parts[1]) + `:\` + rest
	case strings.EqualFold(parts[0], "user") && strings.EqualFold(parts[1], "current"):
		return filepath.Join(homeDir(), rest)
	case strings.EqualFold(parts[0], "user") && strings.EqualFold(parts[1], "all"):
		return filepath.Join(programData(), rest)
	}
	return ""
}

func programData() string {
	if pd := os.Getenv("ProgramData"); pd != "" {
		return pd
	}
	return `C:\ProgramData`
}

// adminPlace：管理者しか書けない場所（本物に入れるには、本物でもう一度入れ直す。UAC）
func adminPlace(real string) bool {
	l := strings.ToLower(matchForm(real))
	return regexpAdmin.MatchString(l)
}

var regexpAdmin = regexp.MustCompile(`^[a-z]:\\(windows|program files|program files \(x86\)|programdata)(\\|$)`)

// deletedMark：箱の中で消したことの印（大きさ 0・作成日時 1986 年）
func deletedMark(fi fs.FileInfo) bool {
	return !fi.IsDir() && fi.Size() == 0 && createdAt(fi).Year() == 1986
}

// scanBox：箱の中身をまとめる（本物と同じものは数えない）
func scanBox() (items []tryItem, noise int, registry bool, err error) {
	root := boxRoot()
	if _, e := os.Stat(root); e != nil {
		return nil, 0, false, nil
	}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if rel == "." {
			return nil
		}
		low := strings.ToLower(strings.ReplaceAll(rel, "/", `\`))
		if !strings.Contains(low, `\`) && !d.IsDir() { // 根の直下（RegHive など）
			if strings.HasPrefix(low, "reghive") {
				registry = true
			}
			return nil
		}
		if d.IsDir() {
			if isReparse(d) {
				return fs.SkipDir
			}
			return nil
		}
		for _, n := range tryNoise {
			if strings.HasPrefix(low+`\`, n) || strings.HasPrefix(low, n) {
				noise++
				return nil
			}
		}
		real := boxToReal(rel)
		if real == "" {
			noise++
			return nil
		}
		fi, e2 := d.Info()
		if e2 != nil {
			return nil
		}
		if deletedMark(fi) {
			if _, e3 := os.Stat(real); e3 == nil {
				items = append(items, tryItem{Kind: "deleted", Real: real})
			}
			return nil
		}
		kind := "new"
		if rf, e3 := os.Stat(real); e3 == nil {
			if rf.Size() == fi.Size() && sameFile(p, real) {
				return nil // 開いただけで同じ
			}
			kind = "changed"
		}
		if adminPlace(real) {
			kind = "admin"
		}
		items = append(items, tryItem{Kind: kind, Real: real, Size: fi.Size(), Hash: fileHash(p)})
		return nil
	})
	sort.Slice(items, func(i, j int) bool { return items[i].Real < items[j].Real })
	return
}

func fileHash(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

func sameFile(a, b string) bool {
	ha := fileHash(a)
	return ha != "" && ha == fileHash(b)
}

// ---- Sandboxie と Defender を呼ぶ（道具の中から。台本の守りは通さない） ----

func sbie(ctx context.Context, d time.Duration, args ...string) error {
	_, err := sbieWait(ctx, d, args...)
	return err
}

// sbieStart：Sandboxie の Start.exe（試験では HANDS_SBIE_START）
func sbieStart() string {
	if v := os.Getenv("HANDS_SBIE_START"); v != "" {
		return v
	}
	return filepath.Join(sbieDir(), "Start.exe")
}

// sbieWait：Start.exe を動かす。待ち時間を過ぎたら Start.exe だけ止めて timedOut（箱の中のものは動いたまま）
func sbieWait(ctx context.Context, d time.Duration, args ...string) (timedOut bool, err error) {
	exe := sbieStart()
	c, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	cmd := exec.CommandContext(c, exe, args...)
	hideWindow(cmd)
	err = cmd.Run()
	if c.Err() == context.DeadlineExceeded && ctx.Err() == nil {
		return true, nil
	}
	return false, err
}

func emptyBox(ctx context.Context) error {
	_ = sbie(ctx, 30*time.Second, "/box:"+tryBox, "/terminate")
	if err := sbie(ctx, 60*time.Second, "/box:"+tryBox, "delete_sandbox_silent"); err != nil {
		return err
	}
	if os.Getenv("HANDS_BOX_ROOT") != "" { // 試験：箱の代わりのフォルダ
		_ = os.RemoveAll(boxRoot())
	}
	if _, err := os.Stat(boxRoot()); err == nil {
		time.Sleep(2 * time.Second)
		if _, err := os.Stat(boxRoot()); err == nil {
			return errors.New("箱の中身が残った（箱の中で何かが動いている）")
		}
	}
	return nil
}

func mpCmdRun() string {
	if v := os.Getenv("HANDS_MPCMDRUN"); v != "" {
		return v
	}
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	p := filepath.Join(pd, "Microsoft", "Windows Defender", "Platform", "CurrentVersion", "MpCmdRun.exe")
	if fileExists(p) {
		return p
	}
	pf := os.Getenv("ProgramFiles")
	if pf == "" {
		pf = `C:\Program Files`
	}
	return filepath.Join(pf, "Windows Defender", "MpCmdRun.exe")
}

// defenderScan：見つからなければ ""、見つかった・検査できなければ理由
func defenderScan(ctx context.Context, path string) (string, error) {
	c, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(c, mpCmdRun(), "-Scan", "-ScanType", "3", "-File", path, "-DisableRemediation")
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		return "", fmt.Errorf("Defender の検査を動かせない：%v", err)
	}
	switch code {
	case 0:
		return "", nil
	case 2:
		return "Defender が見つけた：" + oneLine(lastNonEmpty(string(out)), 200), nil
	}
	return "", fmt.Errorf("Defender の検査が終わりのコード %d で止まった：%s", code, oneLine(lastNonEmpty(string(out)), 200))
}

func lastNonEmpty(s string) string {
	ls := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i := len(ls) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(ls[i]); t != "" {
			return t
		}
	}
	return ""
}

// inDownloads：devkit\downloads の中か
func inDownloads(p string) bool {
	d := strings.ToLower(matchForm(downloadsDir())) + `\`
	return strings.HasPrefix(strings.ToLower(matchForm(p)), d)
}

// ---- 道具 ----

// 作業用のフォルダ（本物の側。箱から見える）：包みの台本・Claude の台本を置く。結果（画面の文字・終わりのコード）は箱の中にだけできる
func boxWorkDir() string { return filepath.Join(homeDir(), "devkit", "boxwork") }

// 包みの台本：対象を動かし、画面の文字とエラーを UTF-8 で out に、終わりのコードを code に書く（2026-11-28 に記録で確かめた形）
const boxWrap = `param([string]$target, [string]$out, [string]$code)
# ネットは中継（127.0.0.1:60230・許可したサイトだけ）を通す。箱の中の設定なので本物は変わらない
$px = 'http://127.0.0.1:60230'
foreach ($n in 'HTTP_PROXY','HTTPS_PROXY','http_proxy','https_proxy') { Set-Item -Path "env:$n" -Value $px }
[System.Net.WebRequest]::DefaultWebProxy = New-Object System.Net.WebProxy($px, $true)
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
try {
  $is = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings'
  Set-ItemProperty -Path $is -Name ProxyEnable -Value 1 -Type DWord -ErrorAction Stop
  Set-ItemProperty -Path $is -Name ProxyServer -Value '127.0.0.1:60230' -ErrorAction Stop
} catch {}
# 進み具合の表示を切る（箱の中では見えない窓に書けず Invoke-WebRequest が失敗した。2026-11-28 の記録）
$ProgressPreference = 'SilentlyContinue'
$c = 0
# 文書（.hta・.html・.xlsx など）は中継の形では動かせない（CantActivateDocumentInPipeline）ので、窓を開くだけにする
$ext = [IO.Path]::GetExtension($target).ToLower()
if ($ext -notin '.ps1', '.bat', '.cmd', '.exe', '.com') {
  try {
    $h = @{ '.hta' = "$env:SystemRoot\System32\mshta.exe"; '.txt' = "$env:SystemRoot\System32\notepad.exe"; '.log' = "$env:SystemRoot\System32\notepad.exe"; '.csv' = "$env:SystemRoot\System32\notepad.exe" }[$ext]
    if ($h) { $p = Start-Process $h -ArgumentList ('"' + $target + '"') -PassThru } else { $p = Start-Process -FilePath $target -PassThru }
    "窓を開いた：$target（pid $($p.Id)。画面を持つものなので、窓を見て確かめる）" | Out-File -LiteralPath $out -Encoding utf8
  } catch { ($_ | Out-String) | Out-File -LiteralPath $out -Encoding utf8; $c = 1 }
  Set-Content -LiteralPath $code -Value $c -Encoding ascii
  exit $c
}
try {
  & $target @args *>&1 | ForEach-Object { "$_" } | Out-File -LiteralPath $out -Encoding utf8 -Width 400
  $c = if ($null -ne $LASTEXITCODE) { $LASTEXITCODE } elseif ($?) { 0 } else { 1 }
} catch { ($_ | Out-String) | Out-File -LiteralPath $out -Append -Encoding utf8; $c = 1 }
Set-Content -LiteralPath $code -Value $c -Encoding ascii
exit $c
`

type tryRun struct {
	N       int       `json:"n"`
	What    string    `json:"what"` // 動かしたもの（ファイルか「台本」）
	At      time.Time `json:"at"`
	Exit    *int      `json:"exit,omitempty"`
	Running bool      `json:"running,omitempty"`
	Stopped bool      `json:"stopped,omitempty"` // STOP（スイッチがオフ）で途中で止めた
	Out     string    `json:"out"`               // 画面の文字のファイル（本物の場所。中身は箱の中）
}

type tryArgs struct {
	Action  string   `json:"action"`
	Path    string   `json:"path"`
	Script  string   `json:"script"`
	Args    []string `json:"args"`
	Seconds int      `json:"seconds"`
	Offer   *bool    `json:"offer"`
	ID      string   `json:"id"`
	Sig     string   `json:"sig"`
	By      string   `json:"by"`
	Lines   int      `json:"lines"`
}

func (a *Agent) try(ctx context.Context, raw json.RawMessage) Reply {
	var in tryArgs
	_ = json.Unmarshal(raw, &in)
	tryMu.Lock()
	defer tryMu.Unlock()
	switch strings.ToLower(in.Action) {
	case "", "run":
		if a.st.Mode() != ModeOperate {
			return errText("pc_try（箱で試す）は「操作も可」のときだけ")
		}
		if strings.TrimSpace(in.Script) != "" {
			// 台本は pc_powershell の箱と同じ決まり（パスワード・鍵の場所は許可が要る・置き場所や箱の設定は触れない）
			info, err := a.inspect(ctx, in.Script)
			if err != nil {
				return errText("%v", err)
			}
			block, needAllow, what := boxScriptCheck(info, in.Script)
			if block != "" {
				return errText("%s", block)
			}
			if needAllow != "" {
				if ok, _ := a.st.Allowed(); !ok {
					return errText("%s（箱の中で動かすときも）", allowMsg(needAllow)).needs(what)
				}
			}
		}
		return a.tryRun(ctx, in)
	case "status", "show":
		return a.tryShow(ctx, in.Lines)
	case "offer":
		if a.st.Mode() != ModeOperate {
			return errText("pc_try は「操作も可」のときだけ")
		}
		return a.tryOffer(ctx)
	case "discard":
		if a.st.Mode() != ModeOperate {
			return errText("捨てるのは「操作も可」のときだけ。スイッチの窓で PC を「操作も可」にしてから、もう一度押してください")
		}
		by := "Claude"
		if in.By != "" && trySigOK(in.ID, "discard", in.Sig) {
			by = in.By
		}
		return a.tryDiscard(ctx, in.ID, by)
	case "apply":
		if !trySigOK(in.ID, "apply", in.Sig) {
			return errText("本物に入れるのは TRY の窓（カオルが押す）だけです。Claude からは入れられません。カオルに TRY の窓の［本物に入れる］を押してもらってください")
		}
		if a.st.Mode() != ModeOperate {
			return errText("PC が「操作も可」になっていません。スイッチの窓で PC を「操作も可」にしてから、もう一度押してください")
		}
		return a.tryApply(ctx, in.ID)
	}
	return errText("action は run・show・offer・discard のどれか（本物に入れるのは TRY の窓だけ）")
}

func (a *Agent) tryRun(ctx context.Context, in tryArgs) Reply {
	script := strings.TrimSpace(in.Script)
	if script == "" && strings.TrimSpace(in.Path) == "" {
		return errText("path（動かすファイル）か script（PowerShell の台本）のどちらかを指定してください")
	}
	if script != "" && strings.TrimSpace(in.Path) != "" {
		return errText("path と script はどちらか一方だけ")
	}
	if !fileExists(sbieStart()) {
		return errText("箱（Sandboxie-Plus）が入っていません")
	}
	var target string
	var scan []string // Defender で調べるもの（devkit\downloads のもの）
	if script == "" {
		target = normPath(in.Path)
		fi, err := os.Stat(target)
		if err != nil || fi.IsDir() {
			return errText("ファイルがありません：%s", target)
		}
		if cls, why := pathClass(target); cls == "switch" || cls == "sensitive" {
			return errText("%s：%sのものは箱でも動かしません", target, why)
		}
		if inDownloads(target) {
			scan = append(scan, target)
		}
	} else {
		for _, m := range riskyDownloadRe.FindAllString(script, -1) {
			if p := filepath.Join(homeDir(), m); fileExists(p) {
				scan = append(scan, p)
			}
		}
	}
	offer := script == "" && inDownloads(target)
	if in.Offer != nil {
		offer = *in.Offer
	}
	secs := in.Seconds
	if secs <= 0 {
		secs = 60
	}
	if secs > 110 {
		secs = 110
	}

	s, err := loadTry()
	switch {
	case err == nil && s.Status == "waiting":
		return errText("前の試し（%s）がカオルの返事待ちです（TRY の窓）。待つか、pc_try action=discard で捨ててから", s.label())
	case err == nil && s.Status == "working":
		// 続けて試す（箱はそのまま）
	default:
		if _, e := os.Stat(boxRoot()); e == nil {
			if err := emptyBox(ctx); err != nil {
				return errText("箱を空にできませんでした：%v", err)
			}
		}
		_ = os.RemoveAll(boxWorkDir())
		s = &tryState{ID: time.Now().Format("20060102-150405"), Started: time.Now(), Status: "working"}
	}
	var def []string
	for _, p := range scan {
		found, err := defenderScan(ctx, p)
		if err != nil {
			return errText("%v。検査できないものは箱でも動かさない", err)
		}
		if found != "" {
			return errText("%s（%s）。箱でも動かさずに止めました（ファイルはそのまま。消すかはカオルに聞く）", found, p)
		}
		def = append(def, filepath.Base(p))
	}
	if len(def) > 0 {
		s.Defender = "見つからなかった（" + strings.Join(def, "・") + "）"
	} else if s.Defender == "" {
		s.Defender = "調べるもの無し（Claude の台本・devkit\\downloads の外のもの）"
	}

	wd := boxWorkDir()
	if err := os.MkdirAll(wd, 0o755); err != nil {
		return errText("作業用のフォルダを作れません：%v", err)
	}
	wrap := filepath.Join(wd, "wrap.ps1")
	if err := os.WriteFile(wrap, append([]byte{0xEF, 0xBB, 0xBF}, []byte(boxWrap)...), 0o644); err != nil {
		return errText("包みの台本を書けません：%v", err)
	}
	n := len(s.Runs) + 1
	what := target
	if script != "" {
		target = filepath.Join(wd, fmt.Sprintf("run-%d.ps1", n))
		if err := os.WriteFile(target, append([]byte{0xEF, 0xBB, 0xBF}, []byte(script)...), 0o644); err != nil {
			return errText("台本を書けません：%v", err)
		}
		what = "台本 " + filepath.Base(target)
	}
	out := filepath.Join(wd, fmt.Sprintf("run-%d.out", n))
	code := filepath.Join(wd, fmt.Sprintf("run-%d.code", n))
	args := append([]string{"/box:" + tryBox, "/wait", "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", wrap, target, out, code}, in.Args...)
	run := tryRun{N: n, What: what, At: time.Now(), Out: out}
	timedOut, err := sbieWait(ctx, time.Duration(secs)*time.Second, args...)
	var ee *exec.ExitError
	switch {
	case stoppedBySwitch(ctx):
		// STOP：箱の中のものを止めて、ここまでの結果（箱の中身・画面の文字）は残す
		run.Stopped = true
		bg, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_ = sbie(bg, 30*time.Second, "/box:"+tryBox, "/terminate")
		cancel()
		ctx = context.Background()
	case timedOut:
		run.Running = true
	case err == nil:
		z := 0
		run.Exit = &z
	case errors.As(err, &ee):
		c := ee.ExitCode()
		run.Exit = &c
	default:
		return errText("箱の中で動かせませんでした：%v", err)
	}
	if s.Path == "" {
		s.Path, s.Args = what, in.Args
	}
	s.Runs = append(s.Runs, run)
	s.Ended = time.Now()
	s.Running = run.Running
	if run.Exit != nil {
		s.Exit = fmt.Sprintf("終わりのコード %d", *run.Exit)
	}
	s.Items, s.Noise, s.Registry, err = scanBox()
	if err != nil {
		return errText("箱の中身を読めませんでした：%v", err)
	}
	a.itemLines(ctx, s.Items)
	if offer {
		s.Status = "waiting"
	}
	if err := saveTry(s); err != nil {
		return errText("試しの記録を書けませんでした：%v", err)
	}
	tail := 40
	if in.Lines > 0 {
		tail = in.Lines
	}
	msg := tryReport(s) + "\n" + runOutput(run, tail)
	if w := a.boxWindows(ctx); len(w) > 0 {
		msg += "\n箱の中の窓（いま開いている。pc_screenshot・pc_ui・pc_ui_do で見て・押して確かめられる）：\n  " + strings.Join(w, "\n  ")
	}
	if offer {
		msg += "\nTRY の窓（右下）に出しました。カオルが［本物に入れる］か［捨てる］を押します。返事は pc_try action=show。待つ間はほかの作業を進めてよい"
	} else {
		msg += "\n（まだカオルには見せていない。直して続けて試すなら pc_try をもう一度・本物に入れたいものがあれば action=offer・試しだけなら action=discard）"
	}
	return okText(msg)
}

// boxWindows：箱の中で開いている窓（題名に [#] が付く）
func (a *Agent) boxWindows(ctx context.Context) []string {
	time.Sleep(1500 * time.Millisecond) // 窓が出るのを少し待つ
	out, err := a.ps.Run(ctx, uiScript("windows.ps1"), map[string]any{}, 30*time.Second)
	if err != nil {
		return nil
	}
	var w []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "[#]") {
			w = append(w, strings.TrimSpace(l))
		}
	}
	return w
}

func (s *tryState) label() string {
	if s.Path == "" {
		return s.ID
	}
	return filepath.Base(s.Path)
}

// runOutput：その回の画面の文字（箱の中の out）と終わりのコード
func runOutput(r tryRun, tail int) string {
	var l []string
	switch {
	case r.Stopped:
		l = append(l, fmt.Sprintf("%d 回目（%s）：%s（箱の中のものも止めた。箱の中身はそのまま）", r.N, r.What, stopMsg))
	case r.Running:
		l = append(l, fmt.Sprintf("%d 回目（%s）：まだ箱の中で動いている（待ち時間を過ぎた）", r.N, r.What))
	case r.Exit != nil && *r.Exit == 0:
		l = append(l, fmt.Sprintf("%d 回目（%s）：終わった（終わりのコード 0＝成功）", r.N, r.What))
	case r.Exit != nil:
		l = append(l, fmt.Sprintf("%d 回目（%s）：終わった（終わりのコード %d＝失敗の合図）", r.N, r.What, *r.Exit))
	}
	if al, bl := netSince(r.At); len(al)+len(bl) > 0 {
		if len(al) > 0 {
			l = append(l, "この回に中継が通したサイト："+strings.Join(al, "・"))
		}
		if len(bl) > 0 {
			l = append(l, "この回に中継が止めたもの："+strings.Join(bl, "・")+"（要るならカオルに一覧へ足してもらう）")
		}
	}
	b, err := os.ReadFile(realToBox(boxRoot(), r.Out))
	if err != nil {
		l = append(l, "画面の文字：無し（記録ができていない）")
		return strings.Join(l, "\n")
	}
	txt := strings.TrimPrefix(string(b), "\ufeff")
	ls := strings.Split(strings.TrimRight(strings.ReplaceAll(txt, "\r\n", "\n"), "\n"), "\n")
	if len(ls) == 1 && ls[0] == "" {
		l = append(l, "画面の文字：無し")
		return strings.Join(l, "\n")
	}
	if len(ls) > tail {
		l = append(l, fmt.Sprintf("画面の文字（%d 行のうち最後の %d 行。全部は lines を増やして action=show）：", len(ls), tail))
		ls = ls[len(ls)-tail:]
	} else {
		l = append(l, fmt.Sprintf("画面の文字（%d 行）：", len(ls)))
	}
	for _, x := range ls {
		l = append(l, "  │ "+oneLine(x, 400))
	}
	return strings.Join(l, "\n")
}

// itemLines：文字のファイルの「前 → 後」（変わった行）と、増えたファイルの先頭
func (a *Agent) itemLines(ctx context.Context, items []tryItem) {
	root := boxRoot()
	read := func(p string) (string, bool) {
		fi, err := os.Stat(p)
		if err != nil || fi.Size() > 256<<10 {
			return "", false
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return "", false
		}
		t, enc := a.decodeText(ctx, b, p)
		if enc == "バイナリ" {
			return "", false
		}
		return t, true
	}
	for i := range items {
		it := &items[i]
		switch it.Kind {
		case "new":
			if t, ok := read(realToBox(root, it.Real)); ok {
				it.Lines = headLines(t, 4)
			}
		case "changed":
			nt, ok1 := read(realToBox(root, it.Real))
			ot, ok2 := read(it.Real)
			if ok1 && ok2 {
				it.Lines = diffLines(ot, nt, 6)
			}
		}
	}
}

func splitLines(t string) []string {
	t = strings.TrimPrefix(strings.ReplaceAll(t, "\r\n", "\n"), "\ufeff")
	t = strings.TrimRight(t, "\n")
	if t == "" {
		return nil
	}
	return strings.Split(t, "\n")
}

func headLines(t string, n int) []string {
	ls := splitLines(t)
	var o []string
	for i, x := range ls {
		if i >= n {
			o = append(o, fmt.Sprintf("…（ほか %d 行）", len(ls)-n))
			break
		}
		o = append(o, "+ "+oneLine(x, 120))
	}
	return o
}

// diffLines：前と後で違う行だけ（- 前・+ 後）。行の並びの共通部分（LCS）で比べる。長いファイルは前後の同じ部分を除いてから
func diffLines(oldT, newT string, n int) []string {
	a, b := splitLines(oldT), splitLines(newT)
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	ea, eb := len(a), len(b)
	for ea > p && eb > p && a[ea-1] == b[eb-1] {
		ea--
		eb--
	}
	a, b = a[p:ea], b[p:eb]
	var o []string
	if len(a)*len(b) > 400*400 {
		for _, x := range a {
			o = append(o, "- "+oneLine(x, 120))
		}
		for _, x := range b {
			o = append(o, "+ "+oneLine(x, 120))
		}
	} else {
		L := make([][]int, len(a)+1)
		for i := range L {
			L[i] = make([]int, len(b)+1)
		}
		for i := len(a) - 1; i >= 0; i-- {
			for j := len(b) - 1; j >= 0; j-- {
				if a[i] == b[j] {
					L[i][j] = L[i+1][j+1] + 1
				} else {
					L[i][j] = max(L[i+1][j], L[i][j+1])
				}
			}
		}
		i, j := 0, 0
		for i < len(a) || j < len(b) {
			switch {
			case i < len(a) && j < len(b) && a[i] == b[j]:
				i++
				j++
			case j < len(b) && (i == len(a) || L[i][j+1] >= L[i+1][j]):
				o = append(o, "+ "+oneLine(b[j], 120))
				j++
			default:
				o = append(o, "- "+oneLine(a[i], 120))
				i++
			}
		}
	}
	if len(o) > n {
		o = append(o[:n], fmt.Sprintf("…（ほか %d 行）", len(o)-n))
	}
	return o
}

func tryReport(s *tryState) string {
	n := map[string]int{}
	for _, it := range s.Items {
		n[it.Kind]++
	}
	l := []string{
		fmt.Sprintf("箱で試した：%s（試しの番号 %s・%d 回）", s.Path, s.ID, max(len(s.Runs), 1)),
		"Defender：" + s.Defender,
		fmt.Sprintf("箱の中で（はじめから合わせて）：増えた %d・変わった %d・消えた %d・管理者の場所 %d（入れない）", n["new"], n["changed"], n["deleted"], n["admin"]),
		"ネット：許可したサイトだけ（中継を通す・一覧はカオルが決める。ほかは箱から直接つながらない）",
	}
	if s.Registry {
		l = append(l, "レジストリの変更あり（箱の中だけ。本物には入れない）")
	}
	if s.Noise > 0 {
		l = append(l, fmt.Sprintf("Windows が勝手に作ったもの・作業用のもの %d（入れない）", s.Noise))
	}
	kw := map[string]string{"new": "増えた", "changed": "変わった", "deleted": "消えた", "admin": "管理者の場所"}
	for i, it := range s.Items {
		if i >= 30 {
			l = append(l, fmt.Sprintf("  ほか %d", len(s.Items)-30))
			break
		}
		if it.Kind == "deleted" {
			l = append(l, fmt.Sprintf("  %s：%s", kw[it.Kind], it.Real))
		} else {
			l = append(l, fmt.Sprintf("  %s：%s（%s）", kw[it.Kind], it.Real, mb(it.Size)))
		}
		for _, x := range it.Lines {
			l = append(l, "      "+x)
		}
	}
	switch s.Status {
	case "working":
		l = append(l, "状態：Claude が試している（カオルにはまだ見せていない）")
	case "waiting":
		l = append(l, "返事：まだ（カオルの TRY の窓）")
	case "applied", "discarded":
		w := map[string]string{"applied": "本物に入れた", "discarded": "捨てた"}[s.Status]
		l = append(l, fmt.Sprintf("返事：%s（%s・%s）", w, s.By, s.At.Format("15:04")))
		if s.Result != "" {
			l = append(l, s.Result)
		}
	}
	return strings.Join(l, "\n")
}

// tryShow：いまの試し（箱の中身を読み直す）と、最後の回の画面の文字
func (a *Agent) tryShow(ctx context.Context, lines int) Reply {
	s, err := loadTry()
	if err != nil {
		return okText("箱で試しているものはありません")
	}
	if s.Status == "working" || s.Status == "waiting" {
		if items, noise, reg, err := scanBox(); err == nil {
			a.itemLines(ctx, items)
			if s.Status == "working" || sameItems(s.Items, items) {
				s.Items, s.Noise, s.Registry = items, noise, reg
				_ = saveTry(s)
			}
		}
	}
	msg := tryReport(s)
	if len(s.Runs) > 0 {
		if lines <= 0 {
			lines = 200
		}
		msg += "\n" + runOutput(s.Runs[len(s.Runs)-1], lines)
	}
	return okText(msg)
}

// tryOffer：Claude の試しを、カオルの TRY の窓に出す
func (a *Agent) tryOffer(ctx context.Context) Reply {
	s, err := loadTry()
	if err != nil || s.Status != "working" {
		return errText("見せる試しがありません（Claude が試しているものだけ見せられる）")
	}
	items, noise, reg, err := scanBox()
	if err != nil {
		return errText("箱の中身を読めませんでした：%v", err)
	}
	if len(items) == 0 {
		return errText("本物に入れるファイルがありません。試しだけなら action=discard で捨てる")
	}
	a.itemLines(ctx, items)
	s.Items, s.Noise, s.Registry, s.Status = items, noise, reg, "waiting"
	if err := saveTry(s); err != nil {
		return errText("試しの記録を書けませんでした：%v", err)
	}
	return okText(tryReport(s) + "\nTRY の窓（右下）に出しました。カオルが［本物に入れる］か［捨てる］を押します")
}

func (a *Agent) tryDiscard(ctx context.Context, id, by string) Reply {
	s, err := loadTry()
	if err == nil && id != "" && s.ID != id {
		return errText("試しの番号が違います（いまは %s）", s.ID)
	}
	if err := emptyBox(ctx); err != nil {
		return errText("箱を空にできませんでした：%v", err)
	}
	_ = os.RemoveAll(boxWorkDir())
	if s != nil && (s.Status == "waiting" || s.Status == "working") {
		s.Status, s.By, s.At = "discarded", by, time.Now()
		s.Result = "箱を空にした（本物は何も変えていない）"
		_ = saveTry(s)
	}
	return okText("捨てた：箱を空にした（本物は何も変えていない）")
}

func (a *Agent) tryApply(ctx context.Context, id string) Reply {
	s, err := loadTry()
	if err != nil || s.ID != id {
		return errText("この試しはもうありません")
	}
	if s.Status != "waiting" {
		return errText("この試しはもう終わっています（%s）", s.Status)
	}
	_ = sbie(ctx, 30*time.Second, "/box:"+tryBox, "/terminate") // 箱の中で動いているものを止めてから
	time.Sleep(time.Second)
	items, _, _, err := scanBox()
	if err != nil {
		return errText("箱の中身を読めませんでした：%v", err)
	}
	// 窓に出したもの（記録）と、いまの箱の中身が同じかを確かめる（窓の表示と違うものは入れない）
	if !sameItems(s.Items, items) {
		a.itemLines(ctx, items)
		s.Items = items
		_ = saveTry(s)
		return errText("箱の中身が TRY の窓に出していたものから変わっていました（箱の中で使った分など）。窓の一覧を新しくしたので、確かめてからもう一度押してください（まだ何も入れていない）")
	}
	return a.tryCommit(ctx, s, items)
}

// 試験で差し替える：取り置きへ送る・置き換える
var (
	tryRename = os.Rename
)

// heldItem：取り置きへ送ったもの（元の場所と、取り置きの中のフォルダ）
type heldItem struct{ From, Dir string }

// tryHold：元のファイルを取り置きへ送り、送れたものを返す（キーは元の場所の小文字。送れなかったものは入らない）
func (a *Agent) tryHold(ctx context.Context, paths []string) (map[string]heldItem, string) {
	if a.holdFn != nil {
		return a.holdFn(ctx, paths)
	}
	b, _ := json.Marshal(map[string]any{"paths": paths})
	r := a.trash(ctx, b)
	held := map[string]heldItem{}
	for _, m := range trashHeldRe.FindAllStringSubmatch(r.Text, -1) {
		held[strings.ToLower(m[1])] = heldItem{m[1], m[2]}
	}
	return held, r.Text
}

// tryCommit：本物に入れる（2026-11-28 直し：全部入れるか、何も入れないか）
//  1. 先に全部を確かめる：試しのあとで本物が変わったもの・消すはずのものが無い/変わった・箱の中の場所が分からない → 1 つでもあれば何も入れずに止める
//  2. 全部を一時ファイルに写し、窓に出した中身（SHA-256）と同じかを確かめる → 違えば何も入れずに止める
//  3. 変わるものの元と消すものを取り置きへ → 1 つでも送れなければ、送った分を取り置きから戻して止める
//  4. 置き換える → 途中で失敗したら、置いた分を外し、取り置きから元を戻す（戻せたかも確かめて返事に出す）
func (a *Agent) tryCommit(ctx context.Context, s *tryState, items []tryItem) Reply {
	root := boxRoot()
	var conflicts, skipped, del, olds []string
	type put struct{ tmp, dst, hash, kind string }
	var puts []put
	for _, it := range items {
		switch it.Kind {
		case "admin":
			skipped = append(skipped, it.Real+"（管理者の場所。本物でもう一度入れ直す）")
		case "deleted":
			rf, e := os.Stat(it.Real)
			switch {
			case e != nil:
				conflicts = append(conflicts, it.Real+"（消すはずのものが本物にもう無い）")
			case rf.ModTime().After(s.Started):
				conflicts = append(conflicts, it.Real+"（試しのあとで本物が変わった。消すと変わった分がなくなる）")
			default:
				del = append(del, it.Real)
			}
		case "new", "changed":
			rf, e := os.Stat(it.Real)
			switch {
			case e == nil && rf.ModTime().After(s.Started):
				conflicts = append(conflicts, it.Real+"（試しのあとで本物が変わった）")
			case e == nil && it.Kind == "new":
				conflicts = append(conflicts, it.Real+"（試しのあとで本物に同じ名前のものができた）")
			case e != nil && it.Kind == "changed":
				conflicts = append(conflicts, it.Real+"（変えるはずの本物がもう無い）")
			case realToBox(root, it.Real) == "":
				conflicts = append(conflicts, it.Real+"（箱の中の場所が分からない）")
			case it.Hash == "":
				conflicts = append(conflicts, it.Real+"（窓に出した中身の記録が無い）")
			default:
				puts = append(puts, put{dst: it.Real, hash: it.Hash, kind: it.Kind})
				if it.Kind == "changed" {
					olds = append(olds, it.Real)
				}
			}
		}
	}
	if len(conflicts) > 0 {
		return errText("本物と食い違うものがあるので、何も入れずに止めました（一部だけ入れることはしない）：%s。捨てるか、Claude に試し直してもらう", strings.Join(conflicts, "、"))
	}
	cleanTmp := func() {
		for _, p := range puts {
			if p.tmp != "" {
				_ = os.Remove(p.tmp)
			}
		}
	}
	for i := range puts {
		tmp, err := copyVerified(realToBox(root, puts[i].dst), puts[i].dst)
		if err == nil && fileHash(tmp) != puts[i].hash {
			_ = os.Remove(tmp)
			err = errors.New("窓に出したときと中身が違う（SHA-256 が合わない）")
		}
		if err != nil {
			cleanTmp()
			return errText("%s：%v。何も入れずに止めました。窓の一覧を確かめてからもう一度押してください", puts[i].dst, err)
		}
		puts[i].tmp = tmp
	}
	// 元を取り置きへ（30 日戻せる）
	hold := append(append([]string{}, olds...), del...)
	held := map[string]heldItem{}
	if len(hold) > 0 {
		var text string
		held, text = a.tryHold(ctx, hold)
		var miss []string
		for _, p := range hold {
			if _, ok := held[strings.ToLower(p)]; !ok {
				miss = append(miss, p)
			}
		}
		if len(miss) > 0 {
			cleanTmp()
			back := restoreHeld(held)
			return errText("元のファイルを取り置きへ送れなかったので、何も入れずに止めました（%s：%s）。%s", strings.Join(miss, "、"), oneLine(text, 200), strings.Join(back, "。"))
		}
	}
	var placed []string
	for i, p := range puts {
		if err := tryRename(p.tmp, p.dst); err != nil {
			// 途中で失敗：置いた分を外し、元を取り置きから戻す
			for _, d := range placed {
				_ = os.Remove(d)
			}
			for _, q := range puts[i:] {
				_ = os.Remove(q.tmp)
			}
			back := restoreHeld(held)
			return errText("%s を置き換えられなかった（%v）ので、入れた分を外して元に戻しました：%s", p.dst, err, strings.Join(back, "。"))
		}
		placed = append(placed, p.dst)
	}
	res := []string{fmt.Sprintf("入れた %d", len(placed))}
	if len(olds) > 0 {
		res = append(res, fmt.Sprintf("変わったものの元 %d を取り置きへ", len(olds)))
	}
	if len(del) > 0 {
		res = append(res, fmt.Sprintf("箱の中で消えたもの %d を取り置きへ", len(del)))
	}
	if len(skipped) > 0 {
		res = append(res, fmt.Sprintf("入れなかった %d：%s", len(skipped), strings.Join(skipped, "、")))
	}
	if err := emptyBox(ctx); err != nil {
		res = append(res, "箱を空にできなかった："+err.Error())
	}
	_ = os.RemoveAll(boxWorkDir())
	s.Status, s.By, s.At, s.Result = "applied", "カオル（TRY の窓）", time.Now(), strings.Join(res, "。")
	_ = saveTry(s)
	return okText("本物に入れた：" + s.Result)
}

// restoreHeld：取り置きに送った元を、元の場所へ写して戻す（中身が取り置きと同じかも確かめる）。結果の言葉を返す
func restoreHeld(held map[string]heldItem) []string {
	if len(held) == 0 {
		return []string{"元は動かしていない"}
	}
	var ok, ng []string
	for _, h := range held {
		real, dir := h.From, h.Dir
		src := filepath.Join(dir, filepath.Base(real))
		if _, err := os.Stat(src); err != nil {
			ng = append(ng, real+"（取り置きに写しが見つからない："+dir+"）")
			continue
		}
		if _, err := os.Stat(real); err == nil {
			if sameFile(src, real) {
				ok = append(ok, real)
			} else {
				ng = append(ng, real+"（元の場所に別のものがある。取り置き："+dir+"）")
			}
			continue
		}
		if err := copyTree(src, real); err != nil || !sameFile(src, real) {
			ng = append(ng, real+"（戻せなかった。取り置き："+dir+"）")
			continue
		}
		ok = append(ok, real)
	}
	out := []string{fmt.Sprintf("取り置きから元を戻した %d（中身を確かめた）", len(ok))}
	if len(ng) > 0 {
		out = append(out, "戻せなかった "+strings.Join(ng, "、")+"。カオルが取り置きから戻す")
	}
	return out
}

func sameItems(a, b []tryItem) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Kind != b[i].Kind || !strings.EqualFold(a[i].Real, b[i].Real) || a[i].Size != b[i].Size || a[i].Hash != b[i].Hash {
			return false
		}
	}
	return true
}

// realToBox：本物の場所 → 箱の中の場所
func realToBox(root, real string) string {
	under := func(base string) (string, bool) {
		if base == "" {
			return "", false
		}
		r, err := filepath.Rel(base, real)
		if err != nil || r == "." || strings.HasPrefix(r, "..") {
			return "", false
		}
		return r, true
	}
	if r, ok := under(homeDir()); ok {
		return filepath.Join(root, "user", "current", r)
	}
	if r, ok := under(programData()); ok {
		return filepath.Join(root, "user", "all", r)
	}
	if len(real) > 3 && real[1] == ':' && (real[2] == '\\' || real[2] == '/') {
		return filepath.Join(root, "drive", strings.ToUpper(real[:1]), real[3:])
	}
	return ""
}

// copyVerified：入れる先のとなりに一時の名前で写し、中身が同じかを確かめる（置き換えは後で）
func copyVerified(src, dst string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", fmt.Errorf("フォルダを作れない：%v", err)
	}
	in, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("箱の中を読めない：%v", err)
	}
	defer in.Close()
	tmp := dst + ".claude-hands-try"
	out, err := os.Create(tmp)
	if err != nil {
		return "", fmt.Errorf("書けない：%v", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return "", fmt.Errorf("写せない：%v", err)
	}
	out.Close()
	if fileHash(tmp) != fileHash(src) {
		_ = os.Remove(tmp)
		return "", errors.New("写した中身が合わない")
	}
	return tmp, nil
}
