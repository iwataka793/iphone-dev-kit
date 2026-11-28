package main

// RECIPE（台本）：PC とブラウザの手を順に書いて、常駐役が 1 回の呼び出しで最後まで動かす（LINK-DESIGN.md の 3・RESEARCH-DESIGN.md の 3）。
// Claude との往復が減るので、速く、言葉（トークン）も少ない。道具は増やさず hands の action（recipes・recipe・save・run）。
//
// 守り（台本を守りの抜け道にしない）：
//   - 手は 1 つずつ、ふつうの呼び出しと同じ道（a.call：スイッチ・道 A〜E・許可・E の止め）を通る
//   - 許可（ALLOW）は使わない：許可が出ている間は台本を流さない。許可が要る手に当たったら止まる
//   - 使えない手：hands（台本の入れ子）・pc_trash（消す）・本物で変える台本（pc_powershell の real）・画面を撮る道具（台本は文字だけ返す）
//   - 失敗したら、その手で止まる（前の手は済んでいる）。E（最後の一押し）もそこで止まり、押すのはカオル
//   - 同時に 2 つ流さない。STOP で途中でも止まる（stop.go）。手ごとに TIMELINE に残る
//   - 置き場所は PC の中（devkit\hands\recipes。中身に個人の情報が入りうるので repo には入れない）。前の版は recipes\history に残す
// まだやらないこと（あとで）：iPhone の手・前の手の結果を次の手に渡す・WATCH（見張り）・記録から自動で台本を作る（操作研究）

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type recipeStep struct {
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
	Note string         `json:"note,omitempty"` // この手で何をするか（ふつうの言葉）
	Show bool           `json:"show,omitempty"` // この手の返事を全部返す（既定は最後の手だけ）
}

type recipe struct {
	Name    string            `json:"name"`
	About   string            `json:"about"`            // 何をする台本か（1 行。探すときに使う）
	Params  map[string]string `json:"params,omitempty"` // 入れ替える言葉 → その説明。手の中の {{名前}} を入れ替える
	Steps   []recipeStep      `json:"steps"`
	Created time.Time         `json:"created"`
	Updated time.Time         `json:"updated"`
	Version int               `json:"version"`
	Runs    int               `json:"runs"`
	OKs     int               `json:"oks"`
	LastAt  time.Time         `json:"last_at,omitempty"`
	LastNG  string            `json:"last_ng,omitempty"` // 最後に止まった所（直すときの手がかり）
}

const (
	recipeMaxSteps = 40
	recipeMaxTime  = 120 * time.Second // リモートの道具は 140 秒で切れるので、その前に返す
)

var (
	recipeNameRe  = regexp.MustCompile(`^[\p{L}\p{N}_\-]{1,40}$`)
	recipeParamRe = regexp.MustCompile(`\{\{([\p{L}\p{N}_\-]{1,30})\}\}`)
	recipeMu      sync.Mutex
)

// 台本で使える道具（読む・ブラウザ・箱・画面の部品。消す・本物で変える・入れ子・画像は無し）
var recipeTools = map[string]bool{
	"browser_tabs": true, "browser_read": true, "browser_do": true, "browser_js": true,
	"pc_powershell": true, "pc_try": true, "pc_files": true, "pc_ui": true, "pc_ui_do": true, "pc_keys": true,
	"pc_open": true, "pc_windows": true, "pc_status": true, "pc_disk": true, "pc_processes": true, "pc_apps": true, "pc_problems": true,
}

func recipeDir() string        { return filepath.Join(handsUserDir(), "recipes") }
func recipeHistoryDir() string { return filepath.Join(recipeDir(), "history") }
func recipeFile(n string) string {
	return filepath.Join(recipeDir(), n+".json")
}

func loadRecipe(name string) (*recipe, error) {
	if !recipeNameRe.MatchString(name) {
		return nil, fmt.Errorf("台本の名前は文字・数字・_・- で 40 文字まで（%q）", name)
	}
	b, err := os.ReadFile(recipeFile(name))
	if err != nil {
		return nil, fmt.Errorf("台本「%s」はありません（hands action=recipes で一覧）", name)
	}
	var r recipe
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("台本「%s」が読めません：%v", name, err)
	}
	return &r, nil
}

func saveRecipeFile(r *recipe) error {
	_ = os.MkdirAll(recipeDir(), 0o755)
	b, _ := json.MarshalIndent(r, "", "  ")
	tmp := recipeFile(r.Name) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, recipeFile(r.Name))
}

// checkRecipeStep：保存するとき・流すときに、使えない手を止める
func checkRecipeStep(i int, st recipeStep) error {
	if !recipeTools[st.Tool] {
		return fmt.Errorf("%d 手目：%s は台本では使えません（使えるのは読む・ブラウザ・箱・画面の部品の道具。消す・本物で変える・画面を撮る・hands は使えない）", i+1, st.Tool)
	}
	if st.Tool == "pc_powershell" {
		if r, _ := st.Args["real"].(bool); r {
			return fmt.Errorf("%d 手目：本物で変える台本（real: true）は台本では使えません（変える台本は箱の中で動く）", i+1)
		}
	}
	if st.Tool == "pc_try" {
		if a, _ := st.Args["action"].(string); a != "" && a != "run" && a != "show" {
			return fmt.Errorf("%d 手目：pc_try は run・show だけ（offer・discard は Claude が見て決める）", i+1)
		}
	}
	return nil
}

func (a *Agent) recipeSave(name, about string, params map[string]string, raw json.RawMessage) Reply {
	if a.st.Mode() != ModeOperate {
		return errText("台本を保存するのは「操作も可」のときだけです")
	}
	if !recipeNameRe.MatchString(name) {
		return errText("name（台本の名前）は文字・数字・_・- で 40 文字まで")
	}
	if strings.TrimSpace(about) == "" {
		return errText("about（何をする台本か。1 行）が要ります。あとで探すときに使います")
	}
	var steps []recipeStep
	if err := json.Unmarshal(raw, &steps); err != nil || len(steps) == 0 {
		return errText("steps は [{\"tool\":\"browser_do\",\"args\":{…},\"note\":\"…\"}, …] の形で書く（%v）", err)
	}
	if len(steps) > recipeMaxSteps {
		return errText("台本の手は %d まで", recipeMaxSteps)
	}
	used := map[string]bool{}
	for i, st := range steps {
		if err := checkRecipeStep(i, st); err != nil {
			return errText("%v", err)
		}
		b, _ := json.Marshal(st.Args)
		for _, m := range recipeParamRe.FindAllStringSubmatch(string(b), -1) {
			used[m[1]] = true
		}
	}
	for p := range used {
		if _, ok := params[p]; !ok {
			return errText("手の中の {{%s}} が params にありません（入れ替える言葉と説明を params に書く）", p)
		}
	}
	r := &recipe{Name: name, About: oneLine(about, 120), Params: params, Steps: steps, Created: time.Now(), Updated: time.Now(), Version: 1}
	note := "新しく保存した"
	if old, err := loadRecipe(name); err == nil {
		// 前の版を残す（悪くなったら戻せるように）
		_ = os.MkdirAll(recipeHistoryDir(), 0o755)
		b, _ := json.MarshalIndent(old, "", "  ")
		_ = os.WriteFile(filepath.Join(recipeHistoryDir(), fmt.Sprintf("%s-v%d.json", name, old.Version)), b, 0o644)
		r.Created, r.Version, r.Runs, r.OKs = old.Created, old.Version+1, old.Runs, old.OKs
		note = fmt.Sprintf("直して保存した（版 %d → %d。前の版は recipes\\history に残した）", old.Version, r.Version)
	}
	if err := saveRecipeFile(r); err != nil {
		return errText("保存できません：%v", err)
	}
	return okText(fmt.Sprintf("台本「%s」を%s：%s（%d 手）。流すときは hands action=run name=%s%s", name, note, r.About, len(steps), name, paramHint(r)))
}

func paramHint(r *recipe) string {
	if len(r.Params) == 0 {
		return ""
	}
	var ps []string
	for k, v := range r.Params {
		ps = append(ps, fmt.Sprintf("%s（%s）", k, v))
	}
	sort.Strings(ps)
	return " values={…}：" + strings.Join(ps, "・")
}

func (a *Agent) recipeList() Reply {
	es, _ := os.ReadDir(recipeDir())
	var ls []string
	for _, e := range es {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		r, err := loadRecipe(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			continue
		}
		last := "まだ流していない"
		if r.Runs > 0 {
			last = fmt.Sprintf("%d 回流して %d 回うまくいった・最後 %s", r.Runs, r.OKs, r.LastAt.Format("01-02 15:04"))
		}
		ls = append(ls, fmt.Sprintf("・%s（版 %d・%d 手）：%s。%s%s", r.Name, r.Version, len(r.Steps), r.About, last, paramHint(r)))
	}
	if len(ls) == 0 {
		return okText("台本はまだありません（hands action=save で保存する）")
	}
	sort.Strings(ls)
	return okText(fmt.Sprintf("台本（%d。%s）：\n%s", len(ls), recipeDir(), strings.Join(ls, "\n")))
}

func (a *Agent) recipeShow(name string) Reply {
	r, err := loadRecipe(name)
	if err != nil {
		return errText("%v", err)
	}
	b, _ := json.MarshalIndent(r.Steps, "", "  ")
	l := []string{fmt.Sprintf("台本「%s」（版 %d・%d 手）：%s%s", r.Name, r.Version, len(r.Steps), r.About, paramHint(r))}
	if r.LastNG != "" {
		l = append(l, "最後に止まった所："+r.LastNG)
	}
	l = append(l, string(b))
	return okText(strings.Join(l, "\n"))
}

// fillParams：手の中の文字の {{名前}} を入れ替える（文字の値の中だけ）
func fillParams(v any, vals map[string]string) any {
	switch x := v.(type) {
	case string:
		return recipeParamRe.ReplaceAllStringFunc(x, func(m string) string {
			k := recipeParamRe.FindStringSubmatch(m)[1]
			if s, ok := vals[k]; ok {
				return s
			}
			return m
		})
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = fillParams(e, vals)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = fillParams(e, vals)
		}
		return out
	}
	return v
}

func (a *Agent) recipeRun(ctx context.Context, name string, vals map[string]string) Reply {
	r, err := loadRecipe(name)
	if err != nil {
		return errText("%v", err)
	}
	if ok, _ := a.st.Allowed(); ok {
		return errText("許可（ALLOW）が出ている間は台本を流しません（台本では許可を使わない決まり）。許可が切れてから（最長 10 分）流すか、1 手ずつふつうに動かしてください")
	}
	for k := range r.Params {
		if _, ok := vals[k]; !ok {
			return errText("values に %s が要ります%s", k, paramHint(r))
		}
	}
	if !recipeMu.TryLock() {
		return errText("ほかの台本が流れているところです（同時に 2 つは流さない）")
	}
	defer recipeMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, recipeMaxTime)
	defer cancel()
	start := time.Now()
	var log, shows []string
	var last Reply
	finish := func(ok bool, ng string) {
		r.Runs++
		if ok {
			r.OKs++
			r.LastNG = ""
		} else {
			r.LastNG = time.Now().Format("01-02 15:04 ") + ng
		}
		r.LastAt = time.Now()
		_ = saveRecipeFile(r)
	}
	for i, st := range r.Steps {
		if err := checkRecipeStep(i, st); err != nil {
			finish(false, err.Error())
			return errText("台本「%s」：%v", name, err)
		}
		args := fillParams(st.Args, vals)
		raw, _ := json.Marshal(args)
		t0 := time.Now()
		rep := a.call(ctx, st.Tool, raw)
		d := time.Since(t0)
		res := "ok"
		if rep.IsError {
			res = "NG: " + strings.SplitN(rep.Text, "\n", 2)[0]
		}
		a.st.Audit(st.Tool, "（台本 "+name+" の "+fmt.Sprint(i+1)+" 手目）"+string(raw), res, d)
		recordCall(st.Tool, raw, rep, d)
		label := st.Note
		if label == "" {
			label = st.Tool
		}
		if rep.IsError {
			why := oneLine(rep.Text, 600)
			switch {
			case stoppedBySwitch(ctx):
				why = stopMsg
			case ctx.Err() != nil:
				why = fmt.Sprintf("台本の時間（%d 秒）を過ぎた", int(recipeMaxTime.Seconds()))
			}
			ng := fmt.Sprintf("%d 手目（%s）：%s", i+1, label, why)
			finish(false, ng)
			log = append(log, fmt.Sprintf("%d. ✗ %s（%.1f 秒）", i+1, label, d.Seconds()))
			out := fmt.Sprintf("台本「%s」は %d 手目で止まりました（前の手は済んでいます）：\n%s\n\n止まった手の返事：\n%s", name, i+1, strings.Join(log, "\n"), clip(rep.Text, 3000))
			if rep.Lane == "E" {
				out += "\n（最後の一押しはカオル。ここまで進めた状態をカオルに伝える）"
			} else if strings.Contains(rep.Text, "カオルの許可が要ります") {
				out += "\n（台本では許可を使いません。この手は台本から外すか、1 手ずつふつうに動かす）"
			} else {
				out += "\n（ページの形が変わったなどなら、この手だけ直して hands action=save で保存し直す）"
			}
			return Reply{Text: out, IsError: true, Lane: rep.Lane, What: rep.What}
		}
		log = append(log, fmt.Sprintf("%d. %s（%.1f 秒）", i+1, label, d.Seconds()))
		if st.Show {
			shows = append(shows, fmt.Sprintf("—— %d 手目（%s）の返事 ——\n%s", i+1, label, clip(rep.Text, 3000)))
		}
		last = rep
	}
	finish(true, "")
	out := fmt.Sprintf("台本「%s」（版 %d）を最後まで流しました（%d 手・%.1f 秒。Claude は途中で考えていない）：\n%s", name, r.Version, len(r.Steps), time.Since(start).Seconds(), strings.Join(log, "\n"))
	if len(shows) > 0 {
		out += "\n\n" + strings.Join(shows, "\n\n")
	}
	if !r.Steps[len(r.Steps)-1].Show {
		out += "\n\n—— 最後の手の返事 ——\n" + clip(last.Text, 6000)
	}
	return Reply{Text: out}
}
