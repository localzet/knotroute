//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	wmDestroy       = 0x0002
	wmCommand       = 0x0111
	wmRButtonUp     = 0x0205
	wmLButtonDblClk = 0x0203
	wmAppTray       = 0x8001
	wmClose         = 0x0010

	nimAdd         = 0x00000000
	nimModify      = 0x00000001
	nimDelete      = 0x00000002
	nimSetVersion  = 0x00000004
	nifMessage     = 0x00000001
	nifIcon        = 0x00000002
	nifTip         = 0x00000004
	nifInfo        = 0x00000010
	notifyVersion4 = 4
	niifInfo       = 0x00000001
	niifWarning    = 0x00000002

	mfString       = 0x00000000
	mfSeparator    = 0x00000800
	mfGray         = 0x00000001
	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100
	tpmNonotify    = 0x0080

	mbOK          = 0x00000000
	mbYesNo       = 0x00000004
	mbIconWarning = 0x00000030
	mbIconError   = 0x00000010
	idYes         = 6

	idiApplication = 32512
	idcArrow       = 32512
	cfUnicodeText  = 13
	gmemMoveable   = 0x0002

	internetOptionRefresh              = 37
	internetOptionSettingsChanged      = 39
	internetOptionProxySettingsChanged = 95

	cmdDashboard   = 1001
	cmdStart       = 1002
	cmdStop        = 1003
	cmdRestart     = 1004
	cmdCopyDomain  = 1005
	cmdIntegration = 1006
	cmdStartup     = 1007
	cmdFolder      = 1008
	cmdExit        = 1099
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	wininet  = syscall.NewLazyDLL("wininet.dll")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procLoadIconW           = user32.NewProc("LoadIconW")
	procCreateIcon          = user32.NewProc("CreateIcon")
	procDestroyIcon         = user32.NewProc("DestroyIcon")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procMessageBoxW         = user32.NewProc("MessageBoxW")
	procOpenClipboard       = user32.NewProc("OpenClipboard")
	procEmptyClipboard      = user32.NewProc("EmptyClipboard")
	procSetClipboardData    = user32.NewProc("SetClipboardData")
	procCloseClipboard      = user32.NewProc("CloseClipboard")

	procShellNotifyIconW         = shell32.NewProc("Shell_NotifyIconW")
	procShellExecuteW            = shell32.NewProc("ShellExecuteW")
	procGetModuleHandleW         = kernel32.NewProc("GetModuleHandleW")
	procCreateMutexW             = kernel32.NewProc("CreateMutexW")
	procGlobalAlloc              = kernel32.NewProc("GlobalAlloc")
	procGlobalLock               = kernel32.NewProc("GlobalLock")
	procGlobalUnlock             = kernel32.NewProc("GlobalUnlock")
	procRtlMoveMemory            = kernel32.NewProc("RtlMoveMemory")
	procGetUserDefaultUILanguage = kernel32.NewProc("GetUserDefaultUILanguage")
	procInternetSetOptionW       = wininet.NewProc("InternetSetOptionW")
)

type point struct{ X, Y int32 }
type message struct {
	HWnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       point
	LPrivate uint32
}
type wndClassEx struct {
	CbSize     uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}
type notifyIconData struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	CallbackMessage  uint32
	Icon             uintptr
	Tip              [128]uint16
	State            uint32
	StateMask        uint32
	Info             [256]uint16
	TimeoutOrVersion uint32
	InfoTitle        [64]uint16
	InfoFlags        uint32
	GUID             [16]byte
	BalloonIcon      uintptr
}

type statusResponse struct {
	Domain string `json:"domain"`
	Peers  []any  `json:"peers"`
	Routes []any  `json:"routes"`
}
type diskConfig struct {
	Dashboard string `json:"dashboard"`
}
type proxyBackup struct {
	Enabled       bool   `json:"enabled"`
	HadAutoConfig bool   `json:"had_auto_config"`
	AutoConfigURL string `json:"auto_config_url,omitempty"`
}

type desktopApp struct {
	mu            sync.Mutex
	hwnd          uintptr
	icon          uintptr
	nid           notifyIconData
	running       bool
	starting      bool
	stopping      bool
	crashRestarts int
	lastCrash     time.Time
	domain        string
	cmd           *exec.Cmd
	dataDir       string
	configPath    string
	daemonPath    string
	dashboardURL  string
	quit          chan struct{}
}

var app *desktopApp

func runDesktop() {
	if !acquireSingleInstance() {
		return
	}
	instance, err := newDesktopApp()
	if err != nil {
		messageBox(0, "KnotRoute", err.Error(), mbOK|mbIconError)
		return
	}
	app = instance
	if err := instance.createWindowAndTray(); err != nil {
		messageBox(0, "KnotRoute", err.Error(), mbOK|mbIconError)
		return
	}
	safeGo(instance.statusLoop)
	safeGo(func() { instance.startNode(false) })
	var msg message
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
	close(instance.quit)
	instance.removeTray()
}

func newDesktopApp() (*desktopApp, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return nil, errors.New(dt("LOCALAPPDATA is not set", "Переменная окружения LOCALAPPDATA не задана"))
	}
	dataDir := filepath.Join(local, "KnotRoute")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	daemon := filepath.Join(filepath.Dir(exe), "knotroute.exe")
	if _, err := os.Stat(daemon); err != nil {
		return nil, fmt.Errorf(dt("knotroute.exe was not found next to the desktop controller: %w", "knotroute.exe не найден рядом с приложением KnotRoute Desktop: %w"), err)
	}
	configPath := filepath.Join(dataDir, "knotroute.json")
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		cmd := exec.Command(daemon, "init", "--config", configPath, "--listen", "0.0.0.0:7447", "--dashboard", "127.0.0.1:8484")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if output, runErr := cmd.CombinedOutput(); runErr != nil {
			return nil, fmt.Errorf(dt("initialize KnotRoute: %v: %s", "не удалось инициализировать KnotRoute: %v: %s"), runErr, bytes.TrimSpace(output))
		}
	}
	a := &desktopApp{dataDir: dataDir, configPath: configPath, daemonPath: daemon, quit: make(chan struct{})}
	a.reloadDashboardURL()
	return a, nil
}

func acquireSingleInstance() bool {
	name, _ := syscall.UTF16PtrFromString("Local\\KnotRouteDesktop")
	_, _, callErr := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if errno, ok := callErr.(syscall.Errno); ok && errno == 183 {
		return false
	}
	return true
}

func (a *desktopApp) reloadDashboardURL() {
	a.dashboardURL = "http://127.0.0.1:8484"
	data, err := os.ReadFile(a.configPath)
	if err != nil {
		return
	}
	var cfg diskConfig
	if json.Unmarshal(data, &cfg) == nil && cfg.Dashboard != "" {
		host, port, splitErr := net.SplitHostPort(cfg.Dashboard)
		if splitErr == nil {
			if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
				host = "127.0.0.1"
			}
			a.dashboardURL = "http://" + net.JoinHostPort(host, port)
		}
	}
}

func (a *desktopApp) createWindowAndTray() error {
	className, _ := syscall.UTF16PtrFromString("KnotRouteDesktopWindow")
	instance, _, _ := procGetModuleHandleW.Call(0)
	icon := createKnotIcon(instance)
	if icon == 0 {
		icon, _, _ = procLoadIconW.Call(0, idiApplication)
	}
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	wc := wndClassEx{CbSize: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: syscall.NewCallback(windowProc), Instance: instance, Icon: icon, Cursor: cursor, ClassName: className, IconSm: icon}
	if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return fmt.Errorf("RegisterClassExW: %v", err)
	}
	title, _ := syscall.UTF16PtrFromString("KnotRoute")
	hwnd, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), 0, 0, 0, 0, 0, 0, 0, instance, 0)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowExW: %v", err)
	}
	a.hwnd, a.icon = hwnd, icon
	a.nid = notifyIconData{CbSize: uint32(unsafe.Sizeof(notifyIconData{})), HWnd: hwnd, UID: 1, UFlags: nifMessage | nifIcon | nifTip, CallbackMessage: wmAppTray, Icon: icon}
	copyUTF16(a.nid.Tip[:], dt("KnotRoute · starting", "KnotRoute · запуск"))
	if r, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&a.nid))); r == 0 {
		return fmt.Errorf("Shell_NotifyIconW: %v", err)
	}
	a.nid.TimeoutOrVersion = notifyVersion4
	procShellNotifyIconW.Call(nimSetVersion, uintptr(unsafe.Pointer(&a.nid)))
	return nil
}

func windowProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	defer func() {
		if recovered := recover(); recovered != nil {
			if app != nil {
				app.balloon(dt("KnotRoute desktop error", "Ошибка KnotRoute Desktop"), fmt.Sprint(recovered), niifWarning)
			}
		}
	}()
	if app == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return r
	}
	switch msg {
	case wmAppTray:
		event := uint32(lParam & 0xffff)
		if event == wmRButtonUp {
			app.showMenu()
		}
		if event == wmLButtonDblClk {
			safeGo(app.openDashboard)
		}
		return 0
	case wmCommand:
		safeGo(func() { app.handleCommand(uint16(wParam & 0xffff)) })
		return 0
	case wmClose:
		procPostMessageW.Call(hwnd, wmDestroy, 0, 0)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	default:
		r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return r
	}
}

func (a *desktopApp) showMenu() {
	a.mu.Lock()
	running, domain := a.running, a.domain
	a.mu.Unlock()
	menu, _, _ := procCreatePopupMenu.Call()
	defer procDestroyMenu.Call(menu)
	appendMenu(menu, mfString, cmdDashboard, dt("Open dashboard", "Открыть панель"))
	appendMenu(menu, mfSeparator, 0, "")
	startFlags, stopFlags := uint32(mfString), uint32(mfString)
	if running {
		startFlags |= mfGray
	} else {
		stopFlags |= mfGray
	}
	appendMenu(menu, startFlags, cmdStart, dt("Start node", "Запустить узел"))
	appendMenu(menu, stopFlags, cmdStop, dt("Stop node", "Остановить узел"))
	appendMenu(menu, stopFlags, cmdRestart, dt("Restart node", "Перезапустить узел"))
	copyFlags := uint32(mfString)
	if domain == "" {
		copyFlags |= mfGray
	}
	appendMenu(menu, copyFlags, cmdCopyDomain, dt("Copy .knot address", "Копировать .knot-адрес"))
	appendMenu(menu, mfSeparator, 0, "")
	integrationText := dt("Enable .knot system integration", "Включить системную интеграцию .knot")
	if a.integrationEnabled() {
		integrationText = dt("Disable .knot system integration", "Отключить системную интеграцию .knot")
	}
	appendMenu(menu, mfString, cmdIntegration, integrationText)
	startupText := dt("Start with Windows", "Запускать вместе с Windows")
	if a.startupEnabled() {
		startupText = dt("Disable start with Windows", "Не запускать вместе с Windows")
	}
	appendMenu(menu, mfString, cmdStartup, startupText)
	appendMenu(menu, mfString, cmdFolder, dt("Open data folder", "Открыть папку данных"))
	appendMenu(menu, mfSeparator, 0, "")
	appendMenu(menu, mfString, cmdExit, dt("Exit tray (node keeps running)", "Закрыть трей (узел продолжит работу)"))
	var p point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	procSetForegroundWindow.Call(a.hwnd)
	selected, _, _ := procTrackPopupMenu.Call(menu, tpmRightButton|tpmReturnCmd|tpmNonotify, uintptr(p.X), uintptr(p.Y), 0, a.hwnd, 0)
	if selected != 0 {
		procPostMessageW.Call(a.hwnd, wmCommand, selected, 0)
	}
}

func appendMenu(menu uintptr, flags uint32, id uint16, text string) {
	ptr, _ := syscall.UTF16PtrFromString(text)
	procAppendMenuW.Call(menu, uintptr(flags), uintptr(id), uintptr(unsafe.Pointer(ptr)))
}

func (a *desktopApp) handleCommand(command uint16) {
	switch command {
	case cmdDashboard:
		a.openDashboard()
	case cmdStart:
		a.startNode(true)
	case cmdStop:
		a.stopNode(true)
	case cmdRestart:
		a.stopNode(false)
		a.startNode(true)
	case cmdCopyDomain:
		a.mu.Lock()
		domain := a.domain
		a.mu.Unlock()
		if domain != "" {
			_ = setClipboard(domain)
			a.balloon(dt("Address copied", "Адрес скопирован"), domain, niifInfo)
		}
	case cmdIntegration:
		if a.integrationEnabled() {
			if err := a.disableIntegration(); err != nil {
				messageBox(a.hwnd, "KnotRoute", err.Error(), mbOK|mbIconError)
			} else {
				a.balloon(dt("System integration disabled", "Системная интеграция отключена"), dt("Previous proxy script settings were restored.", "Предыдущие настройки proxy-скрипта восстановлены."), niifInfo)
			}
		} else if err := a.enableIntegration(); err != nil {
			messageBox(a.hwnd, "KnotRoute", err.Error(), mbOK|mbIconError)
		} else {
			a.balloon(dt(".knot integration enabled", "Интеграция .knot включена"), dt("Windows now sends only .knot web traffic to KnotRoute.", "Windows теперь направляет в KnotRoute только веб-трафик .knot."), niifInfo)
		}
	case cmdStartup:
		if a.startupEnabled() {
			_ = a.setStartup(false)
		} else {
			_ = a.setStartup(true)
		}
	case cmdFolder:
		shellOpen(a.dataDir)
	case cmdExit:
		procPostMessageW.Call(a.hwnd, wmClose, 0, 0)
	}
}

func (a *desktopApp) startNode(notify bool) {
	a.mu.Lock()
	if a.starting {
		a.mu.Unlock()
		return
	}
	a.starting = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.starting = false
		a.mu.Unlock()
	}()
	if a.health() {
		if notify {
			a.balloon("KnotRoute", dt("Node is already running.", "Узел уже запущен."), niifInfo)
		}
		return
	}
	if err := a.validateNodeFiles(); err != nil {
		a.balloon(dt("KnotRoute configuration error", "Ошибка конфигурации KnotRoute"), err.Error(), niifWarning)
		return
	}
	a.reloadDashboardURL()
	logPath := filepath.Join(a.dataDir, "knotroute.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		if notify {
			messageBox(a.hwnd, "KnotRoute", err.Error(), mbOK|mbIconError)
		}
		return
	}
	cmd := exec.Command(a.daemonPath, "run", "--config", a.configPath)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x00000200}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		if notify {
			messageBox(a.hwnd, "KnotRoute", err.Error(), mbOK|mbIconError)
		}
		return
	}
	a.mu.Lock()
	a.cmd = cmd
	a.stopping = false
	a.mu.Unlock()
	safeGo(func() {
		waitErr := cmd.Wait()
		_ = logFile.Close()
		a.mu.Lock()
		intentional := a.stopping
		if a.cmd == cmd {
			a.cmd = nil
		}
		a.running = false
		a.stopping = false
		if !intentional && time.Since(a.lastCrash) > 2*time.Minute {
			a.crashRestarts = 0
		}
		if !intentional {
			a.crashRestarts++
			a.lastCrash = time.Now()
		}
		restarts := a.crashRestarts
		a.mu.Unlock()
		a.updateTooltip()
		if !intentional {
			detail := dt("The node process exited unexpectedly.", "Процесс узла неожиданно завершился.")
			if waitErr != nil {
				detail += " " + waitErr.Error()
			}
			if tail := tailFile(logPath, 1400); tail != "" {
				detail += "\n\n" + tail
			}
			a.balloon(dt("KnotRoute node stopped", "Узел KnotRoute остановлен"), detail, niifWarning)
			if restarts <= 3 {
				select {
				case <-a.quit:
				case <-time.After(time.Duration(restarts) * time.Second):
					a.startNode(false)
				}
			}
		}
	})
	for i := 0; i < 40; i++ {
		if a.health() {
			if notify {
				a.balloon(dt("KnotRoute started", "KnotRoute запущен"), dt("The overlay and local .knot gateways are online.", "Overlay-сеть и локальные .knot-шлюзы доступны."), niifInfo)
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	if notify {
		a.balloon("KnotRoute", dt("The process started, but the dashboard is not reachable. Open the log from the data folder.", "Процесс запущен, но панель недоступна. Проверьте knotroute.log в папке данных."), niifWarning)
	}
}

func (a *desktopApp) validateNodeFiles() error {
	cmd := exec.Command(a.daemonPath, "id", "--config", a.configPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		detail = err.Error()
	}
	return fmt.Errorf(dt("The node configuration or identity is invalid: %s", "Конфигурация или идентичность узла некорректна: %s"), detail)
}

func (a *desktopApp) stopNode(notify bool) {
	a.mu.Lock()
	a.stopping = true
	cmd := a.cmd
	a.mu.Unlock()

	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest(http.MethodPost, a.dashboardURL+"/api/shutdown", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	response, requestErr := client.Do(req)
	if requestErr == nil {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}
	if requestErr != nil && cmd == nil {
		a.mu.Lock()
		a.stopping = false
		a.mu.Unlock()
		if notify {
			a.balloon("KnotRoute", dt("Node is not running.", "Узел не запущен."), niifInfo)
		}
		return
	}

	// A restart must not race the old daemon. The previous implementation slept
	// for 300 ms and could observe the old dashboard as healthy, skip starting a
	// replacement, and then end up with no node once the old process exited.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.mu.Lock()
		current := a.cmd
		a.mu.Unlock()
		if current == nil && !a.health() {
			if notify {
				a.balloon(dt("KnotRoute stopped", "KnotRoute остановлен"), dt("The tray remains available.", "Значок в трее остаётся доступен."), niifInfo)
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	// If the management endpoint was unavailable or graceful shutdown stalled,
	// terminate only the daemon process that this tray instance owns.
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		killDeadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(killDeadline) {
			a.mu.Lock()
			stopped := a.cmd == nil
			a.mu.Unlock()
			if stopped {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	if notify {
		if requestErr != nil && cmd == nil {
			a.balloon("KnotRoute", dt("Node is not running.", "Узел не запущен."), niifInfo)
		} else {
			a.balloon(dt("KnotRoute stopped", "KnotRoute остановлен"), dt("The tray remains available.", "Значок в трее остаётся доступен."), niifInfo)
		}
	}
}

func (a *desktopApp) health() bool {
	client := &http.Client{Timeout: 700 * time.Millisecond}
	response, err := client.Get(a.dashboardURL + "/api/status")
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false
	}
	var status statusResponse
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&status) != nil {
		return false
	}
	a.mu.Lock()
	a.running = true
	a.domain = status.Domain
	a.mu.Unlock()
	return true
}

func (a *desktopApp) statusLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.quit:
			return
		case <-ticker.C:
			a.reloadDashboardURL()
			a.syncIntegrationURL()
			running := a.health()
			a.mu.Lock()
			a.running = running
			if !running {
				a.domain = ""
			}
			a.mu.Unlock()
			a.updateTooltip()
		}
	}
}

func (a *desktopApp) updateTooltip() {
	a.mu.Lock()
	running, domain := a.running, a.domain
	a.mu.Unlock()
	text := dt("KnotRoute · stopped", "KnotRoute · остановлен")
	if running {
		text = dt("KnotRoute · online", "KnotRoute · подключён")
		if domain != "" {
			text += " · " + domain
		}
	}
	nid := a.nid
	nid.UFlags = nifTip
	copyUTF16(nid.Tip[:], text)
	procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

func (a *desktopApp) balloon(title, text string, flag uint32) {
	nid := a.nid
	nid.UFlags = nifInfo
	copyUTF16(nid.InfoTitle[:], title)
	copyUTF16(nid.Info[:], text)
	nid.InfoFlags = flag
	procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}
func (a *desktopApp) removeTray() {
	if a.hwnd != 0 {
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&a.nid)))
	}
	if a.icon != 0 {
		procDestroyIcon.Call(a.icon)
	}
}
func (a *desktopApp) openDashboard() {
	if !a.health() {
		a.startNode(false)
	}
	shellOpen(a.dashboardURL)
}

func (a *desktopApp) integrationEnabled() bool {
	data, err := os.ReadFile(filepath.Join(a.dataDir, "proxy-state.json"))
	if err != nil {
		return false
	}
	var state proxyBackup
	return json.Unmarshal(data, &state) == nil && state.Enabled
}
func (a *desktopApp) enableIntegration() error {
	answer := messageBox(a.hwnd, "KnotRoute", dt("Enabling .knot integration will install the local KnotRoute Root CA into your Windows user Trusted Root store. It is used only to issue certificates for .knot names and its private key stays on this device. Continue?", "Включение интеграции .knot установит локальный корневой сертификат KnotRoute в хранилище доверенных корневых сертификатов текущего пользователя Windows. Он используется только для имён .knot, а приватный ключ остаётся на этом устройстве. Продолжить?"), mbYesNo|mbIconWarning)
	if answer != idYes {
		return errors.New(dt("system integration was cancelled", "системная интеграция отменена"))
	}
	if err := a.runCACommand("install"); err != nil {
		return fmt.Errorf(dt("install KnotRoute Root CA: %w", "не удалось установить корневой сертификат KnotRoute: %w"), err)
	}
	current, exists := regQuery("AutoConfigURL")
	pac := a.dashboardURL + "/proxy.pac"
	if exists && current != "" && !strings.EqualFold(current, pac) {
		answer := messageBox(a.hwnd, "KnotRoute", dt("Windows already has a proxy configuration script:\n\n", "В Windows уже настроен proxy-скрипт:\n\n")+current+dt("\n\nKnotRoute will preserve and restore it when integration is disabled. Continue?", "\n\nKnotRoute сохранит его и восстановит после отключения интеграции. Продолжить?"), mbYesNo|mbIconWarning)
		if answer != idYes {
			return errors.New(dt("system integration was cancelled", "системная интеграция отменена"))
		}
	}
	state := proxyBackup{Enabled: true, HadAutoConfig: exists, AutoConfigURL: current}
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(filepath.Join(a.dataDir, "proxy-state.json"), append(data, '\n'), 0o600); err != nil {
		return err
	}
	if err := regSet("AutoConfigURL", pac); err != nil {
		return err
	}
	refreshInternetSettings()
	return nil
}
func (a *desktopApp) syncIntegrationURL() {
	if !a.integrationEnabled() {
		return
	}
	desired := a.dashboardURL + "/proxy.pac"
	current, ok := regQuery("AutoConfigURL")
	if ok && strings.EqualFold(current, desired) {
		return
	}
	if regSet("AutoConfigURL", desired) == nil {
		refreshInternetSettings()
	}
}
func (a *desktopApp) disableIntegration() error {
	path := filepath.Join(a.dataDir, "proxy-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var state proxyBackup
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	if state.HadAutoConfig {
		if err := regSet("AutoConfigURL", state.AutoConfigURL); err != nil {
			return err
		}
	} else if err := regDelete("AutoConfigURL"); err != nil {
		return err
	}
	state.Enabled = false
	data, _ = json.MarshalIndent(state, "", "  ")
	_ = os.WriteFile(path, append(data, '\n'), 0o600)
	refreshInternetSettings()
	if err := a.runCACommand("uninstall"); err != nil {
		return fmt.Errorf(dt("proxy settings were restored, but removing the KnotRoute Root CA failed: %w", "настройки proxy восстановлены, но удалить корневой сертификат KnotRoute не удалось: %w"), err)
	}
	return nil
}

func (a *desktopApp) runCACommand(action string) error {
	cmd := exec.Command(a.daemonPath, "ca", action, "--config", a.configPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, bytes.TrimSpace(out))
	}
	return nil
}

func (a *desktopApp) startupEnabled() bool {
	value, ok := regQueryAt(`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "KnotRoute")
	return ok && value != ""
}
func (a *desktopApp) setStartup(enabled bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if enabled {
		return regSetAt(`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "KnotRoute", `"`+exe+`"`)
	}
	return regDeleteAt(`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "KnotRoute")
}

const internetSettingsKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`

func regQuery(name string) (string, bool) { return regQueryAt(internetSettingsKey, name) }
func regQueryAt(key, name string) (string, bool) {
	cmd := exec.Command("reg.exe", "query", key, "/v", name)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 3 && strings.EqualFold(fields[0], name) {
			return strings.Join(fields[2:], " "), true
		}
	}
	return "", false
}
func regSet(name, value string) error { return regSetAt(internetSettingsKey, name, value) }
func regSetAt(key, name, value string) error {
	cmd := exec.Command("reg.exe", "add", key, "/v", name, "/t", "REG_SZ", "/d", value, "/f")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(dt("registry update: %v: %s", "не удалось обновить реестр: %v: %s"), err, bytes.TrimSpace(output))
	}
	return nil
}
func regDelete(name string) error { return regDeleteAt(internetSettingsKey, name) }
func regDeleteAt(key, name string) error {
	if _, ok := regQueryAt(key, name); !ok {
		return nil
	}
	cmd := exec.Command("reg.exe", "delete", key, "/v", name, "/f")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(dt("registry delete: %v: %s", "не удалось удалить значение реестра: %v: %s"), err, bytes.TrimSpace(output))
	}
	return nil
}
func refreshInternetSettings() {
	procInternetSetOptionW.Call(0, internetOptionSettingsChanged, 0, 0)
	procInternetSetOptionW.Call(0, internetOptionProxySettingsChanged, 0, 0)
	procInternetSetOptionW.Call(0, internetOptionRefresh, 0, 0)
}

func setClipboard(text string) error {
	if r, _, err := procOpenClipboard.Call(0); r == 0 {
		return err
	}
	defer procCloseClipboard.Call()
	procEmptyClipboard.Call()
	utf := syscall.StringToUTF16(text)
	size := uintptr(len(utf) * 2)
	h, _, err := procGlobalAlloc.Call(gmemMoveable, size)
	if h == 0 {
		return err
	}
	ptr, _, err := procGlobalLock.Call(h)
	if ptr == 0 {
		return err
	}
	if len(utf) > 0 {
		procRtlMoveMemory.Call(ptr, uintptr(unsafe.Pointer(&utf[0])), size)
	}
	procGlobalUnlock.Call(h)
	if r, _, err := procSetClipboardData.Call(cfUnicodeText, h); r == 0 {
		return err
	}
	return nil
}
func shellOpen(target string) {
	verb, _ := syscall.UTF16PtrFromString("open")
	path, _ := syscall.UTF16PtrFromString(target)
	procShellExecuteW.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(path)), 0, 0, 1)
}
func messageBox(hwnd uintptr, title, text string, flags uint32) int {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	r, _, _ := procMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), uintptr(flags))
	return int(r)
}
func safeGo(fn func()) {
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil && app != nil {
				app.balloon(dt("KnotRoute desktop error", "Ошибка KnotRoute Desktop"), fmt.Sprint(recovered), niifWarning)
			}
		}()
		fn()
	}()
}

func isRussianDesktop() bool {
	if value := strings.ToLower(strings.TrimSpace(os.Getenv("KNOTROUTE_LANG"))); value != "" {
		return value == "ru" || strings.HasPrefix(value, "ru-") || strings.HasPrefix(value, "ru_")
	}
	langID, _, _ := procGetUserDefaultUILanguage.Call()
	return uint16(langID)&0x03ff == 0x19
}

func dt(en, ru string) string {
	if isRussianDesktop() {
		return ru
	}
	return en
}

func tailFile(path string, limit int64) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return ""
	}
	start := info.Size() - limit
	if start < 0 {
		start = 0
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return ""
	}
	raw, _ := io.ReadAll(io.LimitReader(file, limit))
	text := strings.TrimSpace(string(raw))
	if start > 0 {
		if idx := strings.IndexByte(text, '\n'); idx >= 0 {
			text = text[idx+1:]
		}
	}
	return text
}

func copyUTF16(dst []uint16, text string) {
	for i := range dst {
		dst[i] = 0
	}
	src := syscall.StringToUTF16(text)
	if len(src) > len(dst) {
		src = src[:len(dst)]
	}
	copy(dst, src)
}

func createKnotIcon(instance uintptr) uintptr {
	const size = 32
	andMask := make([]byte, size*size/8)
	xorMask := make([]byte, size*size*4)
	inside := func(x, y int) bool {
		if x < 4 || x > 27 || y < 4 || y > 27 {
			return false
		}
		if x >= 7 && x <= 24 || y >= 7 && y <= 24 {
			return true
		}
		dx, dy := 0, 0
		if x < 7 {
			dx = 7 - x
		} else if x > 24 {
			dx = x - 24
		}
		if y < 7 {
			dy = 7 - y
		} else if y > 24 {
			dy = y - 24
		}
		return dx*dx+dy*dy <= 9
	}
	onLine := func(x, y, x1, y1, x2, y2, thickness int) bool {
		dx, dy := x2-x1, y2-y1
		px, py := x-x1, y-y1
		den := dx*dx + dy*dy
		if den == 0 {
			return false
		}
		t := float64(px*dx+py*dy) / float64(den)
		if t < 0 || t > 1 {
			return false
		}
		cx, cy := float64(x1)+t*float64(dx), float64(y1)+t*float64(dy)
		distX, distY := float64(x)-cx, float64(y)-cy
		return distX*distX+distY*distY <= float64(thickness*thickness)
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			opaque := inside(x, y)
			if !opaque {
				andMask[y*(size/8)+x/8] |= 1 << (7 - uint(x%8))
			}
			off := ((size-1-y)*size + x) * 4
			if opaque {
				xorMask[off+0], xorMask[off+1], xorMask[off+2], xorMask[off+3] = 29, 50, 55, 255
				k := (x >= 9 && x <= 12 && y >= 7 && y <= 25) || onLine(x, y, 12, 16, 23, 7, 2) || onLine(x, y, 12, 16, 23, 25, 2)
				if k {
					xorMask[off+0], xorMask[off+1], xorMask[off+2], xorMask[off+3] = 193, 225, 78, 255
				}
			}
		}
	}
	icon, _, _ := procCreateIcon.Call(instance, size, size, 1, 32, uintptr(unsafe.Pointer(&andMask[0])), uintptr(unsafe.Pointer(&xorMask[0])))
	return icon
}
