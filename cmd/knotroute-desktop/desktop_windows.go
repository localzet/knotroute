//go:build windows

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/localzet/knotroute/internal/clientruntime"
	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/social"
)

const (
	wmDestroy         = 0x0002
	wmSize            = 0x0005
	wmClose           = 0x0010
	wmCommand         = 0x0111
	wmTimer           = 0x0113
	wmCtlColorEdit    = 0x0133
	wmCtlColorListBox = 0x0134
	wmCtlColorBtn     = 0x0135
	wmCtlColorStatic  = 0x0138
	wmRButtonUp       = 0x0205
	wmLButtonDblClk   = 0x0203
	wmContextMenu     = 0x007B
	wmAppTray         = 0x8001
	wmNull            = 0x0000

	wsOverlappedWindow = 0x00CF0000
	wsVisible          = 0x10000000
	wsChild            = 0x40000000
	wsTabStop          = 0x00010000
	wsVScroll          = 0x00200000
	wsBorder           = 0x00800000
	bsPushButton       = 0x00000000
	bsAutoCheckbox     = 0x00000003
	esAutoHScroll      = 0x0080
	esAutoVScroll      = 0x0040
	esMultiline        = 0x0004
	esReadOnly         = 0x0800
	esWantReturn       = 0x1000
	lbsNotify          = 0x0001
	cbsDropDownList    = 0x0003

	swHide    = 0
	swShow    = 5
	swRestore = 9

	nimAdd         = 0x00000000
	nimModify      = 0x00000001
	nimDelete      = 0x00000002
	nimSetFocus    = 0x00000003
	nimSetVersion  = 0x00000004
	nifMessage     = 0x00000001
	nifIcon        = 0x00000002
	nifTip         = 0x00000004
	nifInfo        = 0x00000010
	nifShowTip     = 0x00000080
	notifyVersion4 = 4
	niifInfo       = 0x00000001
	niifWarning    = 0x00000002

	mfString       = 0x00000000
	mfSeparator    = 0x00000800
	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100
	tmpNoNotify    = 0x0080

	mbOK          = 0x00000000
	mbYesNo       = 0x00000004
	mbIconWarning = 0x00000030
	mbIconError   = 0x00000010
	idYes         = 6

	idiApplication = 32512
	idcArrow       = 32512
	cfUnicodeText  = 13
	gmemMoveable   = 0x0002

	fontNormal = 1
	fontTitle  = 2
	fontMono   = 3

	idNavHome      = 100
	idNavChats     = 101
	idNavCatalog   = 102
	idNavNetwork   = 103
	idNavDeveloper = 104
	idNavSettings  = 105
	idNavFeed      = 106

	idStartStop        = 1001
	idIntegration      = 1002
	idAddContact       = 1010
	idSendMessage      = 1011
	idRefreshChat      = 1012
	idOpenCatalog      = 1020
	idRefreshCatalog   = 1021
	idSaveNetwork      = 1030
	idToggleTransport  = 1031
	idSaveProfile      = 1050
	idSaveCAProfile    = 1051
	idRotateCA         = 1052
	idInstallCA        = 1053
	idOpenData         = 1054
	idDiagnostics      = 1055
	idStartup          = 1056
	idToggleCAAdvanced = 1057

	idContactNode    = 2010
	idContactAlias   = 2011
	idContactsList   = 2012
	idMessagesList   = 2013
	idMessageBody    = 2014
	idCatalogList    = 2020
	idFeedList       = 2021
	idPostBody       = 2022
	idPublishPost    = 1022
	idRefreshFeed    = 1023
	idNetwork        = 2030
	idBeacons        = 2031
	idHops           = 2032
	idTransportMode  = 2033
	idTransportSOCKS = 2034
	idProfileName    = 2050
	idProfileBio     = 2051
	idCACN           = 2060
	idCAO            = 2061
	idCAOU           = 2062
	idCAC            = 2063
	idCAST           = 2064
	idCAL            = 2065
	idCAValidity     = 2066
	idCAStreet       = 2067
	idCAPostal       = 2068

	cmdTrayOpen        = 4001
	cmdTrayToggle      = 4002
	cmdTrayIntegration = 4003
	cmdTrayDiagnostics = 4004
	cmdTrayExit        = 4099
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	wininet  = syscall.NewLazyDLL("wininet.dll")
	uxtheme  = syscall.NewLazyDLL("uxtheme.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")

	procRegisterClassExW       = user32.NewProc("RegisterClassExW")
	procRegisterWindowMessageW = user32.NewProc("RegisterWindowMessageW")
	procCreateWindowExW        = user32.NewProc("CreateWindowExW")
	procDefWindowProcW         = user32.NewProc("DefWindowProcW")
	procGetMessageW            = user32.NewProc("GetMessageW")
	procTranslateMessage       = user32.NewProc("TranslateMessage")
	procDispatchMessageW       = user32.NewProc("DispatchMessageW")
	procPostQuitMessage        = user32.NewProc("PostQuitMessage")
	procPostMessageW           = user32.NewProc("PostMessageW")
	procShowWindow             = user32.NewProc("ShowWindow")
	procUpdateWindow           = user32.NewProc("UpdateWindow")
	procSetWindowTextW         = user32.NewProc("SetWindowTextW")
	procGetWindowTextW         = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW   = user32.NewProc("GetWindowTextLengthW")
	procDestroyWindow          = user32.NewProc("DestroyWindow")
	procMoveWindow             = user32.NewProc("MoveWindow")
	procSetTimer               = user32.NewProc("SetTimer")
	procKillTimer              = user32.NewProc("KillTimer")
	procSendMessageW           = user32.NewProc("SendMessageW")
	procLoadIconW              = user32.NewProc("LoadIconW")
	procLoadCursorW            = user32.NewProc("LoadCursorW")
	procCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	procAppendMenuW            = user32.NewProc("AppendMenuW")
	procTrackPopupMenu         = user32.NewProc("TrackPopupMenu")
	procDestroyMenu            = user32.NewProc("DestroyMenu")
	procGetCursorPos           = user32.NewProc("GetCursorPos")
	procSetForegroundWindow    = user32.NewProc("SetForegroundWindow")
	procMessageBoxW            = user32.NewProc("MessageBoxW")
	procOpenClipboard          = user32.NewProc("OpenClipboard")
	procEmptyClipboard         = user32.NewProc("EmptyClipboard")
	procSetClipboardData       = user32.NewProc("SetClipboardData")
	procCloseClipboard         = user32.NewProc("CloseClipboard")
	procSetTextColor           = gdi32.NewProc("SetTextColor")
	procSetBkColor             = gdi32.NewProc("SetBkColor")
	procCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW            = gdi32.NewProc("CreateFontW")
	procDeleteObject           = gdi32.NewProc("DeleteObject")

	procShellNotifyIconW      = shell32.NewProc("Shell_NotifyIconW")
	procShellExecuteW         = shell32.NewProc("ShellExecuteW")
	procGetModuleHandleW      = kernel32.NewProc("GetModuleHandleW")
	procCreateMutexW          = kernel32.NewProc("CreateMutexW")
	procGlobalAlloc           = kernel32.NewProc("GlobalAlloc")
	procGlobalLock            = kernel32.NewProc("GlobalLock")
	procGlobalUnlock          = kernel32.NewProc("GlobalUnlock")
	procRtlMoveMemory         = kernel32.NewProc("RtlMoveMemory")
	procInternetSetOptionW    = wininet.NewProc("InternetSetOptionW")
	procSetWindowTheme        = uxtheme.NewProc("SetWindowTheme")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
)

type point struct{ X, Y int32 }
type message struct {
	HWnd           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             point
	LPrivate       uint32
}
type wndClassEx struct {
	CbSize, Style                      uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	IconSm                             uintptr
}
type notifyIconData struct {
	CbSize                       uint32
	HWnd                         uintptr
	UID, UFlags, CallbackMessage uint32
	Icon                         uintptr
	Tip                          [128]uint16
	State, StateMask             uint32
	Info                         [256]uint16
	TimeoutOrVersion             uint32
	InfoTitle                    [64]uint16
	InfoFlags                    uint32
	GUID                         [16]byte
	BalloonIcon                  uintptr
}
type proxyBackup struct {
	Enabled, HadAutoConfig bool
	AutoConfigURL          string `json:"auto_config_url,omitempty"`
}

type desktopApp struct {
	mu                                     sync.Mutex
	hwnd                                   uintptr
	nid                                    notifyIconData
	icon                                   uintptr
	rt                                     *clientruntime.Runtime
	ctx                                    context.Context
	cancel                                 context.CancelFunc
	dataDir                                string
	log                                    *os.File
	page                                   int
	content                                []uintptr
	controls                               map[int]uintptr
	contacts                               []string
	catalog                                []string
	catalogURLs                            []string
	integration                            bool
	startup                                bool
	menuOpen                               bool
	transportExpanded                      bool
	caAdvanced                             bool
	fontNormal, fontTitle, fontMono        uintptr
	brushBackground, brushPanel, brushEdit uintptr
}

var app *desktopApp
var taskbarCreatedMessage uint32

func runDesktop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !acquireSingleInstance() {
		return
	}
	a, err := newDesktopApp()
	if err != nil {
		messageBox(0, "KnotRoute", err.Error(), mbOK|mbIconError)
		return
	}
	app = a
	defer a.close()
	if err := a.createUI(); err != nil {
		messageBox(0, "KnotRoute", err.Error(), mbOK|mbIconError)
		return
	}
	safeGo(func() {
		if err := a.rt.Start(a.ctx); err != nil {
			a.logf("runtime start: %v", err)
			a.balloon("KnotRoute", "Не удалось запустить узел: "+err.Error(), niifWarning)
		}
	})
	procSetTimer.Call(a.hwnd, 1, 1000, 0)
	var msg message
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func newDesktopApp() (*desktopApp, error) {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return nil, errors.New("LOCALAPPDATA не задан")
	}
	dataDir := filepath.Join(local, "KnotRoute")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	logf, err := os.OpenFile(filepath.Join(dataDir, "desktop.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	rt, err := clientruntime.Open(dataDir)
	if err != nil {
		_ = logf.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &desktopApp{rt: rt, ctx: ctx, cancel: cancel, dataDir: dataDir, log: logf, page: idNavHome, controls: map[int]uintptr{}}
	a.integration = a.readIntegrationEnabled()
	a.startup = a.readStartupEnabled()
	a.brushBackground, _, _ = procCreateSolidBrush.Call(rgb(13, 17, 23))
	a.brushPanel, _, _ = procCreateSolidBrush.Call(rgb(22, 27, 34))
	a.brushEdit, _, _ = procCreateSolidBrush.Call(rgb(29, 35, 43))
	a.fontNormal = createFont(18, 400, "Segoe UI")
	a.fontTitle = createFont(30, 600, "Segoe UI")
	a.fontMono = createFont(16, 400, "Cascadia Mono")
	a.logf("KnotRoute v4 desktop starting; in-process runtime; config=%s", rt.ConfigPath())
	return a, nil
}

func (a *desktopApp) close() {
	if a.cancel != nil {
		a.cancel()
	}
	if a.rt != nil {
		a.rt.Stop()
	}
	if a.log != nil {
		a.logf("desktop stopped")
		_ = a.log.Close()
	}
	for _, o := range []uintptr{a.fontNormal, a.fontTitle, a.fontMono, a.brushBackground, a.brushPanel, a.brushEdit} {
		if o != 0 {
			procDeleteObject.Call(o)
		}
	}
	if a.hwnd != 0 {
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&a.nid)))
	}
}

func (a *desktopApp) createUI() error {
	className, _ := syscall.UTF16PtrFromString("KnotRouteV4Window")
	instance, _, _ := procGetModuleHandleW.Call(0)
	icon, _, _ := procLoadIconW.Call(0, idiApplication)
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	wc := wndClassEx{CbSize: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: syscall.NewCallback(windowProc), Instance: instance, Icon: icon, Cursor: cursor, Background: a.brushBackground, ClassName: className, IconSm: icon}
	if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return fmt.Errorf("RegisterClassExW: %v", err)
	}
	title, _ := syscall.UTF16PtrFromString("KnotRoute v4")
	hwnd, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), wsOverlappedWindow, 100, 80, 1180, 760, 0, 0, instance, 0)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowExW: %v", err)
	}
	a.hwnd, a.icon = hwnd, icon
	enabled := int32(1)
	procDwmSetWindowAttribute.Call(hwnd, 20, uintptr(unsafe.Pointer(&enabled)), unsafe.Sizeof(enabled))
	a.nid = notifyIconData{CbSize: uint32(unsafe.Sizeof(notifyIconData{})), HWnd: hwnd, UID: 1, UFlags: nifMessage | nifIcon | nifTip | nifShowTip, CallbackMessage: wmAppTray, Icon: icon}
	copyUTF16(a.nid.Tip[:], "KnotRoute v4")
	name, _ := syscall.UTF16PtrFromString("TaskbarCreated")
	if v, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(name))); v != 0 {
		taskbarCreatedMessage = uint32(v)
	}
	if err := a.addTrayIcon(); err != nil {
		return err
	}
	a.buildChrome()
	a.showPage(idNavHome)
	procShowWindow.Call(hwnd, swShow)
	procUpdateWindow.Call(hwnd)
	return nil
}

func (a *desktopApp) buildChrome() {
	a.static("KnotRoute", 26, 22, 180, 40, fontTitle)
	a.static("связь, которая сама находит дорогу", 26, 61, 210, 36, fontNormal)
	navs := []struct {
		id   int
		text string
	}{{idNavHome, "Главная"}, {idNavChats, "Чаты"}, {idNavFeed, "Лента"}, {idNavCatalog, "Каталог"}, {idNavNetwork, "Сеть"}, {idNavDeveloper, "Разработчикам"}, {idNavSettings, "Настройки"}}
	y := 126
	for _, n := range navs {
		a.button(n.id, n.text, 24, y, 205, 42)
		y += 50
	}
}

func (a *desktopApp) clearContent() {
	for _, h := range a.content {
		procDestroyWindow.Call(h)
	}
	a.content = nil
	for id, h := range a.controls {
		if id >= 1000 {
			_ = h
			delete(a.controls, id)
		}
	}
}
func (a *desktopApp) showPage(page int) {
	a.page = page
	a.clearContent()
	switch page {
	case idNavHome:
		a.homePage()
	case idNavChats:
		a.chatsPage()
	case idNavFeed:
		a.feedPage()
	case idNavCatalog:
		a.catalogPage()
	case idNavNetwork:
		a.networkPage()
	case idNavDeveloper:
		a.developerPage()
	case idNavSettings:
		a.settingsPage()
	}
}

func (a *desktopApp) homePage() {
	a.title("Главная", "Состояние сети и быстрые действия")
	st, ok := a.rt.Status()
	status := "Запуск…"
	detail := "Локальный runtime запускается внутри этого приложения."
	if ok {
		status = "Узел работает"
		detail = fmt.Sprintf("Пиров: %d · маршрутов: %d · circuits: %d", len(st.Peers), len(st.Routes), st.ActiveCircuits)
		if len(st.Peers) == 0 {
			status = "Узел работает · нет пиров"
		}
	}
	a.card("СОЕДИНЕНИЕ", status, detail, 275, 120, 820, 105)
	profile, _ := a.rt.UserProfile()
	a.card("ПРОФИЛЬ", profile.DisplayName, short(profile.ID), 275, 245, 395, 105)
	domain := a.rt.NodeDomain()
	a.card("АДРЕС УЗЛА", short(domain), "Технический адрес. Сайты и чаты используют отдельные идентификаторы.", 700, 245, 395, 105)
	transport := "direct"
	if ok && st.Transport.LastSelected != "" {
		transport = st.Transport.LastSelected
	}
	a.card("ТРАНСПОРТ", transport, "Auto может переключиться на локальный Xray SOCKS5 при недоступном direct.", 275, 370, 395, 105)
	proxy := "—"
	if ok {
		proxy = st.Proxy.HTTP
	}
	a.card(".KNOT В БРАУЗЕРАХ", proxy, "Системная PAC-интеграция направляет только .knot в локальный HTTP proxy.", 700, 370, 395, 105)
	label := "Включить .knot в браузерах"
	if a.integration {
		label = "Отключить .knot в браузерах"
	}
	a.contentButton(idIntegration, label, 275, 510, 310, 44)
	runLabel := "Остановить сеть"
	if !a.rt.Running() {
		runLabel = "Запустить сеть"
	}
	a.contentButton(idStartStop, runLabel, 600, 510, 220, 44)
}

func (a *desktopApp) chatsPage() {
	a.title("Чаты", "v4 messenger: пользовательская identity отделена от node identity")
	state := a.rt.SocialState()
	a.contacts = a.contacts[:0]
	list := a.listbox(idContactsList, 275, 150, 320, 350)
	for id, c := range state.Contacts {
		display := c.Profile.DisplayName
		if c.Alias != "" {
			display = c.Alias
		}
		addList(list, display+"  ·  "+short(id))
		a.contacts = append(a.contacts, id)
	}
	a.edit(idContactNode, "kr_… или node.knot", 275, 520, 320, 34, false)
	a.edit(idContactAlias, "Имя контакта (необязательно)", 275, 562, 320, 34, false)
	a.contentButton(idAddContact, "Добавить контакт", 275, 608, 180, 40)
	messages := a.listbox(idMessagesList, 625, 150, 470, 350)
	if len(a.contacts) > 0 {
		a.renderMessages(messages, a.contacts[0])
	}
	a.edit(idMessageBody, "Сообщение…", 625, 520, 470, 72, true)
	a.contentButton(idSendMessage, "Отправить", 915, 608, 180, 40)
	a.contentButton(idRefreshChat, "Обновить", 785, 608, 118, 40)
}

func (a *desktopApp) renderMessages(list uintptr, userID string) {
	procSendMessageW.Call(list, 0x0184, 0, 0) // LB_RESETCONTENT
	for _, m := range a.rt.SocialState().Messages[userID] {
		who := "Вы"
		if m.Sender.ID == userID {
			who = m.Sender.DisplayName
		}
		addList(list, fmt.Sprintf("%s: %s", who, m.Body))
	}
}

func (a *desktopApp) feedPage() {
	a.title("Лента", "Подписанные посты ваших контактов и ваш собственный профиль")
	list := a.listbox(idFeedList, 275, 145, 820, 365)
	state := a.rt.SocialState()
	for _, post := range state.Posts {
		name := post.Author.DisplayName
		if name == "" {
			name = short(post.Author.ID)
		}
		stamp := time.Unix(post.CreatedUnix, 0).Local().Format("02.01 15:04")
		addList(list, fmt.Sprintf("%s · %s  —  %s", name, stamp, strings.ReplaceAll(post.Text, "\n", " ")))
	}
	if len(state.Posts) == 0 {
		addList(list, "Постов пока нет. Добавьте контакты или опубликуйте первый пост.")
	}
	a.edit(idPostBody, "Что нового?", 275, 530, 820, 64, true)
	a.contentButton(idPublishPost, "Опубликовать", 275, 610, 180, 40)
	a.contentButton(idRefreshFeed, "Обновить от контактов", 470, 610, 220, 40)
	a.contentStatic("Alpha: лента синхронизируется напрямую с online-контактами. Offline mailbox и распределённые подписки будут добавлены отдельно.", 710, 608, 385, 48, fontNormal)
}

func (a *desktopApp) publishPost() {
	body := strings.TrimSpace(a.text(idPostBody))
	if body == "" {
		a.error(errors.New("текст поста пуст"))
		return
	}
	if _, err := a.rt.CreatePost(body, nil); err != nil {
		a.error(err)
		return
	}
	a.showPage(idNavFeed)
}

func (a *desktopApp) refreshFeed() {
	state := a.rt.SocialState()
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	var failures []string
	for id, contact := range state.Contacts {
		if _, err := a.rt.FetchContactFeed(ctx, id); err != nil {
			name := contact.Profile.DisplayName
			if name == "" {
				name = short(id)
			}
			failures = append(failures, name+": "+err.Error())
		}
	}
	a.showPage(idNavFeed)
	if len(failures) > 0 {
		a.info("Лента обновлена частично: " + strings.Join(failures, "; "))
	}
}

func (a *desktopApp) catalogPage() {
	a.title("Каталог", "Наблюдаемые сервисы сети. Это не глобальный список всех hidden services.")
	list := a.listbox(idCatalogList, 275, 150, 820, 430)
	a.catalog, a.catalogURLs = nil, nil
	if st, ok := a.rt.Status(); ok {
		for _, r := range st.Routes {
			for _, svc := range r.Services {
				if svc == "kr-chat" {
					continue
				}
				domain := svc + "." + r.Domain
				a.catalog = append(a.catalog, svc+" · "+domain)
				a.catalogURLs = append(a.catalogURLs, "https://"+domain+"/")
			}
		}
		for _, svc := range st.KnownServices {
			title := svc.Metadata["title"]
			if title == "" {
				title = "Опубликованный сервис"
			}
			a.catalog = append(a.catalog, title+" · "+svc.Domain)
			a.catalogURLs = append(a.catalogURLs, "https://"+svc.Domain+"/")
		}
	}
	for _, v := range a.catalog {
		addList(list, v)
	}
	a.contentButton(idOpenCatalog, "Открыть в браузере", 275, 600, 210, 42)
	a.contentButton(idRefreshCatalog, "Обновить", 500, 600, 130, 42)
}

func (a *desktopApp) networkPage() {
	a.title("Сеть", "Основные параметры сверху; редкие транспортные настройки скрыты")
	cfg := a.rt.Config()
	a.contentStatic("Network ID", 275, 135, 200, 28, fontNormal)
	a.editValue(idNetwork, cfg.NetworkID, 275, 165, 820, 34, false)
	a.contentStatic("Beacon API URLs (по одному на строку)", 275, 213, 420, 28, fontNormal)
	a.editValue(idBeacons, strings.Join(cfg.Discovery.Beacons, "\r\n"), 275, 243, 820, 92, true)
	a.contentStatic("Circuit hops", 275, 350, 160, 28, fontNormal)
	a.editValue(idHops, strconv.Itoa(cfg.Privacy.CircuitHops), 275, 380, 100, 32, false)
	toggle := "Дополнительные параметры транспорта ▾"
	if a.transportExpanded {
		toggle = "Скрыть параметры транспорта ▴"
	}
	a.contentButton(idToggleTransport, toggle, 275, 435, 315, 40)
	if a.transportExpanded {
		combo := a.combo(idTransportMode, 275, 500, 260, 180)
		addCombo(combo, "Авто: direct → Xray")
		addCombo(combo, "Только direct")
		addCombo(combo, "Через Xray/SOCKS5")
		sel := 0
		if cfg.Transport.Mode == "direct" {
			sel = 1
		} else if cfg.Transport.Mode == "proxy" {
			sel = 2
		}
		procSendMessageW.Call(combo, 0x014E, uintptr(sel), 0)
		socks := ""
		for _, ep := range cfg.Transport.Endpoints {
			if ep.Type == "socks5" && ep.Enabled {
				socks = ep.Endpoint
				break
			}
		}
		a.contentStatic("Локальный SOCKS5 Xray", 560, 472, 300, 28, fontNormal)
		a.editValue(idTransportSOCKS, socks, 560, 500, 350, 34, false)
		a.contentStatic("Например 127.0.0.1:10808. Xray здесь — только transport overlay-пиров; identity, routing и services остаются KnotRoute.", 560, 539, 535, 45, fontNormal)
	}
	a.contentButton(idSaveNetwork, "Сохранить и перезапустить", 275, 600, 260, 44)
}

func (a *desktopApp) developerPage() {
	a.title("Разработчикам", "Сервисы, идентификаторы, алиасы и локальные пути — отдельно от обычного UX")
	cfg := a.rt.Config()
	st, _ := a.rt.Status()
	var b strings.Builder
	fmt.Fprintf(&b, "KnotRoute v4 developer reference\r\n\r\nkn_  Network ID — изолирует overlay namespace. Это не пароль.\r\nkr_  Node ID — криптографическая identity конкретного узла.\r\nku_  User ID — identity пользователя мессенджера/социального слоя.\r\nks_  Service ID — переносимая identity опубликованного сервиса.\r\n\r\nNODE .knot\r\n%s\r\nТехнический адрес узла. Сервис web на узле: web.<node>.knot.\r\n\r\nCANONICAL SERVICE .knot\r\nОпубликованный service ID получает отдельный .knot и может переезжать между узлами.\r\n\r\nPATHS\r\nconfig: %s\r\ndata:   %s\r\nidentity: %s\r\nCA:       %s\r\n\r\nALIASES\r\nАлиас — локальное удобное имя в конфиге; он не меняет криптографическую identity.\r\n\r\nSERVICES\r\n", a.rt.NodeDomain(), a.rt.ConfigPath(), a.dataDir, cfg.IdentityFile, cfg.CA.Directory)
	for _, svc := range st.Services {
		fmt.Fprintf(&b, "- %s → %s", svc.Name, svc.Target)
		if svc.Domain != "" {
			fmt.Fprintf(&b, " · %s", svc.Domain)
		}
		fmt.Fprintln(&b)
	}
	fmt.Fprintf(&b, "\r\nКак публиковать Docker-сервис:\r\nsidecar получает target container:port, хранит service identity в persistent /data и публикует descriptor через introduction points. Не удаляйте volume /data при миграции, если хотите сохранить .knot identity.\r\n\r\nТранспорт Xray:\r\nПоднимите локальный SOCKS5 inbound в Xray и укажите его в разделе «Сеть → Дополнительно». KnotRoute остаётся overlay/service layer, Xray отвечает только за доставку peer TCP-потока.")
	a.readOnly(b.String(), 275, 135, 820, 500)
	a.contentButton(idDiagnostics, "Собрать диагностику", 275, 650, 200, 40)
}

func (a *desktopApp) settingsPage() {
	a.title("Настройки", "Профиль, автозапуск и ваш локальный Root CA")
	state := a.rt.SocialState()
	a.contentStatic("Имя", 275, 130, 180, 24, fontNormal)
	a.editValue(idProfileName, state.DisplayName, 275, 158, 390, 32, false)
	a.contentStatic("О себе", 690, 130, 180, 24, fontNormal)
	a.editValue(idProfileBio, state.Bio, 690, 158, 405, 64, true)
	a.contentButton(idSaveProfile, "Сохранить профиль", 275, 232, 180, 40)
	cfg := a.rt.Config()
	a.contentStatic("Root CA subject / issuer", 275, 294, 420, 34, fontTitle)
	labels := []struct {
		id        int
		name, val string
		x, y, w   int
	}{{idCACN, "Common Name (CN)", cfg.CA.Subject.CommonName, 275, 338, 390}, {idCAO, "Organization (O)", strings.Join(cfg.CA.Subject.Organization, ", "), 690, 338, 405}, {idCAOU, "Org Unit (OU)", strings.Join(cfg.CA.Subject.OrganizationalUnit, ", "), 275, 408, 390}, {idCAC, "Country (C)", strings.Join(cfg.CA.Subject.Country, ", "), 690, 408, 190}, {idCAST, "Region (ST)", strings.Join(cfg.CA.Subject.Province, ", "), 895, 408, 200}, {idCAL, "Locality (L)", strings.Join(cfg.CA.Subject.Locality, ", "), 275, 478, 390}, {idCAValidity, "Срок, дней", strconv.Itoa(cfg.CA.ValidityDays), 690, 478, 190}}
	for _, f := range labels {
		a.contentStatic(f.name, f.x, f.y, f.w, 22, fontNormal)
		a.editValue(f.id, f.val, f.x, f.y+24, f.w, 30, false)
	}
	toggleText := "Дополнительные поля сертификата ▾"
	if a.caAdvanced {
		toggleText = "Дополнительные поля сертификата ▴"
	}
	a.contentButton(idToggleCAAdvanced, toggleText, 275, 548, 290, 34)
	a.contentStatic("У self-signed Root CA Issuer = Subject. Изменение профиля требует явной ротации.", 585, 548, 510, 36, fontNormal)
	buttonY := 600
	if a.caAdvanced {
		a.contentStatic("Street address", 275, 590, 390, 22, fontNormal)
		a.editValue(idCAStreet, strings.Join(cfg.CA.Subject.StreetAddress, ", "), 275, 614, 390, 30, false)
		a.contentStatic("Postal code", 690, 590, 200, 22, fontNormal)
		a.editValue(idCAPostal, strings.Join(cfg.CA.Subject.PostalCode, ", "), 690, 614, 200, 30, false)
		buttonY = 654
	}
	a.contentButton(idSaveCAProfile, "Сохранить параметры CA", 275, buttonY, 210, 40)
	a.contentButton(idRotateCA, "Перевыпустить Root CA", 500, buttonY, 210, 40)
	a.contentButton(idInstallCA, "Установить Root CA", 725, buttonY, 190, 40)
	a.contentButton(idOpenData, "Папка данных", 930, buttonY, 165, 40)
	startText := "Включить автозапуск"
	if a.startup {
		startText = "Отключить автозапуск"
	}
	if !a.caAdvanced {
		a.contentButton(idStartup, startText, 275, 650, 210, 38)
	}
}

func windowProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) (result uintptr) {
	defer func() {
		if r := recover(); r != nil {
			if app != nil {
				app.logf("panic in windowProc: %v\n%s", r, debug.Stack())
			}
			result = 0
		}
	}()
	if app == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return r
	}
	if taskbarCreatedMessage != 0 && msg == taskbarCreatedMessage {
		_ = app.addTrayIcon()
		return 0
	}
	switch msg {
	case wmCommand:
		id := int(uint16(wParam & 0xffff))
		app.handleCommand(id)
		return 0
	case wmTimer:
		if app.page == idNavHome {
			app.showPage(idNavHome)
		}
		return 0
	case wmCtlColorStatic:
		procSetTextColor.Call(wParam, rgb(226, 232, 240))
		procSetBkColor.Call(wParam, rgb(13, 17, 23))
		return app.brushBackground
	case wmCtlColorEdit, wmCtlColorListBox:
		procSetTextColor.Call(wParam, rgb(237, 242, 247))
		procSetBkColor.Call(wParam, rgb(29, 35, 43))
		return app.brushEdit
	case wmCtlColorBtn:
		procSetTextColor.Call(wParam, rgb(226, 232, 240))
		procSetBkColor.Call(wParam, rgb(22, 27, 34))
		return app.brushPanel
	case wmAppTray:
		event := uint32(lParam & 0xffff)
		if event == wmLButtonDblClk {
			app.showMain()
		} else if event == wmRButtonUp || event == wmContextMenu {
			app.showTrayMenu()
		}
		return 0
	case wmClose:
		procShowWindow.Call(hwnd, swHide)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func (a *desktopApp) handleCommand(id int) {
	if id >= idNavHome && id <= idNavSettings {
		a.showPage(id)
		return
	}
	switch id {
	case idStartStop:
		safeGo(func() {
			if a.rt.Running() {
				a.rt.Stop()
			} else if err := a.rt.Start(a.ctx); err != nil {
				a.error(err)
			}
			a.showPage(idNavHome)
		})
	case idIntegration:
		safeGo(func() {
			if a.integration {
				if err := a.disableIntegration(); err != nil {
					a.error(err)
				}
			} else if err := a.enableIntegration(); err != nil {
				a.error(err)
			}
			a.showPage(idNavHome)
		})
	case idAddContact:
		safeGo(a.addContact)
	case idSendMessage:
		safeGo(a.sendMessage)
	case idRefreshChat:
		a.showPage(idNavChats)
	case idPublishPost:
		safeGo(a.publishPost)
	case idRefreshFeed:
		safeGo(a.refreshFeed)
	case idOpenCatalog:
		a.openSelectedCatalog()
	case idRefreshCatalog:
		a.showPage(idNavCatalog)
	case idSaveNetwork:
		safeGo(a.saveNetwork)
	case idToggleTransport:
		a.transportExpanded = !a.transportExpanded
		a.showPage(idNavNetwork)
	case idSaveProfile:
		if err := a.rt.SetUserProfile(a.text(idProfileName), a.text(idProfileBio)); err != nil {
			a.error(err)
		} else {
			a.info("Профиль сохранён")
		}
	case idSaveCAProfile:
		a.saveCAProfile(false)
	case idRotateCA:
		a.saveCAProfile(true)
	case idToggleCAAdvanced:
		a.caAdvanced = !a.caAdvanced
		a.showPage(idNavSettings)
	case idInstallCA:
		safeGo(func() {
			if err := a.rt.InstallCA(); err != nil {
				a.error(err)
			} else {
				a.info("Root CA установлен для текущего пользователя Windows")
			}
		})
	case idOpenData:
		shellOpen(a.dataDir)
	case idDiagnostics:
		if p, err := a.writeDiagnostics(); err != nil {
			a.error(err)
		} else {
			shellOpen(p)
		}
	case idStartup:
		if err := a.setStartup(!a.startup); err != nil {
			a.error(err)
		} else {
			a.startup = !a.startup
			a.showPage(idNavSettings)
		}
	case cmdTrayOpen:
		a.showMain()
	case cmdTrayToggle:
		safeGo(func() {
			if a.rt.Running() {
				a.rt.Stop()
			} else {
				_ = a.rt.Start(a.ctx)
			}
		})
	case cmdTrayIntegration:
		a.handleCommand(idIntegration)
	case cmdTrayDiagnostics:
		a.handleCommand(idDiagnostics)
	case cmdTrayExit:
		procKillTimer.Call(a.hwnd, 1)
		procDestroyWindow.Call(a.hwnd)
	}
}

func (a *desktopApp) addContact() {
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	c, err := a.rt.AddContact(ctx, a.text(idContactNode), a.text(idContactAlias))
	if err != nil {
		a.error(err)
		return
	}
	a.info("Добавлен контакт: " + c.Profile.DisplayName)
	a.showPage(idNavChats)
}
func (a *desktopApp) selectedContact() string {
	h := a.controls[idContactsList]
	if h == 0 {
		return ""
	}
	idx, _, _ := procSendMessageW.Call(h, 0x0188, 0, 0)
	if int32(idx) < 0 || int(idx) >= len(a.contacts) {
		if len(a.contacts) > 0 {
			return a.contacts[0]
		}
		return ""
	}
	return a.contacts[int(idx)]
}
func (a *desktopApp) sendMessage() {
	id := a.selectedContact()
	if id == "" {
		a.error(errors.New("сначала выберите контакт"))
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, 25*time.Second)
	defer cancel()
	if _, err := a.rt.SendMessage(ctx, id, a.text(idMessageBody)); err != nil {
		a.error(err)
		return
	}
	a.showPage(idNavChats)
}
func (a *desktopApp) openSelectedCatalog() {
	h := a.controls[idCatalogList]
	if h == 0 {
		return
	}
	idx, _, _ := procSendMessageW.Call(h, 0x0188, 0, 0)
	if int32(idx) >= 0 && int(idx) < len(a.catalogURLs) {
		shellOpen(a.catalogURLs[int(idx)])
	}
}

func (a *desktopApp) saveNetwork() {
	cfg := a.rt.Config()
	cfg.NetworkID = strings.TrimSpace(a.text(idNetwork))
	lines := strings.FieldsFunc(a.text(idBeacons), func(r rune) bool { return r == '\r' || r == '\n' })
	cfg.Discovery.Beacons = nil
	for _, v := range lines {
		v = strings.TrimSpace(v)
		if v != "" {
			cfg.Discovery.Beacons = append(cfg.Discovery.Beacons, v)
		}
	}
	if n, err := strconv.Atoi(strings.TrimSpace(a.text(idHops))); err == nil {
		cfg.Privacy.CircuitHops = n
	}
	if combo := a.controls[idTransportMode]; combo != 0 {
		sel, _, _ := procSendMessageW.Call(combo, 0x0147, 0, 0)
		switch sel {
		case 1:
			cfg.Transport.Mode = "direct"
		case 2:
			cfg.Transport.Mode = "proxy"
		default:
			cfg.Transport.Mode = "auto"
		}
		cfg.Transport.DirectFirst = cfg.Transport.Mode != "proxy"
		cfg.Transport.FallbackDirect = cfg.Transport.Mode != "proxy"
		socks := strings.TrimSpace(a.text(idTransportSOCKS))
		cfg.Transport.Endpoints = nil
		if socks != "" {
			cfg.Transport.Endpoints = []config.TransportEndpoint{{Name: "xray", Type: "socks5", Endpoint: socks, Enabled: true, Priority: 10}}
		}
	}
	if err := a.rt.SaveConfig(cfg); err != nil {
		a.error(err)
		return
	}
	if err := a.rt.Restart(a.ctx); err != nil {
		a.error(err)
		return
	}
	a.info("Сеть сохранена и перезапущена")
	a.showPage(idNavNetwork)
}

func (a *desktopApp) saveCAProfile(rotate bool) {
	cfg := a.rt.Config()
	cfg.CA.Subject.CommonName = strings.TrimSpace(a.text(idCACN))
	cfg.CA.Subject.Organization = csv(a.text(idCAO))
	cfg.CA.Subject.OrganizationalUnit = csv(a.text(idCAOU))
	cfg.CA.Subject.Country = csv(a.text(idCAC))
	cfg.CA.Subject.Province = csv(a.text(idCAST))
	cfg.CA.Subject.Locality = csv(a.text(idCAL))
	if _, ok := a.controls[idCAStreet]; ok {
		cfg.CA.Subject.StreetAddress = csv(a.text(idCAStreet))
	}
	if _, ok := a.controls[idCAPostal]; ok {
		cfg.CA.Subject.PostalCode = csv(a.text(idCAPostal))
	}
	if v, err := strconv.Atoi(strings.TrimSpace(a.text(idCAValidity))); err == nil {
		cfg.CA.ValidityDays = v
	}
	if err := a.rt.SaveConfig(cfg); err != nil {
		a.error(err)
		return
	}
	if !rotate {
		a.info("Профиль CA сохранён. Текущий Root CA не изменён.")
		return
	}
	if messageBox(a.hwnd, "KnotRoute", "Перевыпустить Root CA? Старый сертификат перестанет подписывать новые .knot-сертификаты, а новый нужно заново установить на все клиентские устройства.", mbYesNo|mbIconWarning) != idYes {
		return
	}
	safeGo(func() {
		info, err := a.rt.RotateCA()
		if err != nil {
			a.error(err)
			return
		}
		if err := a.rt.InstallCA(); err != nil {
			a.error(fmt.Errorf("CA перевыпущен, но установка в Windows не удалась: %w", err))
			return
		}
		if err := a.rt.Restart(a.ctx); err != nil {
			a.error(fmt.Errorf("CA перевыпущен и установлен, но KnotRoute не удалось перезапустить: %w", err))
			return
		}
		a.info("Новый Root CA установлен и активирован. SHA-256: " + info.Fingerprint)
	})
}

func (a *desktopApp) writeDiagnostics() (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "KnotRoute v4 diagnostics\r\ncreated: %s\r\nconfig: %s\r\ndata: %s\r\nrunning: %t\r\nlast error: %s\r\nuser: %s\r\nnode: %s\r\n\r\n", time.Now().Format(time.RFC3339), a.rt.ConfigPath(), a.dataDir, a.rt.Running(), a.rt.LastError(), a.rt.UserID(), a.rt.NodeDomain())
	if st, ok := a.rt.Status(); ok {
		raw, _ := json.MarshalIndent(st, "", "  ")
		b.Write(raw)
		b.WriteString("\r\n")
	}
	b.WriteString("\r\ndesktop.log tail\r\n")
	b.WriteString(tailFile(filepath.Join(a.dataDir, "desktop.log"), 24<<10))
	path := filepath.Join(a.dataDir, "diagnostics-"+time.Now().Format("20060102-150405")+".txt")
	return path, os.WriteFile(path, []byte(b.String()), 0o600)
}

func (a *desktopApp) title(title, subtitle string) {
	a.contentStatic(title, 275, 32, 820, 42, fontTitle)
	a.contentStatic(subtitle, 275, 76, 820, 34, fontNormal)
}
func (a *desktopApp) card(kicker, value, detail string, x, y, w, h int) {
	a.contentStatic(kicker, x+14, y+10, w-28, 20, fontNormal)
	a.contentStatic(value, x+14, y+35, w-28, 28, fontTitle)
	a.contentStatic(detail, x+14, y+70, w-28, h-72, fontNormal)
}

func (a *desktopApp) static(text string, x, y, w, h, which int) uintptr {
	return a.createControl("STATIC", text, wsChild|wsVisible, 0, x, y, w, h, 0, which, false)
}
func (a *desktopApp) contentStatic(text string, x, y, w, h, which int) uintptr {
	return a.createControl("STATIC", text, wsChild|wsVisible, 0, x, y, w, h, 0, which, true)
}
func (a *desktopApp) button(id int, text string, x, y, w, h int) uintptr {
	return a.createControl("BUTTON", text, wsChild|wsVisible|wsTabStop|bsPushButton, 0, x, y, w, h, id, fontNormal, false)
}
func (a *desktopApp) contentButton(id int, text string, x, y, w, h int) uintptr {
	return a.createControl("BUTTON", text, wsChild|wsVisible|wsTabStop|bsPushButton, 0, x, y, w, h, id, fontNormal, true)
}
func (a *desktopApp) edit(id int, placeholder string, x, y, w, h int, multi bool) uintptr {
	hnd := a.editValue(id, "", x, y, w, h, multi)
	setCue(hnd, placeholder)
	return hnd
}
func (a *desktopApp) editValue(id int, value string, x, y, w, h int, multi bool) uintptr {
	style := uint32(wsChild | wsVisible | wsTabStop | wsBorder | esAutoHScroll)
	if multi {
		style = wsChild | wsVisible | wsTabStop | wsBorder | esMultiline | esAutoVScroll | esWantReturn | wsVScroll
	}
	return a.createControl("EDIT", value, style, 0, x, y, w, h, id, fontNormal, true)
}
func (a *desktopApp) readOnly(text string, x, y, w, h int) uintptr {
	return a.createControl("EDIT", text, wsChild|wsVisible|wsBorder|esMultiline|esAutoVScroll|esReadOnly|wsVScroll, 0, x, y, w, h, 0, fontMono, true)
}
func (a *desktopApp) listbox(id, x, y, w, h int) uintptr {
	return a.createControl("LISTBOX", "", wsChild|wsVisible|wsBorder|wsVScroll|lbsNotify, 0, x, y, w, h, id, fontNormal, true)
}
func (a *desktopApp) combo(id, x, y, w, h int) uintptr {
	return a.createControl("COMBOBOX", "", wsChild|wsVisible|wsTabStop|cbsDropDownList, 0, x, y, w, h, id, fontNormal, true)
}
func (a *desktopApp) createControl(class, text string, style uint32, ex uint32, x, y, w, h, id, which int, content bool) uintptr {
	c, _ := syscall.UTF16PtrFromString(class)
	t, _ := syscall.UTF16PtrFromString(text)
	instance, _, _ := procGetModuleHandleW.Call(0)
	hwnd, _, _ := procCreateWindowExW.Call(uintptr(ex), uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(t)), uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), a.hwnd, uintptr(id), instance, 0)
	if hwnd != 0 {
		darkTheme, _ := syscall.UTF16PtrFromString("DarkMode_Explorer")
		procSetWindowTheme.Call(hwnd, uintptr(unsafe.Pointer(darkTheme)), 0)
		font := a.fontNormal
		if which == fontTitle {
			font = a.fontTitle
		} else if which == fontMono {
			font = a.fontMono
		}
		procSendMessageW.Call(hwnd, 0x0030, font, 1)
		if id != 0 {
			a.controls[id] = hwnd
		}
		if content {
			a.content = append(a.content, hwnd)
		}
	}
	return hwnd
}
func (a *desktopApp) text(id int) string {
	h := a.controls[id]
	if h == 0 {
		return ""
	}
	n, _, _ := procGetWindowTextLengthW.Call(h)
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(h, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return syscall.UTF16ToString(buf)
}

func (a *desktopApp) showMain() {
	procShowWindow.Call(a.hwnd, swRestore)
	procSetForegroundWindow.Call(a.hwnd)
}
func (a *desktopApp) addTrayIcon() error {
	if r, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&a.nid))); r == 0 {
		return err
	}
	nid := a.nid
	nid.TimeoutOrVersion = notifyVersion4
	procShellNotifyIconW.Call(nimSetVersion, uintptr(unsafe.Pointer(&nid)))
	return nil
}
func (a *desktopApp) showTrayMenu() {
	a.mu.Lock()
	if a.menuOpen {
		a.mu.Unlock()
		return
	}
	a.menuOpen = true
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.menuOpen = false; a.mu.Unlock() }()
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)
	appendMenu(menu, mfString, cmdTrayOpen, "Открыть KnotRoute")
	label := "Остановить сеть"
	if !a.rt.Running() {
		label = "Запустить сеть"
	}
	appendMenu(menu, mfString, cmdTrayToggle, label)
	appendMenu(menu, mfSeparator, 0, "")
	intLabel := "Включить .knot в браузерах"
	if a.integration {
		intLabel = "Отключить .knot в браузерах"
	}
	appendMenu(menu, mfString, cmdTrayIntegration, intLabel)
	appendMenu(menu, mfString, cmdTrayDiagnostics, "Собрать диагностику")
	appendMenu(menu, mfSeparator, 0, "")
	appendMenu(menu, mfString, cmdTrayExit, "Выход")
	var p point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	procSetForegroundWindow.Call(a.hwnd)
	sel, _, _ := procTrackPopupMenu.Call(menu, tpmRightButton|tpmReturnCmd|tmpNoNotify, uintptr(p.X), uintptr(p.Y), 0, a.hwnd, 0)
	procPostMessageW.Call(a.hwnd, wmNull, 0, 0)
	procShellNotifyIconW.Call(nimSetFocus, uintptr(unsafe.Pointer(&a.nid)))
	if sel != 0 {
		a.handleCommand(int(sel))
	}
}
func appendMenu(menu uintptr, flags uint32, id int, text string) {
	p, _ := syscall.UTF16PtrFromString(text)
	procAppendMenuW.Call(menu, uintptr(flags), uintptr(id), uintptr(unsafe.Pointer(p)))
}
func (a *desktopApp) balloon(title, text string, flags uint32) {
	nid := a.nid
	nid.UFlags = nifInfo
	copyUTF16(nid.InfoTitle[:], title)
	copyUTF16(nid.Info[:], text)
	nid.InfoFlags = flags
	procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

func (a *desktopApp) enableIntegration() error {
	if messageBox(a.hwnd, "KnotRoute", "KnotRoute установит ваш локальный Root CA в хранилище текущего пользователя и включит PAC только для .knot. Продолжить?", mbYesNo|mbIconWarning) != idYes {
		return errors.New("операция отменена")
	}
	if err := a.rt.InstallCA(); err != nil {
		return err
	}
	current, exists := regQuery("AutoConfigURL")
	pac := "http://127.0.0.1:8484/proxy.pac"
	state := proxyBackup{Enabled: true, HadAutoConfig: exists, AutoConfigURL: current}
	raw, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(filepath.Join(a.dataDir, "proxy-state.json"), raw, 0o600); err != nil {
		return err
	}
	if err := regSet("AutoConfigURL", pac); err != nil {
		return err
	}
	refreshInternetSettings()
	a.integration = true
	return nil
}
func (a *desktopApp) disableIntegration() error {
	path := filepath.Join(a.dataDir, "proxy-state.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var state proxyBackup
	if err := json.Unmarshal(raw, &state); err != nil {
		return err
	}
	if state.HadAutoConfig {
		if err := regSet("AutoConfigURL", state.AutoConfigURL); err != nil {
			return err
		}
	} else if err := regDelete("AutoConfigURL"); err != nil {
		return err
	}
	_ = a.rt.UninstallCA()
	state.Enabled = false
	raw, _ = json.MarshalIndent(state, "", "  ")
	_ = os.WriteFile(path, raw, 0o600)
	refreshInternetSettings()
	a.integration = false
	return nil
}
func (a *desktopApp) readIntegrationEnabled() bool {
	raw, err := os.ReadFile(filepath.Join(a.dataDir, "proxy-state.json"))
	if err != nil {
		return false
	}
	var s proxyBackup
	return json.Unmarshal(raw, &s) == nil && s.Enabled
}

func (a *desktopApp) readStartupEnabled() bool {
	v, ok := regQueryAt(`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "KnotRoute")
	return ok && v != ""
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
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) >= 3 && strings.EqualFold(f[0], name) {
			return strings.Join(f[2:], " "), true
		}
	}
	return "", false
}
func regSet(name, value string) error { return regSetAt(internetSettingsKey, name, value) }
func regSetAt(key, name, value string) error {
	cmd := exec.Command("reg.exe", "add", key, "/v", name, "/t", "REG_SZ", "/d", value, "/f")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("registry: %v: %s", err, strings.TrimSpace(string(out)))
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
	_, err := cmd.CombinedOutput()
	return err
}
func refreshInternetSettings() {
	for _, opt := range []uintptr{39, 95, 37} {
		procInternetSetOptionW.Call(0, opt, 0, 0)
	}
}

func acquireSingleInstance() bool {
	name, _ := syscall.UTF16PtrFromString("Local\\KnotRouteDesktopV4")
	_, _, err := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if errno, ok := err.(syscall.Errno); ok && errno == 183 {
		return false
	}
	return true
}
func createFont(height, weight int, name string) uintptr {
	p, _ := syscall.UTF16PtrFromString(name)
	h, _, _ := procCreateFontW.Call(uintptr(-height), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(p)))
	return h
}
func rgb(r, g, b uint32) uintptr { return uintptr(r | (g << 8) | (b << 16)) }
func copyUTF16(dst []uint16, s string) {
	src := syscall.StringToUTF16(s)
	copy(dst, src[:min(len(src), len(dst))])
}
func addList(h uintptr, s string) {
	p, _ := syscall.UTF16PtrFromString(s)
	procSendMessageW.Call(h, 0x0180, 0, uintptr(unsafe.Pointer(p)))
}
func addCombo(h uintptr, s string) {
	p, _ := syscall.UTF16PtrFromString(s)
	procSendMessageW.Call(h, 0x0143, 0, uintptr(unsafe.Pointer(p)))
}
func setCue(h uintptr, s string) {
	p, _ := syscall.UTF16PtrFromString(s)
	procSendMessageW.Call(h, 0x1501, 1, uintptr(unsafe.Pointer(p)))
}
func csv(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
func short(s string) string {
	if len([]rune(s)) <= 38 {
		return s
	}
	r := []rune(s)
	return string(r[:18]) + "…" + string(r[len(r)-14:])
}
func shellOpen(target string) {
	verb, _ := syscall.UTF16PtrFromString("open")
	p, _ := syscall.UTF16PtrFromString(target)
	procShellExecuteW.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(p)), 0, 0, 1)
}
func messageBox(hwnd uintptr, title, text string, flags uint32) int {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	r, _, _ := procMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), uintptr(flags))
	return int(r)
}
func (a *desktopApp) info(s string) { a.balloon("KnotRoute", s, niifInfo) }
func (a *desktopApp) error(err error) {
	a.logf("error: %v", err)
	messageBox(a.hwnd, "KnotRoute", err.Error(), mbOK|mbIconError)
}
func (a *desktopApp) logf(f string, args ...any) {
	if a.log != nil {
		fmt.Fprintf(a.log, "%s %s\n", time.Now().Format(time.RFC3339Nano), fmt.Sprintf(f, args...))
		_ = a.log.Sync()
	}
}
func safeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil && app != nil {
				app.logf("background panic: %v\n%s", r, debug.Stack())
			}
		}()
		fn()
	}()
}
func tailFile(path string, limit int64) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return ""
	}
	start := st.Size() - limit
	if start < 0 {
		start = 0
	}
	_, _ = f.Seek(start, 0)
	raw := make([]byte, st.Size()-start)
	n, _ := f.Read(raw)
	return string(raw[:n])
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
	procRtlMoveMemory.Call(ptr, uintptr(unsafe.Pointer(&utf[0])), size)
	procGlobalUnlock.Call(h)
	if r, _, err := procSetClipboardData.Call(cfUnicodeText, h); r == 0 {
		return err
	}
	return nil
}

var _ = social.State{}
var _ = setClipboard
