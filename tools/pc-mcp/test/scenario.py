"""PC の道具を通しで試す（GitHub の Windows と、手元の Linux＋pwsh の両方で動く）。

環境変数：
  PCMCP   拡張機能の exe（pc-mcp.exe）
  PCAGENT 常駐役の exe（pc-agent.exe）
  PC_AGENT_DIR / PC_AGENT_ALLOW_DIR  置き場所（試験用の一時フォルダ）
"""
import json, os, subprocess, sys, time, pathlib, tempfile, shutil, threading, http.server, functools


def run_ps(script):
    try:
        return subprocess.run(["powershell", "-NoProfile", "-Command", script], capture_output=True, text=True, timeout=60).stdout
    except Exception as e:
        return f"err {e}"

WIN = os.name == "nt"
MCP = os.environ["PCMCP"]
AGENT = os.environ["PCAGENT"]
SD = pathlib.Path(os.environ["PC_AGENT_DIR"])
AD = pathlib.Path(os.environ["PC_AGENT_ALLOW_DIR"])
SD.mkdir(parents=True, exist_ok=True)
AD.mkdir(parents=True, exist_ok=True)
fails = []
# ブラウザの試験：試験用の Chrome（一時のプロフィール）を常駐役に教えておく
CHROME_PROF = pathlib.Path(tempfile.gettempdir()) / "pc-mcp-chrome"
shutil.rmtree(CHROME_PROF, ignore_errors=True)
os.environ["PC_BROWSER_USERDATA"] = str(CHROME_PROF)
os.environ["PC_BROWSER_PROFILE"] = "kaoru"   # 既定は Claude 用の Chrome。今までの試験は「カオルの Chrome」（試験用の一時のもの）で
CLAUDE_CHROME = pathlib.Path(tempfile.gettempdir()) / "pc-mcp-chrome-claude"
shutil.rmtree(CLAUDE_CHROME, ignore_errors=True)
os.environ["PC_CLAUDE_CHROME_DIR"] = str(CLAUDE_CHROME)
os.environ["PC_CLAUDE_CHROME_ARGS"] = "--headless=new --no-sandbox --disable-gpu"
BD = pathlib.Path(tempfile.gettempdir()) / "pc-mcp-browser-switch"
shutil.rmtree(BD, ignore_errors=True)
BD.mkdir(parents=True, exist_ok=True)
os.environ["PC_BROWSER_DIR"] = str(BD)
# 取り置き（試験では守りの権限なしの一時フォルダ）
HD = pathlib.Path(tempfile.gettempdir()) / "pc-mcp-hands"
shutil.rmtree(HD, ignore_errors=True)
(HD / "holding").mkdir(parents=True, exist_ok=True)
os.environ["PC_HANDS_DIR"] = str(HD)
os.environ["PC_HANDS_FREE_GB"] = "200"  # 試験の機械の C は空きが少ないので
HU = pathlib.Path(tempfile.gettempdir()) / "pc-mcp-hands-user"
shutil.rmtree(HU, ignore_errors=True)
HU.mkdir(parents=True, exist_ok=True)
os.environ["PC_HANDS_USERDIR"] = str(HU)
GT = pathlib.Path.home() / "pc-mcp-hands-test"   # Git で記録するフォルダ（ホームの中）
shutil.rmtree(GT, ignore_errors=True)
GT.mkdir(parents=True, exist_ok=True)
(GT / "keep.txt").write_text("before\n")
(HU / "git-folders.txt").write_text("pc-mcp-hands-test\n")
# 常駐役はカオルの PC では devkit\switch-app（ホームの中）から起動される。試験でもホームの中のフォルダから起動する
HERE = pathlib.Path(__file__).resolve().parent  # chdir の前に
AGENT_CWD = pathlib.Path.home() / "pc-mcp-agent-cwd"
AGENT_CWD.mkdir(parents=True, exist_ok=True)
os.chdir(AGENT_CWD)


def call(name, args=None, timeout=300):
    msgs = [
        {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "t", "version": "0"}}},
        {"jsonrpc": "2.0", "method": "notifications/initialized"},
        {"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": {"name": name, "arguments": args or {}}},
    ]
    p = subprocess.Popen([MCP], stdin=subprocess.PIPE, stdout=subprocess.PIPE)
    for m in msgs:
        p.stdin.write((json.dumps(m, ensure_ascii=False) + "\n").encode("utf-8"))
        p.stdin.flush()
    t0 = time.time()
    res = None
    while time.time() - t0 < timeout:
        l = p.stdout.readline()
        if not l:
            break
        m = json.loads(l)
        if m.get("id") == 2:
            res = m
            break
    dt = time.time() - t0
    p.stdin.close()
    p.kill()
    if res is None:
        return True, "（返事なし）", False, dt
    r = res.get("result", {})
    texts = [c.get("text", "") for c in r.get("content", []) if c.get("type") == "text"]
    img = any(c.get("type") == "image" for c in r.get("content", []))
    return bool(r.get("isError")), "\n".join(texts), img, dt


def check(label, name, args, want_err, contains=None, show=600):
    err, text, img, dt = call(name, args)
    ok = (err == want_err) and (contains is None or contains in text)
    mark = "OK " if ok else "NG "
    print(f"{mark}{label}（{name}・{dt:.1f} 秒・エラー={err}・画像={img}）")
    print("    " + text[:show].replace("\n", "\n    "))
    if not ok:
        fails.append(label)
    return err, text


def look(label, name, args):
    err, text, img, dt = call(name, args)
    print(f"-- {label}（{name}・{dt:.1f} 秒・エラー={err}・画像={img}）")
    print("    " + text[:1500].replace("\n", "\n    "))
    return err, text


def set_mode(m):
    (SD / "mode.txt").write_text(m + "\r\n")
    time.sleep(1.2)


def set_bmode(m):
    (BD / "mode.txt").write_text(m + "\r\n")
    time.sleep(1.2)


def start_agent():
    flags = 0x08000000 if WIN else 0
    subprocess.Popen([AGENT, "agent"], creationflags=flags)
    for _ in range(50):
        if (SD / "token.txt").exists():
            return
        time.sleep(0.2)
    print("常駐役が起動しない")
    fails.append("常駐役の起動")


tmp = pathlib.Path(tempfile.gettempdir()) / "pc-mcp-test"
tmp.mkdir(exist_ok=True)

print("=== 1. オフ（常駐役なし）")
check("状態", "pc_status", {}, False, "動いていない")
check("オフなら調べられない", "pc_disk", {}, True, "オフ")

print("=== 2. 見るだけ")
set_mode("view")
start_agent()
check("状態", "pc_status", {}, False, "見るだけ")
check("容量（ドライブと膨らみやすい場所）", "pc_disk", {"seconds": 60}, False, "■ ドライブ" if WIN else "■")
check("容量（一時フォルダの中・大きいファイル）", "pc_disk", {"path": str(tmp.parent), "files": True, "top": 5, "seconds": 60}, False, "の中")
check("不具合の記録", "pc_problems", {"hours": 24, "max": 5}, False, "■ イベントログ" if WIN else None)
(check if WIN else look)("プロセス（メモリ）", "pc_processes", {"top": 5}, *([False] if WIN else []))
check("プロセス（CPU）", "pc_processes", {"sort": "cpu", "top": 5}, False)
check("アプリ", "pc_apps", {"max": 5}, False, "■" if WIN else None)
(check if WIN else look)("起動時に動くもの", "pc_apps", {"kind": "startup"}, *([False] if WIN else []))
check("ファイルの一覧", "pc_files", {"action": "list", "path": str(tmp.parent), "max": 5}, False)
check("PowerShell（読む）", "pc_powershell", {"script": "Get-Process | Sort-Object WS -Descending | Select-Object -First 3 -ExpandProperty ProcessName"}, False)
check("PowerShell（速さ：2 回目）", "pc_powershell", {"script": "1+1"}, False, "2")
check("見るだけでは作れない", "pc_powershell", {"script": f"New-Item -ItemType File '{tmp / 'a.txt'}'"}, True, "見るだけ")
check("見るだけではごみ箱に送れない", "pc_trash", {"paths": [str(tmp / "a.txt")]}, True, "見るだけ")

print("=== 3. 操作も可")
set_mode("operate")
f = tmp / "trash-me.txt"
check("ファイルを作る（本物で）", "pc_powershell", {"script": f"Set-Content -Path '{f}' -Value 'test'; Test-Path '{f}'", "real": True}, False, "True")
nb = tmp / "box-only.txt"
check("変える台本は既定で箱（箱が無ければ動かさない）", "pc_powershell", {"script": f"Set-Content -Path '{nb}' -Value 'x'"}, True, "Sandboxie")
if nb.exists():
    fails.append("箱で動かすはずの台本が本物を変えた")
check("完全に消すのは許可が要る（本物で）", "pc_powershell", {"script": f"Remove-Item '{f}'", "real": True}, True, "許可")
check("資格情報は許可が要る", "pc_powershell", {"script": "cmdkey /list", "real": True}, True, "許可")
check("資格情報は箱でも許可が要る", "pc_powershell", {"script": "cmdkey /list"}, True, "許可")
check("外への通信は許可が要る（本物で）", "pc_powershell", {"script": "Invoke-WebRequest https://example.com -UseBasicParsing", "real": True}, True, "許可")
check("鍵の場所は箱でも許可が要る", "pc_try", {"script": r"Get-Content C:\Users\x\.ssh\id_rsa"}, True, "許可")
check("スイッチの置き場所は触れない", "pc_powershell", {"script": f"Get-Content '{SD / 'mode.txt'}'"}, True, "置き場所")
check("許可の置き場所は触れない", "pc_powershell", {"script": f"Set-Content '{AD / 'allow.txt'}' x"}, True, "置き場所")
check("箱（Sandboxie）の設定は触れない", "pc_powershell", {"script": r"& 'C:\Program Files\Sandboxie-Plus\SbieIni.exe' set ClaudeBox BlockNetworkFiles n"}, True, "箱（Sandboxie）")
check("箱の設定の窓は開けない", "pc_open", {"target": r"C:\Program Files\Sandboxie-Plus\SandMan.exe"}, True, "箱（Sandboxie）")
check("落としたものを箱の外で動かさない", "pc_powershell", {"script": r"Start-Process 'C:\Users\x\devkit\downloads\setup.exe'", "real": True}, True, "pc_try")
check("落としたものを開かない", "pc_open", {"target": r"C:\Users\x\devkit\downloads\setup.exe"}, True, "pc_try")
check("箱の中身は台本から触らない", "pc_powershell", {"script": r"Copy-Item C:\Sandbox\x\ClaudeBox\a.txt C:\Users\x\a.txt"}, True, "TRY の窓")
check("箱が無ければ作業場も動かさない", "pc_try", {"script": "Write-Output hi"}, True, "Sandboxie")
check("本物に入れるのは TRY の窓だけ", "pc_try", {"action": "apply", "id": "x"}, True, "TRY の窓")
check("鍵の場所の中身は許可が要る", "pc_files", {"action": "read", "path": "~/.ssh/id_rsa"}, True, "許可")
if WIN:
    check("取り置きに写してから元を消す（ごみ箱は通さない）", "pc_trash", {"paths": [str(f)]}, False, "ごみ箱は通さない")
    print("    残っているか：", f.exists())
    if f.exists():
        fails.append("消したのに残っている")
    rb = run_ps("(New-Object -ComObject Shell.Application).NameSpace(10).Items() | Where-Object { $_.Name -like 'trash-me*' } | Measure-Object | ForEach-Object Count")
    print("    ごみ箱の中の trash-me：", rb.strip())
    if rb.strip() not in ("", "0"):
        fails.append("取り置きに写したのにごみ箱にも入っている")
    held = list((HD / "holding").glob("*/trash-me.txt"))
    print("    取り置きの中：", held)
    if not held or held[0].read_text().strip() != "test":
        fails.append("取り置きに写っていない")
    d = tmp / "trash-dir"
    (d / "sub").mkdir(parents=True, exist_ok=True)
    (d / "sub" / "x.txt").write_text("xyz")
    check("フォルダも取り置きへ", "pc_trash", {"paths": [str(d)]}, False, "取り置き 30 日")
    if not list((HD / "holding").glob("*/trash-dir/sub/x.txt")):
        fails.append("フォルダが取り置きに写っていない")
    g = tmp / "recycle-only.txt"
    g.write_text("r")
    check("recycle: true はごみ箱だけ", "pc_trash", {"paths": [str(g)], "recycle": True}, False, "取り置きなし")
    check("状態に取り置きが出る", "pc_status", {}, False, "取り置き：2 件")
    check("取り置きの場所は触れない", "pc_powershell", {"script": f"Get-ChildItem '{HD / 'holding'}'"}, True, "置き場所")
    check("Windows 本体はごみ箱にも送れない", "pc_trash", {"paths": [r"C:\Windows\notepad.exe"]}, True, "許可")

if WIN:
    print("=== 3b. 作業のまとまり（hands）")
    check("作業を始める", "hands", {"action": "start", "title": "試しの作業"}, False, "作業を始めた")
    check("Git で記録するファイルを変える", "pc_powershell", {"script": f"Set-Content -Path '{GT / 'keep.txt'}' -Value 'after' -NoNewline; Set-Content -Path '{GT / 'new.txt'}' -Value 'new' -NoNewline; 'done'", "real": True}, False)
    h = tmp / "hands-trash.txt"
    h.write_text("held")
    check("作業の中で消す", "pc_trash", {"paths": [str(h)]}, False, "取り置き 30 日")
    err, text, _, _ = call("hands", {"action": "done"})
    print("    " + text.replace("\n", "\n    ")[:1200])
    if err or "変えた：pc-mcp-hands-test/keep.txt" not in text or "作った：pc-mcp-hands-test/new.txt" not in text or str(h) not in text:
        fails.append("DONE の報告に変えたもの・消したものが出ていない")
    tid = ""
    for l in text.splitlines():
        if "task=" in l:
            tid = l.split("task=")[1].split("（")[0].strip()
    check("TIMELINE を読む", "hands", {"action": "log", "task": tid}, False, "pc_trash")
    check("まとめて戻す", "hands", {"action": "undo", "task": tid}, False, "戻した")
    print("    keep.txt：", (GT / "keep.txt").read_text().strip(), "／new.txt あり：", (GT / "new.txt").exists(), "／hands-trash.txt あり：", h.exists())
    if (GT / "keep.txt").read_text().strip() != "before":
        fails.append("変えたファイルが戻っていない")
    if (GT / "new.txt").exists():
        fails.append("作ったファイルが取り置きへ行っていない")
    if not h.exists():
        fails.append("消したものが取り置きから戻っていない")
    check("取り置きは消えていない（戻すのは写す）", "pc_status", {}, False, "取り置き：")

if WIN:
    print("=== 3c. 作業の道（A〜E）")
    check("C：変える台本には確かめが付く", "pc_powershell", {"script": f"Set-Content -Path '{GT / 'c-ok.txt'}' -Value 'ok' -NoNewline; 'done'", "real": True}, False, "C（変更）の確かめ")
    check("C：エラーが出たら決まったフォルダの変更を自動で戻す", "pc_powershell", {"script": f"Set-Content -Path '{GT / 'c-ng.txt'}' -Value 'ng' -NoNewline; Write-Error 'わざとのエラー'", "real": True}, True, "自動で戻した")
    if (GT / "c-ng.txt").exists():
        fails.append("C：エラーのあと、作ったファイルが残っている")
    if not (GT / "c-ok.txt").exists():
        fails.append("C：エラーの無い変更まで戻した")
    check("C：黙らせたエラーは数えない", "pc_powershell", {"script": f"Set-Content -Path '{GT / 'c-quiet.txt'}' -Value 'q' -NoNewline; Get-Item 'C:\\no-such-file' -ErrorAction SilentlyContinue; 'done'", "real": True}, False, "黙らせたもの")
    if not (GT / "c-quiet.txt").exists():
        fails.append("C：黙らせたエラーで戻した")
    err, text, _, _ = call("pc_powershell", {"script": "1+2"})
    if err or "C（変更）" in text:
        fails.append("A：読むだけの台本に C の確かめが付いた")
    check("E：「送信」のボタンは押さない", "pc_ui_do", {"action": "click", "target": "送信", "window": "notepad"}, True, "E（")
    check("E：「Sign in」のボタンは押さない", "pc_ui_do", {"action": "click", "target": "Sign in"}, True, "E（")
    tl = HU / "timeline"
    lanes = set()
    for f in tl.glob("*.jsonl"):
        for l in f.read_text(encoding="utf-8").splitlines():
            try:
                lanes.add(json.loads(l).get("lane", ""))
            except Exception:
                pass
    print("    TIMELINE の道：", sorted(lanes))
    for want in ("A", "C", "D", "E"):
        if want not in lanes:
            fails.append("TIMELINE に道 " + want + " が無い")

print("=== 4. 許可（ALLOW）")
(AD / "allow.txt").write_text("allow\r\n")
check("状態（許可あり）", "pc_status", {}, False, "あり")
if WIN:
    check("許可があれば資格情報の一覧を見られる", "pc_powershell", {"script": "cmdkey /list | Select-Object -First 3", "real": True}, False)
check("許可があれば完全に消せる", "pc_powershell", {"script": f"Set-Content '{tmp / 'b.txt'}' x; Remove-Item '{tmp / 'b.txt'}'; Test-Path '{tmp / 'b.txt'}'", "real": True}, False, "False")
check("許可があってもスイッチの置き場所は触れない", "pc_powershell", {"script": f"Set-Content '{SD / 'mode.txt'}' operate"}, True, "置き場所")
set_mode("operate")
check("スイッチを切り替えると許可は取り消し", "pc_status", {}, False, "許可（ALLOW）：なし")

if WIN:
    print("=== 5. 画面と部品（GitHub の Windows に画面があるかは分からないので、結果を見るだけ）")
    look("窓の一覧", "pc_windows", {})
    look("画面", "pc_screenshot", {})
    look("メモ帳を開く", "pc_open", {"target": "notepad.exe"})
    time.sleep(2)
    look("窓の一覧", "pc_windows", {})
    look("メモ帳の部品", "pc_ui", {"window": "notepad", "max": 30})
    look("メモ帳に貼る", "pc_keys", {"window": "notepad", "text": "こんにちは PC の道具", "paste": True})
    look("メモ帳に打つ", "pc_keys", {"window": "notepad", "text": " abc 123", "enter": True})
    look("打ったあとの窓の一覧", "pc_windows", {"all": True})
    look("打ったあとの前の窓の部品（window なし）", "pc_ui", {"max": 8})
    look("Notepad（大文字）で探す", "pc_ui", {"window": "Notepad", "max": 5})
    look("診断：窓を素で数える", "pc_powershell", {"script": "Add-Type -AssemblyName UIAutomationClient; $r=[System.Windows.Automation.AutomationElement]::RootElement; $c=$r.FindAll([System.Windows.Automation.TreeScope]::Children,[System.Windows.Automation.Condition]::TrueCondition); foreach($w in $c){ '{0} | {1} | {2}' -f $w.Current.Name,$w.Current.ProcessId,$w.Current.ClassName }; Get-Process notepad -ErrorAction SilentlyContinue | ForEach-Object { 'proc {0} {1} [{2}]' -f $_.Id,$_.MainWindowHandle,$_.MainWindowTitle }", "real": True})
    look("文字を入れる（set_value）", "pc_ui_do", {"window": "Notepad", "target": "Text Editor", "action": "set_value", "value": "set_value で入れた"})
    look("メモ帳の中身（打った文字）", "pc_ui", {"window": "notepad", "max": 3})
    err, text = look("メモ帳の中身（set_value のあと）", "pc_ui", {"window": "notepad", "filter": "入れた"})
    if "set_value で入れた" not in text:
        fails.append("メモ帳に入れた文字が読めない")
    look("メモ帳の窓", "pc_screenshot", {"window": "notepad"})
    look("メモ帳を閉じる（保存しない）", "pc_powershell", {"script": "Get-Process notepad -ErrorAction SilentlyContinue | Stop-Process -Force; 'closed'", "real": True})


def find_chrome():
    for c in [os.environ.get("PC_TEST_CHROME", ""), r"C:\Program Files\Google\Chrome\Application\chrome.exe",
              r"C:\Program Files (x86)\Google\Chrome\Application\chrome.exe", shutil.which("google-chrome") or "", shutil.which("chromium") or ""]:
        if c and os.path.exists(c):
            return c
    return None


chrome = find_chrome()
if not chrome:
    print("=== 5b. ブラウザ：Chrome が無いので飛ばす")
else:
    print("=== 5b. ブラウザ（試験用の Chrome・headless）")
    check("ブラウザのスイッチがオフなら使えない", "browser_tabs", {}, True, "ブラウザのスイッチがオフ")
    set_bmode("operate")
    check("Chrome のリモートデバッグがオフ", "browser_tabs", {}, True, "リモートデバッグがオフ")
    site = HERE / "site"
    srv = http.server.ThreadingHTTPServer(("127.0.0.1", 0), functools.partial(http.server.SimpleHTTPRequestHandler, directory=str(site)))
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    base = f"http://127.0.0.1:{srv.server_address[1]}"
    cp = subprocess.Popen([chrome, "--headless=new", "--no-sandbox", "--disable-gpu", "--no-first-run", "--remote-debugging-port=0",
                           f"--user-data-dir={CHROME_PROF}", "about:blank"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    for _ in range(300):  # 60 秒まで待つ（CI の Windows では 20 秒で立ち上がらないことがあった。2026-11-28）
        if (CHROME_PROF / "DevToolsActivePort").exists():
            break
        time.sleep(0.2)
    print("    試験用の Chrome：", "立ち上がった" if (CHROME_PROF / "DevToolsActivePort").exists() else "60 秒たっても DevToolsActivePort が無い")
    time.sleep(1)
    check("タブを開く", "browser_tabs", {"action": "new", "url": base + "/index.html"}, False, "試験のページ")
    check("部品の一覧", "browser_read", {}, False, "「決める」")
    check("パスワード欄に印", "browser_read", {"filter": "password"}, False, "Claude は打たない")
    check("まとめて動かす（打つ・選ぶ・押す・待つ）", "browser_do", {"steps": [
        {"type": {"ref": "text=名前", "text": "かおる"}}, {"select": {"ref": "css=#fruit", "value": "みかん"}},
        {"click": "text=決める"}, {"wait_for": "押した"}], "read": "text"}, False, "押した:かおる:b")
    check("パスワード欄には打たない", "browser_do", {"steps": [{"type": {"ref": "css=#pw", "text": "x"}}]}, True, "打ちません")
    check("パスワード欄では貼らない", "browser_do", {"steps": [{"click": "css=#pw"}, {"key": "Ctrl+V"}], "read": "none"}, True, "押しません")
    check("canvas を本物のクリックで押す・遠くのボタン", "browser_do", {"steps": [
        {"click": "css=#cv"}, {"click": "遠くのボタン"},
        {"js": "document.getElementById('cvmsg').textContent + ' / ' + document.getElementById('far').textContent"}], "read": "none"}, False, "trusted=true")
    check("ページを移る・あとで出る文字を待つ・戻る", "browser_do", {"steps": [
        {"click": "text=次のページへ"}, {"click": "あとで出す"}, {"wait_for": "あとから出た"}, {"back": True}, {"js": "location.pathname"}], "read": "none"}, False, "index.html")
    # 分かれ道・繰り返し・確かめ（台本で使う。道具は増やさない）
    check("確かめ：あるはずの部品がある", "browser_do", {"steps": [{"check": {"text": "決める"}}], "read": "none"}, False, "→ OK")
    check("確かめ：ないはずのものがあれば止まる", "browser_do", {"steps": [{"check": {"text": "決める", "absent": True}}, {"wait": 1}], "read": "none"}, True, "はずなのに違った")
    check("分かれ道：無ければ else の手", "browser_do", {"steps": [{"if": {"text": "どこにも無い文字", "then": [{"js": "'then'"}], "else": [{"js": "'else-ran'"}]}}], "read": "none"}, False, "else-ran")
    check("繰り返し：出るまで押す（2 回目で出る）", "browser_do", {"steps": [
        {"js": "window.__n=0; document.title"},
        {"repeat": {"times": 5, "until": {"css": "#rep2"}, "steps": [{"js": "window.__n++; if(window.__n>=2){const d=document.createElement('div'); d.id='rep2'; d.textContent='rep'; document.body.appendChild(d)} window.__n"}]}},
        {"js": "window.__n"}], "read": "none"}, False, "repeat 2 回")
    check("繰り返し：ならなければ止まる", "browser_do", {"steps": [{"repeat": {"times": 2, "until": "絶対に出ない文字", "steps": [{"wait": 10}]}}], "read": "none"}, True, "なりませんでした")
    # 台本（RECIPE）：保存して 1 回の呼び出しで流す・言葉の入れ替え・失敗で止まる
    check("台本を保存する", "hands", {"action": "save", "name": "試験の台本", "about": "試験のページを確かめて言葉を返す", "params": {"w": "返す言葉"},
        "steps": [{"tool": "browser_do", "args": {"steps": [{"check": {"text": "決める"}}], "read": "none"}, "note": "ページを確かめる"},
                  {"tool": "browser_js", "args": {"code": "'{{w}}' + '!'"}, "note": "言葉を返す"}]}, False, "新しく保存した")
    check("台本を流す（言葉を入れ替える）", "hands", {"action": "run", "name": "試験の台本", "values": {"w": "やった"}}, False, "やった!")
    check("台本の一覧", "hands", {"action": "recipes"}, False, "1 回流して 1 回うまくいった")
    check("台本では消せない", "hands", {"action": "save", "name": "消す台本", "about": "x", "steps": [{"tool": "pc_trash", "args": {"paths": [str(tmp / "x")]}}]}, True, "台本では使えません")
    check("台本は失敗した手で止まる", "hands", {"action": "save", "name": "止まる台本", "about": "x", "steps": [
        {"tool": "browser_do", "args": {"steps": [{"check": {"text": "どこにも無い文字"}}], "read": "none"}}, {"tool": "browser_js", "args": {"code": "'動いてはいけない'"}}]}, False, "保存")
    check("台本は失敗した手で止まる（流す）", "hands", {"action": "run", "name": "止まる台本"}, True, "1 手目で止まりました")
    check("JavaScript（await・最後の値）", "browser_js", {"code": "let a = 20; await new Promise(r => setTimeout(r, 50)); a + 1"}, False, "21")
    check("クッキーの JavaScript は許可が要る", "browser_js", {"code": "document.cookie"}, True, "許可")
    check("Chrome の設定には行かない", "browser_do", {"steps": [{"goto": "chrome://settings/passwords"}]}, True, "行きません")
    check("ログインのページは許可が要る", "browser_do", {"steps": [{"goto": base + "/login.html"}]}, True, "許可")
    check("撮る（部品だけ）", "browser_screenshot", {"target": "css=#cv"}, False, "click_xy")
    (AD / "allow.txt").write_text("allow\r\n")
    check("許可があればログインのページも動かせる", "browser_do", {"steps": [
        {"goto": base + "/login.html"}, {"type": {"ref": "css=#u", "text": "user"}}, {"click": "次へ"}, {"wait_for": "押された"}], "read": "none"}, False, "出た")
    check("E：許可があっても「送る」は押さない", "browser_do", {"steps": [{"click": "送る"}], "read": "none"}, True, "E（")
    check("E：分かれ道・繰り返しの中でも「送る」は押さない", "browser_do", {"steps": [{"if": {"text": "ログイン", "then": [{"repeat": {"times": 1, "steps": [{"click": "送る"}]}}], "else": [{"click": "送る"}]}}], "read": "none"}, True, "E（")
    # Claude 用の Chrome（道具が自分で起動する・ログインのページは許可があっても開かない）
    os.environ["PC_CHROME_EXE"] = chrome
    check("Claude 用の Chrome を起動してタブを開く", "browser_tabs", {"action": "new", "url": base + "/index.html", "profile": "claude"}, False, "試験のページ")
    if not (CLAUDE_CHROME / "DevToolsActivePort").exists():
        fails.append("Claude 用の Chrome が別の User Data で起動していない")
    check("Claude 用の Chrome で読む", "browser_read", {"profile": "claude"}, False, "「決める」")
    check("Claude 用の Chrome ではログインのページに行かない（許可があっても）", "browser_do", {"profile": "claude", "steps": [{"goto": base + "/login.html"}], "read": "none"}, True, "Claude 用の Chrome では")
    check("Claude 用の Chrome のタブは閉じてよい", "browser_tabs", {"action": "close", "tab": "試験のページ", "profile": "claude"}, False, "閉じました")
    check("2 つの Chrome につながっている", "pc_status", {}, False, "Claude 用の Chrome・カオルの Chrome")
    set_mode("operate")  # 許可を取り消す
    check("タブを並び順の番号では選ばない", "browser_read", {"tab": "1"}, True, "番号ではなく")
    check("カオルのタブは閉じない", "browser_tabs", {"action": "close", "tab": "about:blank"}, True, "閉じません")
    check("Claude が開いたタブは閉じる", "browser_tabs", {"action": "close", "tab": "ログイン"}, False, "閉じました")
    set_bmode("view")
    check("ブラウザが見るだけでは動かせない", "browser_do", {"steps": [{"wait": 1}]}, True, "見るだけ")
    check("ブラウザが見るだけでも一覧は見られる", "browser_tabs", {}, False, "タブ")
    set_bmode("operate")
    set_mode("off")
    time.sleep(3)
    check("PC がオフでもブラウザは使える", "browser_tabs", {}, False, "タブ")
    check("PC がオフなら PC の道具は使えない", "pc_disk", {}, True, "PC のスイッチがオフ")
    set_mode("operate")
    set_bmode("off")
    time.sleep(3)
    check("ブラウザをオフにすると Chrome と切れる", "pc_status", {}, False, "Chrome：つながっていない")
    check("ブラウザがオフなら使えない", "browser_read", {}, True, "ブラウザのスイッチがオフ")
    cp.kill()
    srv.shutdown()

if WIN:
    print("=== 5b. STOP（スイッチをオフ）で、動いている台本も止まる")
    set_mode("operate")
    box = {}
    def long_run():
        t0 = time.time()
        box["r"] = call("pc_powershell", {"script": "Start-Sleep -Seconds 40; 'done'", "real": True, "seconds": 120})
        box["dt"] = time.time() - t0
    th = threading.Thread(target=long_run)
    th.start()
    time.sleep(4)
    set_mode("off")
    th.join(60)
    err, text, _, _ = box.get("r", (False, "（返事なし）", False, 0))
    print(f"    {box.get('dt', -1):.1f} 秒・エラー={err}：" + text[:300].replace("\n", "\n    "))
    if not err or "STOP" not in text or box.get("dt", 99) > 25:
        fails.append("STOP で動いている台本が止まらない")
    if "C（変更）の確かめ" not in text:
        fails.append("STOP で止めたあとの確かめ（控えに戻す）が切れた")
    for _ in range(20):  # 常駐役が終わるのを待ってから起こし直す（ブラウザがオンなら終わらない）
        if not (SD / "token.txt").exists():
            break
        time.sleep(0.5)
    start_agent()
    # 控えが取れなければ本物では変えない（記録するフォルダを空にして試す）
    set_mode("operate")
    gf = HU / "git-folders.txt"
    keep_gf = gf.read_text(encoding="utf-8")
    gf.write_text("# none\n", encoding="utf-8")
    nf = tmp / "no-snapshot.txt"
    check("控えが取れなければ本物では変えない", "pc_powershell", {"script": f"Set-Content -Path '{nf}' -Value 'x'", "real": True}, True, "控え")
    if nf.exists():
        fails.append("控えが取れないのに本物を変えた")
    gf.write_text(keep_gf, encoding="utf-8")

print("=== 6. オフにすると常駐役が終わる")
set_mode("off")
time.sleep(4)
check("状態", "pc_status", {}, False, "動いていない")

print()
print("記録（audit.log）の最後：")
try:
    print((SD / "audit.log").read_text(encoding="utf-8").splitlines()[-5:])
except Exception as e:
    print(e)
if fails:
    print("NG：", fails)
    sys.exit(1)
print("全部 OK")
