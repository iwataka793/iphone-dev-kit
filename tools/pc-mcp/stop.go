package main

// STOP（スイッチがオフ）で、動いている処理も止める（2026-11-28 直し）。
// 今までは、スイッチがオフになっても、動いている PowerShell の台本（最大 10 分）と箱の中のものは終わるまで動いていた。
//
//   - PC の道具は「PC のスイッチの間だけ生きる」文脈で動かす。PC がオフ（STOP・切り替え・使わない時間）になったら切る
//     → PowerShell の台本は途中で止まる（常駐の PowerShell を起動し直す）。箱の中で動いているものも止める（中身は残る）
//   - ブラウザの道具は、ブラウザのスイッチで同じようにする
//   - 止めないもの：控え・戻しにかかわる短い処理（pc_trash＝取り置きへ写してから消す・hands＝まとめて戻す・
//     カオルの TRY の窓の［本物に入れる］［捨てる］。台本を流す hands action=run は止める）。途中で切ると、元が消えたのに取り置きの記録が返らない、
//     半分だけ入る、などになるので最後まで終わらせる（どれも短い）
//   - 「本物で」変える台本（道 C）を途中で止めたときも、後の確かめ（エラーなら決まったフォルダを控えに戻す）は切らずにやる

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

type stopScope struct {
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	on     bool
}

func (s *stopScope) get() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx == nil {
		s.ctx, s.cancel = context.WithCancel(context.Background())
	}
	return s.ctx
}

// set：スイッチの今を知らせる。オンからオフに変わったら、動いているものを切って true
func (s *stopScope) set(on bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	was := s.on
	s.on = on
	if was && !on && s.cancel != nil {
		s.cancel()
		s.ctx, s.cancel = context.WithCancel(context.Background())
		return true
	}
	return false
}

// keepRunning：STOP でも途中で切らない道具（控え・戻しにかかわる短い処理）
func keepRunning(tool string, raw json.RawMessage) bool {
	switch tool {
	case "pc_trash":
		return true
	case "hands":
		// 台本を流す（run）は STOP で止める。ほか（start・done・undo など）は短いので最後まで
		var in struct {
			Action string `json:"action"`
		}
		_ = json.Unmarshal(raw, &in)
		return strings.ToLower(in.Action) != "run"
	case "pc_try":
		var in struct {
			Action string `json:"action"`
		}
		_ = json.Unmarshal(raw, &in)
		switch strings.ToLower(in.Action) {
		case "apply", "discard":
			return true
		}
	}
	return false
}

// callCtx：道具を動かす文脈。スイッチがオフになったら切れる（keepRunning の道具は切れない）
func (a *Agent) callCtx(parent context.Context, tool string, raw json.RawMessage) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	if keepRunning(tool, raw) {
		return ctx, cancel
	}
	sc := &a.pcStop
	if isBrowserTool(tool) {
		sc = &a.brStop
	}
	stop := context.AfterFunc(sc.get(), cancel)
	return ctx, func() { stop(); cancel() }
}

// watchStop：2 秒ごとの見回りから呼ぶ。PC がオフになったら、箱の中で動いているものも止める
func (a *Agent) watchStop(pcOn, brOn bool) {
	if a.pcStop.set(pcOn) {
		a.st.Audit("（STOP）", "PC のスイッチがオフになったので、動いている台本と箱の中のものを止めた", "ok", 0)
		if fileExists(sbieStart()) {
			c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_ = sbie(c, 30*time.Second, "/box:"+tryBox, "/terminate")
			cancel()
		}
	}
	a.brStop.set(brOn)
}

// stoppedBySwitch：この文脈がスイッチのオフで切れたか
func stoppedBySwitch(ctx context.Context) bool { return ctx.Err() == context.Canceled }

const stopMsg = "STOP（スイッチがオフ）で途中で止めました"
