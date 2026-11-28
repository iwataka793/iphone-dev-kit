package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ⑤：STOP（PC のスイッチがオフ）で、動いている PC の道具は切れる。控え・戻しの道具とブラウザは切れない
func TestStopScope(t *testing.T) {
	a := &Agent{}
	a.pcStop.set(true)
	a.brStop.set(true)
	ps, c1 := a.callCtx(context.Background(), "pc_powershell", json.RawMessage(`{}`))
	defer c1()
	tr, c2 := a.callCtx(context.Background(), "pc_trash", json.RawMessage(`{}`))
	defer c2()
	ap, c3 := a.callCtx(context.Background(), "pc_try", json.RawMessage(`{"action":"apply"}`))
	defer c3()
	run, c4 := a.callCtx(context.Background(), "pc_try", json.RawMessage(`{"action":"run"}`))
	defer c4()
	br, c5 := a.callCtx(context.Background(), "browser_do", json.RawMessage(`{}`))
	defer c5()
	rr, c7 := a.callCtx(context.Background(), "hands", json.RawMessage(`{"action":"run","name":"x"}`))
	defer c7()
	hd, c8 := a.callCtx(context.Background(), "hands", json.RawMessage(`{"action":"undo"}`))
	defer c8()
	if a.pcStop.set(true) {
		t.Fatal("オンのままなのに切った")
	}
	if !a.pcStop.set(false) {
		t.Fatal("オフにしても切らない")
	}
	time.Sleep(10 * time.Millisecond)
	if !stoppedBySwitch(ps) || !stoppedBySwitch(run) {
		t.Errorf("動いている台本・箱の試しが切れない")
	}
	if tr.Err() != nil || ap.Err() != nil {
		t.Errorf("取り置き・本物に入れるのを途中で切った")
	}
	if !stoppedBySwitch(rr) || hd.Err() != nil {
		t.Errorf("台本（hands run）は止まり、まとめて戻す（undo）は止まらないはず")
	}
	if br.Err() != nil {
		t.Errorf("PC の STOP でブラウザまで切れた")
	}
	// オフのあとに始めたものは、次のオフまで生きる
	a.pcStop.set(true)
	nx, c6 := a.callCtx(context.Background(), "pc_powershell", nil)
	defer c6()
	if nx.Err() != nil {
		t.Errorf("新しい呼び出しが始めから切れている")
	}
}

// ⑤：箱で試している途中の STOP：箱の中のものを止め、ここまでの記録（箱の中身・試しの記録）は残す
func TestTryStopMidRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows では本物の Sandboxie で試す")
	}
	home := t.TempDir()
	box := t.TempDir()
	log := filepath.Join(t.TempDir(), "calls.txt")
	made := filepath.Join(box, "user", "current", "work", "half.txt") // 箱の中で、止める前にできたもの
	fake := filepath.Join(t.TempDir(), "fake.sh")
	_ = os.WriteFile(fake, []byte("#!/bin/sh\necho \"$*\" >> "+log+"\ncase \"$*\" in *\"/wait\"*) mkdir -p "+filepath.Dir(made)+"; echo half > "+made+"; exec sleep 30;; esac\nexit 0\n"), 0o755)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HANDS_BOX_ROOT", box)
	t.Setenv("HANDS_SBIE_START", fake)
	t.Setenv("PC_HANDS_USERDIR", filepath.Join(home, "devkit", "hands"))
	a := &Agent{st: NewState(t.TempDir(), time.Minute), ps: NewRunner()}
	_ = os.WriteFile(a.st.path("mode.txt"), []byte("operate"), 0o644)
	a.insp = func(string) (scriptInfo, error) { return scriptInfo{Cmds: []string{"Set-Content"}}, nil }
	a.pcStop.set(true)
	raw := json.RawMessage(`{"action":"run","script":"Set-Content x y","seconds":60}`)
	ctx, cancel := a.callCtx(context.Background(), "pc_try", raw)
	defer cancel()
	done := make(chan Reply, 1)
	t0 := time.Now()
	go func() { done <- a.try(ctx, raw) }()
	time.Sleep(500 * time.Millisecond)
	a.pcStop.set(false)
	var r Reply
	select {
	case r = <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("STOP しても箱の試しが止まらない")
	}
	if time.Since(t0) > 10*time.Second || !strings.Contains(r.Text, "STOP") {
		t.Errorf("止めた返事が違う（%v）：%s", time.Since(t0), r.Text)
	}
	if !strings.Contains(r.Text, "half.txt") {
		t.Errorf("ここまでにできたものが返事に無い：%s", r.Text)
	}
	if b, _ := os.ReadFile(log); !strings.Contains(string(b), "/terminate") {
		t.Errorf("箱の中のものを止めていない：%s", b)
	}
	s, err := loadTry()
	if err != nil || len(s.Runs) != 1 || !s.Runs[0].Stopped || s.Status != "working" {
		t.Errorf("試しの記録が残っていない：%+v %v", s, err)
	}
	if _, err := os.Stat(made); err != nil {
		t.Errorf("箱の中身が消えた")
	}
}
