package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newRecipeAgent(t *testing.T) (*Agent, string) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	t.Setenv("PC_HANDS_USERDIR", filepath.Join(home, "devkit", "hands"))
	t.Setenv("PC_HANDS_DIR", t.TempDir())
	t.Setenv("PC_AGENT_ALLOW_DIR", t.TempDir())
	st := NewState(t.TempDir(), time.Minute)
	a := &Agent{st: st, bst: NewState(t.TempDir(), time.Minute), ps: NewRunner()}
	_ = os.WriteFile(st.path("mode.txt"), []byte("operate"), 0o644)
	work := filepath.Join(home, "work")
	_ = os.MkdirAll(work, 0o755)
	_ = os.WriteFile(filepath.Join(work, "memo-a.txt"), []byte("a"), 0o644)
	return a, work
}

func handsCall(a *Agent, m map[string]any) Reply {
	b, _ := json.Marshal(m)
	return a.hands(context.Background(), b)
}

// 台本：保存の決まり（使えない手・本物で変える・入れ替える言葉の書き忘れ）
func TestRecipeSaveRules(t *testing.T) {
	a, _ := newRecipeAgent(t)
	bad := []struct {
		steps []map[string]any
		want  string
	}{
		{[]map[string]any{{"tool": "pc_trash", "args": map[string]any{"paths": []string{"x"}}}}, "台本では使えません"},
		{[]map[string]any{{"tool": "hands", "args": map[string]any{"action": "run"}}}, "台本では使えません"},
		{[]map[string]any{{"tool": "pc_screenshot", "args": map[string]any{}}}, "台本では使えません"},
		{[]map[string]any{{"tool": "pc_powershell", "args": map[string]any{"script": "x", "real": true}}}, "real"},
		{[]map[string]any{{"tool": "pc_try", "args": map[string]any{"action": "offer"}}}, "run・show だけ"},
		{[]map[string]any{{"tool": "pc_files", "args": map[string]any{"path": "{{dir}}"}}}, "params にありません"},
	}
	for _, c := range bad {
		r := handsCall(a, map[string]any{"action": "save", "name": "x", "about": "試し", "steps": c.steps})
		if !r.IsError || !strings.Contains(r.Text, c.want) {
			t.Errorf("%v：%s", c.steps, r.Text)
		}
	}
	if r := handsCall(a, map[string]any{"action": "save", "name": "a/b", "about": "x", "steps": []map[string]any{{"tool": "pc_status"}}}); !r.IsError {
		t.Errorf("名前にスラッシュを許した")
	}
}

// 台本：保存 → 流す（入れ替え・最後の手の返事）→ 直すと版が上がり前の版が残る → 一覧・中身
func TestRecipeRun(t *testing.T) {
	a, work := newRecipeAgent(t)
	steps := []map[string]any{
		{"tool": "pc_files", "args": map[string]any{"path": "{{dir}}"}, "note": "フォルダを見る"},
		{"tool": "pc_files", "args": map[string]any{"action": "find", "path": "{{dir}}", "pattern": "memo-*.txt"}, "note": "メモを探す", "show": true},
	}
	if r := handsCall(a, map[string]any{"action": "save", "name": "メモ探し", "about": "フォルダのメモを探す", "params": map[string]string{"dir": "探すフォルダ"}, "steps": steps}); r.IsError {
		t.Fatalf("保存できない：%s", r.Text)
	}
	if r := handsCall(a, map[string]any{"action": "run", "name": "メモ探し"}); !r.IsError || !strings.Contains(r.Text, "values に dir") {
		t.Errorf("言葉なしで流した：%s", r.Text)
	}
	r := handsCall(a, map[string]any{"action": "run", "name": "メモ探し", "values": map[string]string{"dir": work}})
	if r.IsError || !strings.Contains(r.Text, "最後まで流しました") || !strings.Contains(r.Text, "memo-a.txt") {
		t.Fatalf("流せない：%s", r.Text)
	}
	// TIMELINE に 1 手ずつ残る
	es := entries(time.Now().Add(-time.Minute), time.Now().Add(time.Minute), "")
	if n := len(es); n < 2 {
		t.Errorf("TIMELINE に手が残っていない：%d", n)
	}
	// 直して保存 → 版 2・前の版が残る・回数は引き継ぐ
	if r := handsCall(a, map[string]any{"action": "save", "name": "メモ探し", "about": "フォルダのメモを探す（直した）", "params": map[string]string{"dir": "探すフォルダ"}, "steps": steps[1:]}); r.IsError || !strings.Contains(r.Text, "版 1 → 2") {
		t.Errorf("直した保存：%s", r.Text)
	}
	if _, err := os.Stat(filepath.Join(recipeHistoryDir(), "メモ探し-v1.json")); err != nil {
		t.Errorf("前の版が残っていない")
	}
	if r := handsCall(a, map[string]any{"action": "recipes"}); !strings.Contains(r.Text, "メモ探し（版 2・1 手）") || !strings.Contains(r.Text, "1 回流して 1 回うまくいった") {
		t.Errorf("一覧：%s", r.Text)
	}
	if r := handsCall(a, map[string]any{"action": "recipe", "name": "メモ探し"}); !strings.Contains(r.Text, "memo-*.txt") {
		t.Errorf("中身：%s", r.Text)
	}
}

// 台本：失敗した手で止まる（後の手は動かない）・止まった所を残す。許可が出ている間は流さない
func TestRecipeStops(t *testing.T) {
	a, work := newRecipeAgent(t)
	steps := []map[string]any{
		{"tool": "pc_files", "args": map[string]any{"path": work}, "note": "見る"},
		{"tool": "browser_tabs", "args": map[string]any{}, "note": "タブ（ブラウザはオフ）"},
		{"tool": "pc_files", "args": map[string]any{"path": filepath.Join(work, "never")}, "note": "ここは動かない"},
	}
	if r := handsCall(a, map[string]any{"action": "save", "name": "止まる", "about": "途中で止まる", "steps": steps}); r.IsError {
		t.Fatal(r.Text)
	}
	r := handsCall(a, map[string]any{"action": "run", "name": "止まる"})
	if !r.IsError || !strings.Contains(r.Text, "2 手目で止まりました") || strings.Contains(r.Text, "ここは動かない") {
		t.Fatalf("止まらない：%s", r.Text)
	}
	if rr, _ := loadRecipe("止まる"); rr.Runs != 1 || rr.OKs != 0 || !strings.Contains(rr.LastNG, "2 手目") {
		t.Errorf("止まった所が残っていない：%+v", rr)
	}
	// 許可が出ている間は流さない
	_ = os.WriteFile(allowFile(), []byte("allow"), 0o644)
	if r := handsCall(a, map[string]any{"action": "run", "name": "止まる"}); !r.IsError || !strings.Contains(r.Text, "許可（ALLOW）が出ている間") {
		t.Errorf("許可の間に流した：%s", r.Text)
	}
}

func TestFillParams(t *testing.T) {
	v := fillParams(map[string]any{"a": "x {{q}} y", "b": []any{"{{q}}", 3.0, "{{nope}}"}}, map[string]string{"q": "ねこ"})
	b, _ := json.Marshal(v)
	if string(b) != `{"a":"x ねこ y","b":["ねこ",3,"{{nope}}"]}` {
		t.Errorf("%s", b)
	}
}
