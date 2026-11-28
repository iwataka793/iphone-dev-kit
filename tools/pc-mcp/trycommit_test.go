package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// 本物に入れる（TRY の［本物に入れる］）：全部入れるか何も入れないか・承認後の変化・取り置きの失敗・途中の失敗と元に戻す
type commitEnv struct {
	t        *testing.T
	a        *Agent
	real     string // 本物の場所（ユーザーフォルダの tools）
	ub       string // 箱の中の同じ場所
	box      string
	holdDir  string
	holdFail map[string]bool // この名前のものは取り置きへ送れない
	sig      string
}

func newCommitEnv(t *testing.T) *commitEnv {
	if runtime.GOOS == "windows" {
		t.Skip("Windows では本物の Sandboxie で試す")
	}
	e := &commitEnv{t: t, holdFail: map[string]bool{}}
	home := t.TempDir()
	e.box = t.TempDir()
	e.holdDir = t.TempDir()
	keyf := filepath.Join(t.TempDir(), "request.key")
	_ = os.WriteFile(keyf, []byte("k1"), 0o600)
	fake := filepath.Join(t.TempDir(), "fake.sh")
	_ = os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HANDS_BOX_ROOT", e.box)
	t.Setenv("HANDS_SBIE_START", fake)
	t.Setenv("HANDS_KEY_FILE", keyf)
	t.Setenv("PC_HANDS_USERDIR", filepath.Join(home, "devkit", "hands"))
	e.real = filepath.Join(home, "tools")
	e.ub = filepath.Join(e.box, "user", "current", "tools")
	old := time.Now().Add(-time.Hour)
	for n, v := range map[string]string{"change.txt": "before", "remove.txt": "real", "keep.txt": "same"} {
		e.write(filepath.Join(e.real, n), v)
		_ = os.Chtimes(filepath.Join(e.real, n), old, old)
	}
	e.write(filepath.Join(e.ub, "change.txt"), "after!")
	e.write(filepath.Join(e.ub, "new.txt"), "new")
	e.write(filepath.Join(e.ub, "remove.txt"), "")
	mark := time.Date(1986, 5, 24, 0, 47, 2, 0, time.Local)
	_ = os.Chtimes(filepath.Join(e.ub, "remove.txt"), mark, mark)
	e.a = &Agent{st: NewState(t.TempDir(), time.Minute), ps: NewRunner()}
	_ = os.WriteFile(e.a.st.path("mode.txt"), []byte("operate"), 0o644)
	n := 0
	e.a.holdFn = func(_ context.Context, paths []string) (map[string]heldItem, string) {
		held := map[string]heldItem{}
		var txt []string
		for _, p := range paths {
			if e.holdFail[filepath.Base(p)] {
				txt = append(txt, "ERROR: "+p+"：送れない（試験）")
				continue
			}
			n++
			d := filepath.Join(e.holdDir, fmt.Sprint(n))
			_ = os.MkdirAll(d, 0o755)
			if err := os.Rename(p, filepath.Join(d, filepath.Base(p))); err != nil {
				continue
			}
			held[strings.ToLower(p)] = heldItem{p, d}
		}
		return held, strings.Join(txt, "\n")
	}
	items, _, _, err := scanBox()
	if err != nil || len(items) != 3 {
		t.Fatalf("箱の中身：%v %v", items, err)
	}
	s := &tryState{ID: "c1", Started: time.Now().Add(-time.Minute), Status: "waiting", Items: items}
	if err := saveTry(s); err != nil {
		t.Fatal(err)
	}
	m := hmac.New(sha256.New, []byte("k1"))
	m.Write([]byte(trySignText("c1", "apply")))
	e.sig = hex.EncodeToString(m.Sum(nil))
	return e
}

func (e *commitEnv) write(p, s string) {
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *commitEnv) apply() Reply {
	b, _ := json.Marshal(map[string]any{"action": "apply", "id": "c1", "sig": e.sig})
	return e.a.try(context.Background(), b)
}

func (e *commitEnv) read(n string) string {
	b, err := os.ReadFile(filepath.Join(e.real, n))
	if err != nil {
		return "（無い）"
	}
	return string(b)
}

// 本物が試しの前のまま（何も入っていない）か
func (e *commitEnv) untouched(why string) {
	e.t.Helper()
	if e.read("change.txt") != "before" || e.read("remove.txt") != "real" || e.read("new.txt") != "（無い）" || e.read("keep.txt") != "same" {
		e.t.Errorf("%s：本物が変わった（change=%q remove=%q new=%q）", why, e.read("change.txt"), e.read("remove.txt"), e.read("new.txt"))
	}
	es, _ := os.ReadDir(e.real)
	for _, x := range es {
		if strings.HasSuffix(x.Name(), ".claude-hands-try") {
			e.t.Errorf("%s：一時のファイルが残った %s", why, x.Name())
		}
	}
}

func TestTryCommitOK(t *testing.T) {
	e := newCommitEnv(t)
	r := e.apply()
	if r.IsError || !strings.Contains(r.Text, "入れた 2") {
		t.Fatalf("入れられない：%s", r.Text)
	}
	if e.read("change.txt") != "after!" || e.read("new.txt") != "new" || e.read("remove.txt") != "（無い）" {
		t.Errorf("入れた結果が違う")
	}
	if s, _ := loadTry(); s.Status != "applied" {
		t.Errorf("状態 %s", s.Status)
	}
	// DONE の報告に「箱で試したこと」が出る（本物に入れた結果まで）
	l := strings.Join(tryDoneLines(time.Now().Add(-time.Hour), time.Now().Add(time.Minute)), "\n")
	if !strings.Contains(l, "c1") || !strings.Contains(l, "本物に入れた") || !strings.Contains(l, "入れた 2") {
		t.Errorf("DONE に試しが出ない：%s", l)
	}
	// 試しのあとに始めた作業には出ない
	if l := tryDoneLines(time.Now().Add(time.Hour), time.Now().Add(2*time.Hour)); len(l) != 0 {
		t.Errorf("関係ない作業に試しが出た：%v", l)
	}
}

// ①：窓に出したあとで箱の中身が変わったら（大きさが同じでも）入れない。もう一度押してもらう
func TestTryCommitChangedAfterOffer(t *testing.T) {
	e := newCommitEnv(t)
	e.write(filepath.Join(e.ub, "change.txt"), "AFTER?") // 同じ 6 バイト
	r := e.apply()
	if !r.IsError || !strings.Contains(r.Text, "変わっていました") {
		t.Fatalf("中身が変わったのに入れた：%s", r.Text)
	}
	e.untouched("承認後の変化")
	// 新しくした一覧で、もう一度押せば入る
	if r := e.apply(); r.IsError || e.read("change.txt") != "AFTER?" {
		t.Errorf("押し直しで入らない：%s", r.Text)
	}
}

// ④：消すはずのものが試しのあとで本物で変わっていたら、何も入れない
func TestTryCommitDeleteConflict(t *testing.T) {
	e := newCommitEnv(t)
	e.write(filepath.Join(e.real, "remove.txt"), "kaoru edited")
	r := e.apply()
	if !r.IsError || !strings.Contains(r.Text, "一部だけ入れることはしない") {
		t.Fatalf("食い違いがあるのに入れた：%s", r.Text)
	}
	if e.read("change.txt") != "before" || e.read("new.txt") != "（無い）" || e.read("remove.txt") != "kaoru edited" {
		t.Errorf("食い違いで止めたのに本物が変わった")
	}
}

// ②④：取り置きへ 1 つでも送れなければ、送った分を戻して何も入れない
func TestTryCommitHoldFails(t *testing.T) {
	e := newCommitEnv(t)
	e.holdFail["remove.txt"] = true
	r := e.apply()
	if !r.IsError || !strings.Contains(r.Text, "何も入れずに止めました") || !strings.Contains(r.Text, "元を戻した 1") {
		t.Fatalf("取り置きに送れないのに進んだ／戻した結果が出ない：%s", r.Text)
	}
	e.untouched("取り置きの失敗")
}

// ④：置き換えの途中で失敗したら、入れた分を外して元に戻す（戻した結果も返事に出す）
func TestTryCommitRenameFailsMidway(t *testing.T) {
	e := newCommitEnv(t)
	calls := 0
	tryRename = func(a, b string) error {
		calls++
		if calls == 2 {
			return errors.New("使用中（試験）")
		}
		return os.Rename(a, b)
	}
	defer func() { tryRename = os.Rename }()
	r := e.apply()
	if !r.IsError || !strings.Contains(r.Text, "元に戻しました") || !strings.Contains(r.Text, "元を戻した 2（中身を確かめた）") {
		t.Fatalf("途中の失敗で戻さない：%s", r.Text)
	}
	e.untouched("途中の失敗")
	if s, _ := loadTry(); s.Status != "waiting" {
		t.Errorf("途中で止めたのに状態が %s", s.Status)
	}
}
