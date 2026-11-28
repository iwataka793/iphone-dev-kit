package main

// 作業のまとまり（hands の道具）：TIMELINE・DONE の報告・まとめて戻す・Git の記録（ASK-DESIGN.md の作る順番 2）。
//
//   TIMELINE：3 つの手（PC・ブラウザ・iPhone）の道具の呼び出しを、1 本の時系列の記録に残す
//             （%USERPROFILE%\devkit\hands\timeline\日付.jsonl。iPhone の拡張機能も同じ所に書く）
//   作業のまとまり：Claude が hands start（題名）〜 hands done で区切る。今の作業は task.json
//   Git：決まったフォルダ（hands\git-folders.txt。既定 devkit/setup）の文字のファイルを、start と done で記録
//        （記録の置き場所は %ProgramData%\claude-hands\git。GitHub には上げない）
//   DONE：記録（TIMELINE と Git）から「何を変えたか」を作る（Claude の説明からは作らない）
//   まとめて戻す：その作業で Claude が変えた・消した分だけ戻す。あとで変わっているものは戻さずに知らせる

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

func handsUserDir() string {
	return env("PC_HANDS_USERDIR", filepath.Join(homeDir(), "devkit", "hands"))
}
func timelineDir() string { return filepath.Join(handsUserDir(), "timeline") }
func gitDir() string      { return filepath.Join(handsDir(), "git") }

type undoItem struct {
	Kind string `json:"kind"` // trash：取り置きから元の場所へ写す
	From string `json:"from"`
	Held string `json:"held"`
}

type tlEntry struct {
	T    time.Time  `json:"t"`
	Hand string     `json:"hand"` // pc・browser・iphone
	Tool string     `json:"tool"`
	Args string     `json:"args,omitempty"`
	OK   bool       `json:"ok"`
	Res  string     `json:"res,omitempty"`
	Ms   int64      `json:"ms"`
	Task string     `json:"task,omitempty"`
	Lane string     `json:"lane,omitempty"` // 作業の道 A〜E（lane.go）
	Undo []undoItem `json:"undo,omitempty"`
}

type handsTask struct {
	ID     string    `json:"id"`
	Title  string    `json:"title"`
	Start  time.Time `json:"start"`
	End    time.Time `json:"end,omitempty"`
	Before string    `json:"before,omitempty"` // Git の記録（始め）
	After  string    `json:"after,omitempty"`  // Git の記録（終わり）
}

var tlMu sync.Mutex

func oneLine(s string, n int) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " ⏎ ")
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func appendTimeline(e tlEntry) {
	tlMu.Lock()
	defer tlMu.Unlock()
	if e.Task == "" {
		if t := currentTask(); t != nil {
			e.Task = t.ID
		}
	}
	_ = os.MkdirAll(timelineDir(), 0o755)
	f, err := os.OpenFile(filepath.Join(timelineDir(), e.T.Format("2006-01-02")+".jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	b, _ := json.Marshal(e)
	_, _ = f.Write(append(b, '\r', '\n'))
}

var trashHeldRe = regexp.MustCompile(`(?m)^(?:消した（取り置き 30 日・ごみ箱は通さない）|ごみ箱へ（取り置き 30 日）)：(.+?) → (.+?)(?:（(?:元を消せず|空のフォルダ).*）)?\s*$`)

// recordCall：常駐役が受けた道具の呼び出しを TIMELINE に残す
func recordCall(tool string, args json.RawMessage, rep Reply, d time.Duration) {
	if tool == "pc_status" {
		return
	}
	if tool == "hands" {
		var in struct {
			Action string `json:"action"`
		}
		_ = json.Unmarshal(args, &in)
		if in.Action != "undo" { // start・done は hands の中で書く。log は読むだけ
			return
		}
	}
	hand := "pc"
	if isBrowserTool(tool) {
		hand = "browser"
	}
	e := tlEntry{T: time.Now(), Hand: hand, Tool: tool, Args: oneLine(string(args), 300), OK: !rep.IsError, Res: oneLine(strings.SplitN(rep.Text, "\n", 2)[0], 160), Ms: d.Milliseconds()}
	e.Lane = rep.Lane
	if e.Lane == "" {
		var m map[string]any
		_ = json.Unmarshal(args, &m)
		e.Lane = laneOf(tool, m)
	}
	if tool == "pc_trash" {
		for _, m := range trashHeldRe.FindAllStringSubmatch(rep.Text, -1) {
			e.Undo = append(e.Undo, undoItem{Kind: "trash", From: m[1], Held: m[2]})
		}
	}
	appendTimeline(e)
}

// ---- 作業のまとまり ----

func taskFile() string { return filepath.Join(handsUserDir(), "task.json") }

func currentTask() *handsTask {
	b, err := os.ReadFile(taskFile())
	if err != nil {
		return nil
	}
	var t handsTask
	if json.Unmarshal(b, &t) != nil || t.ID == "" {
		return nil
	}
	return &t
}

func saveTaskFile(path string, t *handsTask) error {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	b, _ := json.MarshalIndent(t, "", "  ")
	return os.WriteFile(path, b, 0o644)
}

func doneDir() string { return filepath.Join(handsUserDir(), "done") }

func loadDoneTask(id string) (*handsTask, error) {
	b, err := os.ReadFile(filepath.Join(doneDir(), id+".json"))
	if err != nil {
		return nil, fmt.Errorf("作業 %s の記録がありません（hands action=log で作業の番号を見る）", id)
	}
	var t handsTask
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// entries：from〜to の TIMELINE（task を指定したらその作業の分だけ）
func entries(from, to time.Time, task string) []tlEntry {
	var out []tlEntry
	for d := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.Local); !d.After(to); d = d.AddDate(0, 0, 1) {
		f, err := os.Open(filepath.Join(timelineDir(), d.Format("2006-01-02")+".jsonl"))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			var e tlEntry
			if json.Unmarshal(bytes.TrimSpace(sc.Bytes()), &e) != nil {
				continue
			}
			if e.T.Before(from) || e.T.After(to) || (task != "" && e.Task != task) {
				continue
			}
			out = append(out, e)
		}
		f.Close()
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].T.Before(out[j].T) })
	return out
}

// ---- Git ----

func gitExe() string {
	if v := env("PC_HANDS_GIT", ""); v != "" {
		return v
	}
	for _, p := range []string{`C:\Program Files\Git\cmd\git.exe`, `C:\Program Files (x86)\Git\cmd\git.exe`} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath("git"); err == nil {
		return p
	}
	return ""
}

// gitPaths：記録するフォルダ（ホームからの相対。git-folders.txt の 1 行 1 つ。# はコメント）
func gitPaths() []string {
	cfg := filepath.Join(handsUserDir(), "git-folders.txt")
	b, err := os.ReadFile(cfg)
	if err != nil {
		_ = os.MkdirAll(handsUserDir(), 0o755)
		def := "# claude-hands：作業の前後を Git に記録するフォルダ（ユーザーフォルダからの場所。1 行に 1 つ）\r\ndevkit/setup\r\n"
		_ = os.WriteFile(cfg, []byte(def), 0o644)
		b = []byte(def)
	}
	var ps []string
	for _, l := range strings.Split(strings.ReplaceAll(strings.TrimPrefix(string(b), "\ufeff"), "\r\n", "\n"), "\n") {
		l = strings.Trim(strings.TrimSpace(l), `/\`)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		l = filepath.ToSlash(l)
		if st, err := os.Stat(filepath.Join(homeDir(), filepath.FromSlash(l))); err == nil && st.IsDir() {
			ps = append(ps, l)
		}
	}
	return ps
}

const gitExclude = "# claude-hands：大きいもの・作り直せるものは記録しない\nruns/\nfiles/\nlogs/\nbackup/\n*.png\n*.jpg\n*.jpeg\n*.gif\n*.mp4\n*.mov\n*.zip\n*.ipa\n*.exe\n*.dll\n*.msi\n*.mcpb\n*.log\n*.db\n*.plist\n"

func gitRaw(ctx context.Context, args ...string) ([]byte, error) {
	exe := gitExe()
	if exe == "" {
		return nil, fmt.Errorf("Git が見つかりません（C:\\Program Files\\Git）")
	}
	base := []string{"--git-dir=" + gitDir(), "--work-tree=" + homeDir(), "-c", "safe.directory=*", "-c", "core.autocrlf=false", "-c", "core.quotepath=false",
		"-c", "user.name=claude-hands", "-c", "user.email=hands@localhost", "-c", "gc.auto=0"}
	cmd := exec.CommandContext(ctx, exe, append(base, args...)...)
	cmd.Dir = homeDir() // 場所はユーザーフォルダから数える（常駐役は devkit\switch-app から起動されるので、そのままだとそこからの場所になる）
	hideWindow(cmd)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return out.Bytes(), fmt.Errorf("git %s：%v %s", args[0], err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

func gitText(ctx context.Context, args ...string) (string, error) {
	b, err := gitRaw(ctx, args...)
	return strings.TrimSpace(string(b)), err
}

// gitSnapshot：決まったフォルダの今を記録して、その記録の番号を返す
func gitSnapshot(ctx context.Context, msg string) (string, error) {
	paths := gitPaths()
	if len(paths) == 0 {
		return "", fmt.Errorf("記録するフォルダがありません（%s）", filepath.Join(handsUserDir(), "git-folders.txt"))
	}
	if _, err := os.Stat(filepath.Join(gitDir(), "HEAD")); err != nil {
		if err := os.MkdirAll(gitDir(), 0o755); err != nil {
			return "", fmt.Errorf("Git の記録の場所を作れません（%s）。カオルに pc-setup.bat を動かしてもらう：%v", gitDir(), err)
		}
		if _, err := gitText(ctx, "init", "-q"); err != nil {
			return "", err
		}
		_ = os.MkdirAll(filepath.Join(gitDir(), "info"), 0o755)
		_ = os.WriteFile(filepath.Join(gitDir(), "info", "exclude"), []byte(gitExclude), 0o644)
	}
	if _, err := gitText(ctx, append([]string{"add", "-A", "--"}, paths...)...); err != nil {
		return "", err
	}
	if _, err := gitText(ctx, "commit", "-q", "--allow-empty", "-m", msg); err != nil {
		return "", err
	}
	return gitText(ctx, "rev-parse", "HEAD")
}

type gitChange struct{ st, path string }

func gitChanges(ctx context.Context, a, b string) ([]gitChange, error) {
	out, err := gitText(ctx, "diff", "--name-status", "--no-renames", a, b)
	if err != nil {
		return nil, err
	}
	var cs []gitChange
	for _, l := range strings.Split(out, "\n") {
		if p := strings.SplitN(l, "\t", 2); len(p) == 2 {
			cs = append(cs, gitChange{p[0][:1], p[1]})
		}
	}
	return cs, nil
}

var stWord = map[string]string{"M": "変えた", "A": "作った", "D": "消した"}

// ---- 道具 ----

func (a *Agent) hands(ctx context.Context, raw json.RawMessage) Reply {
	var in struct {
		Action string `json:"action"`
		Title  string `json:"title"`
		Task   string `json:"task"`
		Max    int    `json:"max"`
	}
	_ = json.Unmarshal(raw, &in)
	switch in.Action {
	case "start":
		return a.handsStart(ctx, in.Title)
	case "done":
		return a.handsDone(ctx)
	case "undo":
		if a.st.Mode() != ModeOperate {
			return errText("戻すのは「操作も可」のときだけです。要るならカオルに PC-OPERATE.bat を押してもらってください")
		}
		return a.handsUndo(ctx, in.Task)
	case "", "status", "log":
		return a.handsLog(in.Task, in.Max)
	}
	return errText("action は start（題名）・done・log・undo（task）のどれか")
}

func (a *Agent) handsStart(ctx context.Context, title string) Reply {
	var pre []string
	if t := currentTask(); t != nil {
		r := a.handsDone(ctx)
		pre = append(pre, "（前の作業「"+t.Title+"」が開いたままだったので閉じた）", r.Text, "")
	}
	if strings.TrimSpace(title) == "" {
		title = "（題名なし）"
	}
	t := &handsTask{ID: time.Now().Format("20060102-150405"), Title: oneLine(title, 80), Start: time.Now()}
	note := ""
	if h, err := gitSnapshot(ctx, t.ID+" 前："+t.Title); err == nil {
		t.Before = h
	} else {
		note = "（Git の記録はできなかった：" + err.Error() + "）"
	}
	if err := saveTaskFile(taskFile(), t); err != nil {
		return errText("作業を始められません：%v", err)
	}
	appendTimeline(tlEntry{T: t.Start, Hand: "pc", Tool: "hands", Args: "start " + t.Title, OK: true, Task: t.ID})
	return okText(strings.Join(append(pre, fmt.Sprintf("作業を始めた：%s（番号 %s）%s\n終わったら hands action=done。決まったフォルダ（%s）の文字のファイルは前後を記録する", t.Title, t.ID, note, strings.Join(gitPaths(), "・"))), "\n"))
}

func (a *Agent) handsDone(ctx context.Context) Reply {
	t := currentTask()
	if t == nil {
		return errText("始めた作業がありません（hands action=start title=…）")
	}
	t.End = time.Now()
	if t.Before != "" {
		if h, err := gitSnapshot(ctx, t.ID+" 後："+t.Title); err == nil {
			t.After = h
		}
	}
	appendTimeline(tlEntry{T: t.End, Hand: "pc", Tool: "hands", Args: "done " + t.Title, OK: true, Task: t.ID})
	rep := a.report(ctx, t)
	_ = saveTaskFile(filepath.Join(doneDir(), t.ID+".json"), t)
	_ = os.WriteFile(filepath.Join(doneDir(), t.ID+".txt"), []byte(strings.ReplaceAll(rep, "\n", "\r\n")), 0o644)
	_ = os.Remove(taskFile())
	return okText(rep)
}

// report：DONE の報告（記録から作る）
func (a *Agent) report(ctx context.Context, t *handsTask) string {
	es := entries(t.Start, t.End, t.ID)
	count := map[string]int{}
	var hands = map[string]int{}
	ng := 0
	var held []undoItem
	lanes := map[string]int{}
	for _, e := range es {
		if e.Tool == "hands" {
			continue
		}
		if e.Lane != "" {
			lanes[e.Lane]++
		}
		count[e.Tool]++
		hands[e.Hand]++
		if !e.OK {
			ng++
		}
		held = append(held, e.Undo...)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "DONE：%s（%s〜%s）\n", t.Title, t.Start.Format("15:04"), t.End.Format("15:04"))
	fmt.Fprintf(&b, "手の数：PC %d・ブラウザ %d・iPhone %d（うまくいかなかった %d）\n", hands["pc"], hands["browser"], hands["iphone"], ng)
	if len(lanes) > 0 {
		var ls []string
		for _, k := range []string{"A", "B", "C", "D", "E"} {
			if lanes[k] > 0 {
				ls = append(ls, fmt.Sprintf("%s %s %d", k, laneWord[k], lanes[k]))
			}
		}
		fmt.Fprintf(&b, "道：%s\n", strings.Join(ls, "・"))
	}
	if t.Before != "" && t.After != "" {
		cs, err := gitChanges(ctx, t.Before, t.After)
		if err != nil {
			fmt.Fprintf(&b, "変えたファイル：Git から読めなかった（%v）\n", err)
		} else {
			fmt.Fprintf(&b, "変えたファイル（%s。戻せる）：%d\n", strings.Join(gitPaths(), "・"), len(cs))
			for i, c := range cs {
				if i >= 30 {
					fmt.Fprintf(&b, "  ほか %d\n", len(cs)-i)
					break
				}
				fmt.Fprintf(&b, "  %s：%s\n", stWord[c.st], c.path)
			}
		}
	} else {
		b.WriteString("変えたファイル：Git の記録なし\n")
	}
	for _, l := range tryDoneLines(t.Start, t.End) {
		b.WriteString(l + "\n")
	}
	fmt.Fprintf(&b, "消したもの（取り置き。戻せる）：%d\n", len(held))
	for _, h := range held {
		fmt.Fprintf(&b, "  %s\n", h.From)
	}
	var tools []string
	for k, v := range count {
		if k != "pc_trash" {
			tools = append(tools, fmt.Sprintf("%s %d", k, v))
		}
	}
	sort.Strings(tools)
	if len(tools) > 0 {
		b.WriteString("そのほかの操作（中身は TIMELINE に記録。決まったフォルダの外の変更・送ったもの・設定は、まとめて戻すの対象外）：\n  " + strings.Join(tools, "・") + "\n")
	}
	fmt.Fprintf(&b, "戻すとき：hands action=undo task=%s（Claude の分だけ。あとで変わったものは戻さない）", t.ID)
	return b.String()
}

func (a *Agent) handsLog(task string, max int) Reply {
	if max <= 0 {
		max = 40
	}
	var lines []string
	if t := currentTask(); t != nil {
		lines = append(lines, fmt.Sprintf("今の作業：%s（番号 %s・%s から）", t.Title, t.ID, t.Start.Format("01-02 15:04")))
	} else {
		lines = append(lines, "今の作業：なし")
	}
	if ds, err := os.ReadDir(doneDir()); err == nil {
		var ids []string
		for _, d := range ds {
			if strings.HasSuffix(d.Name(), ".json") {
				ids = append(ids, strings.TrimSuffix(d.Name(), ".json"))
			}
		}
		sort.Sort(sort.Reverse(sort.StringSlice(ids)))
		if len(ids) > 0 {
			lines = append(lines, "終わった作業（新しい順）：")
			for i, id := range ids {
				if i >= 5 {
					break
				}
				if t, err := loadDoneTask(id); err == nil {
					lines = append(lines, fmt.Sprintf("  %s %s（%s〜%s）", t.ID, t.Title, t.Start.Format("01-02 15:04"), t.End.Format("15:04")))
				}
			}
		}
	}
	var es []tlEntry
	if task != "" {
		if t, err := loadDoneTask(task); err == nil {
			es = entries(t.Start, t.End, t.ID)
		} else if c := currentTask(); c != nil && c.ID == task {
			es = entries(c.Start, time.Now(), c.ID)
		} else {
			return errText("%v", err)
		}
	} else {
		es = entries(time.Now().Add(-24*time.Hour), time.Now(), "")
	}
	if len(es) > max {
		es = es[len(es)-max:]
	}
	lines = append(lines, fmt.Sprintf("TIMELINE（%d 件）：", len(es)))
	hn := map[string]string{"pc": "PC", "browser": "WEB", "iphone": "iPhone"}
	for _, e := range es {
		r := "ok"
		if !e.OK {
			r = "NG " + e.Res
		}
		lines = append(lines, fmt.Sprintf("  %s [%s] %s %s → %s", e.T.Format("15:04:05"), hn[e.Hand], e.Tool, oneLine(e.Args, 120), r))
	}
	return okText(strings.Join(lines, "\n"))
}

func (a *Agent) handsUndo(ctx context.Context, id string) Reply {
	if id == "" {
		return errText("task（作業の番号）を指定してください（hands action=log で見る）")
	}
	t, err := loadDoneTask(id)
	if err != nil {
		return errText("%v", err)
	}
	var out []string
	out = append(out, fmt.Sprintf("まとめて戻す：%s（%s〜%s）", t.Title, t.Start.Format("01-02 15:04"), t.End.Format("15:04")))
	// 1) 文字のファイル（Git）
	if t.Before != "" && t.After != "" {
		out = append(out, a.restoreGit(ctx, t.Before, t.After)...)
	}
	// 2) 消したもの（取り置きから元の場所へ写す）
	for _, e := range entries(t.Start, t.End, t.ID) {
		for _, u := range e.Undo {
			if u.Kind != "trash" {
				continue
			}
			if _, err := os.Stat(u.From); err == nil {
				out = append(out, "  戻さなかった（元の場所に別のものがある）："+u.From)
				continue
			}
			src := filepath.Join(u.Held, filepath.Base(u.From))
			if err := copyTree(src, u.From); err != nil {
				out = append(out, "  戻せなかった："+u.From+"（"+err.Error()+"）")
			} else {
				out = append(out, "  取り置きから戻した："+u.From)
			}
		}
	}
	if len(out) == 1 {
		out = append(out, "  戻すものはなかった")
	}
	return okText(strings.Join(out, "\n"))
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func copyTree(src, dst string) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return copyFile(src, dst, st)
	}
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		to := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(p, to, fi)
	})
}

func copyFile(src, dst string, fi os.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, fi.ModTime(), fi.ModTime())
}

// restoreGit：Git の記録 before〜after の間に変わった決まったフォルダのファイルを、before に戻す（あとで変わったものは戻さない。作ったファイルは取り置きへ）
func (a *Agent) restoreGit(ctx context.Context, before, after string) []string {
	var out []string
	cs, err := gitChanges(ctx, before, after)
	if err != nil {
		out = append(out, "Git：読めなかった（"+err.Error()+"）")
	}
	var toTrash []string
	for _, c := range cs {
		cur := filepath.Join(homeDir(), filepath.FromSlash(c.path))
		now := ""
		if _, err := os.Stat(cur); err == nil {
			now, _ = gitText(ctx, "hash-object", "--", cur)
		}
		want := ""
		if c.st != "D" {
			want, _ = gitText(ctx, "rev-parse", after+":"+c.path)
		}
		if now != want {
			out = append(out, "  戻さなかった（あとで変わっている）："+c.path)
			continue
		}
		switch c.st {
		case "M", "D":
			b, err := gitRaw(ctx, "show", before+":"+c.path)
			if err == nil {
				_ = os.MkdirAll(filepath.Dir(cur), 0o755)
				err = os.WriteFile(cur, b, 0o644)
			}
			if err != nil {
				out = append(out, "  戻せなかった："+c.path+"（"+err.Error()+"）")
			} else {
				out = append(out, "  戻した（"+stWord[c.st]+"のを元に）："+c.path)
			}
		case "A":
			toTrash = append(toTrash, cur)
		}
	}
	if len(toTrash) > 0 { // Claude が作ったファイルは、消さずに取り置きへ
		r := a.trash(ctx, mustJSON(map[string]any{"paths": toTrash}))
		out = append(out, "  作ったファイルを取り置きへ："+oneLine(r.Text, 400))
	}
	return out
}
