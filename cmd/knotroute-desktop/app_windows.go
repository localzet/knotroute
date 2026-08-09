//go:build windows

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/localzet/knotroute/internal/certauth"
	"github.com/localzet/knotroute/internal/clientruntime"
	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/overlay"
	transportpkg "github.com/localzet/knotroute/internal/transport"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	rt      *clientruntime.Runtime
	dataDir string

	mu            sync.RWMutex
	uiCtx         context.Context
	nodeCtx       context.Context
	nodeCancel    context.CancelFunc
	operation     string
	operationErr  string
	lastStateHash string
	ready         bool
	integration   bool
	startupOn     bool
	caInfo        *certauth.Info
}

type DesktopSnapshot struct {
	Version     string        `json:"version"`
	Running     bool          `json:"running"`
	Operation   string        `json:"operation,omitempty"`
	Error       string        `json:"error,omitempty"`
	Integration bool          `json:"integration"`
	Startup     bool          `json:"startup"`
	Overview    OverviewView  `json:"overview"`
	Profile     ProfileView   `json:"profile"`
	Contacts    []ContactView `json:"contacts"`
	Posts       []PostView    `json:"posts"`
	Catalog     []CatalogView `json:"catalog"`
	Network     NetworkView   `json:"network"`
	Developer   DeveloperView `json:"developer"`
	CA          CAView        `json:"ca"`
	Paths       PathView      `json:"paths"`
}

type OverviewView struct {
	State        string `json:"state"`
	StateDetail  string `json:"state_detail"`
	PeerCount    int    `json:"peer_count"`
	RouteCount   int    `json:"route_count"`
	ServiceCount int    `json:"service_count"`
	Transport    string `json:"transport"`
	NodeDomain   string `json:"node_domain"`
	NetworkID    string `json:"network_id"`
	BytesSent    uint64 `json:"bytes_sent"`
	BytesRecv    uint64 `json:"bytes_recv"`
}

type ProfileView struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
}

type ContactView struct {
	ID          string        `json:"id"`
	Node        string        `json:"node"`
	DisplayName string        `json:"display_name"`
	Alias       string        `json:"alias,omitempty"`
	Bio         string        `json:"bio,omitempty"`
	Messages    []MessageView `json:"messages"`
	LastMessage string        `json:"last_message,omitempty"`
	LastUnix    int64         `json:"last_unix,omitempty"`
}

type MessageView struct {
	ID       string `json:"id"`
	Mine     bool   `json:"mine"`
	Body     string `json:"body"`
	Unix     int64  `json:"unix"`
	SenderID string `json:"sender_id"`
}

type PostView struct {
	ID       string   `json:"id"`
	AuthorID string   `json:"author_id"`
	Author   string   `json:"author"`
	Text     string   `json:"text"`
	Tags     []string `json:"tags"`
	Unix     int64    `json:"unix"`
	Mine     bool     `json:"mine"`
}

type CatalogView struct {
	Kind        string   `json:"kind"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Domain      string   `json:"domain"`
	URL         string   `json:"url"`
	Tags        []string `json:"tags"`
	Hops        int      `json:"hops,omitempty"`
}

type NetworkView struct {
	NetworkID     string                 `json:"network_id"`
	Beacons       []string               `json:"beacons"`
	CircuitHops   int                    `json:"circuit_hops"`
	TransportMode string                 `json:"transport_mode"`
	DirectFirst   bool                   `json:"direct_first"`
	XraySOCKS     string                 `json:"xray_socks"`
	Peers         []overlay.PeerStatus   `json:"peers"`
	Routes        []overlay.RouteStatus  `json:"routes"`
	Attempts      []transportpkg.Attempt `json:"transport_attempts"`
}

type DeveloperView struct {
	NodeID         string                       `json:"node_id"`
	NodeDomain     string                       `json:"node_domain"`
	Listen         []string                     `json:"listen"`
	Services       []overlay.ServiceStatus      `json:"services"`
	KnownServices  []overlay.KnownServiceStatus `json:"known_services"`
	Aliases        []overlay.AliasStatus        `json:"aliases"`
	Forwards       []overlay.ForwardStatus      `json:"forwards"`
	Events         []overlay.Event              `json:"events"`
	ActiveStreams  int                          `json:"active_streams"`
	ActiveCircuits int                          `json:"active_circuits"`
	Descriptors    int                          `json:"descriptors"`
}

type CAView struct {
	Enabled            bool     `json:"enabled"`
	InterceptHTTPS     bool     `json:"intercept_https"`
	ValidityDays       int      `json:"validity_days"`
	CommonName         string   `json:"common_name"`
	Organization       []string `json:"organization"`
	OrganizationalUnit []string `json:"organizational_unit"`
	Country            []string `json:"country"`
	Province           []string `json:"province"`
	Locality           []string `json:"locality"`
	StreetAddress      []string `json:"street_address"`
	PostalCode         []string `json:"postal_code"`
	Fingerprint        string   `json:"fingerprint,omitempty"`
	Subject            string   `json:"subject,omitempty"`
	Issuer             string   `json:"issuer,omitempty"`
	NotAfter           string   `json:"not_after,omitempty"`
}

type PathView struct {
	Data         string `json:"data"`
	Config       string `json:"config"`
	Identity     string `json:"identity"`
	UserIdentity string `json:"user_identity"`
	Social       string `json:"social"`
	CA           string `json:"ca"`
}

type NetworkInput struct {
	NetworkID     string   `json:"network_id"`
	Beacons       []string `json:"beacons"`
	CircuitHops   int      `json:"circuit_hops"`
	TransportMode string   `json:"transport_mode"`
	DirectFirst   bool     `json:"direct_first"`
	XraySOCKS     string   `json:"xray_socks"`
}

type CAInput struct {
	Enabled            bool     `json:"enabled"`
	InterceptHTTPS     bool     `json:"intercept_https"`
	ValidityDays       int      `json:"validity_days"`
	CommonName         string   `json:"common_name"`
	Organization       []string `json:"organization"`
	OrganizationalUnit []string `json:"organizational_unit"`
	Country            []string `json:"country"`
	Province           []string `json:"province"`
	Locality           []string `json:"locality"`
	StreetAddress      []string `json:"street_address"`
	PostalCode         []string `json:"postal_code"`
}

func newDesktopApp(rt *clientruntime.Runtime, dataDir string) *App {
	a := &App{rt: rt, dataDir: dataDir, integration: browserIntegrationEnabled(dataDir), startupOn: startupEnabled()}
	if info, err := rt.CAInfo(); err == nil {
		a.caInfo = &info
	}
	return a
}

func (a *App) startup(ctx context.Context) {
	nodeCtx, nodeCancel := context.WithCancel(ctx)
	a.mu.Lock()
	a.uiCtx = ctx
	a.nodeCtx = nodeCtx
	a.nodeCancel = nodeCancel
	a.operation = "Запускаем защищённую сеть…"
	a.mu.Unlock()
	go func(runCtx context.Context) {
		if err := a.rt.Start(runCtx); err != nil {
			a.setOperation("", err)
		} else {
			a.setOperation("", nil)
		}
		a.emitSnapshot(true)
	}(nodeCtx)
}

func (a *App) domReady(ctx context.Context) {
	a.mu.Lock()
	a.uiCtx = ctx
	if a.ready {
		a.mu.Unlock()
		return
	}
	a.ready = true
	a.mu.Unlock()
	a.emitSnapshot(true)
	go a.eventLoop(ctx)
}

func (a *App) shutdown(context.Context) {
	a.mu.Lock()
	cancel := a.nodeCancel
	a.nodeCancel = nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	a.rt.Stop()
}

func (a *App) eventLoop(ctx context.Context) {
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.emitSnapshot(false)
		}
	}
}

func (a *App) setOperation(operation string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.operation = operation
	if err == nil {
		a.operationErr = ""
	} else {
		a.operationErr = err.Error()
	}
}

func (a *App) emitSnapshot(force bool) {
	snap := a.snapshot()
	raw, _ := json.Marshal(snap)
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	a.mu.Lock()
	if !force && hash == a.lastStateHash {
		a.mu.Unlock()
		return
	}
	a.lastStateHash = hash
	ctx := a.nodeCtx
	a.mu.Unlock()
	if ctx != nil {
		wailsruntime.EventsEmit(ctx, "knotroute:snapshot", snap)
	}
}

func (a *App) GetSnapshot() DesktopSnapshot { return a.snapshot() }

func (a *App) snapshot() DesktopSnapshot {
	cfg := a.rt.Config()
	socialState := a.rt.SocialState()
	running := a.rt.Running()
	st, hasStatus := a.rt.Status()
	profile := ProfileView{ID: a.rt.UserID(), DisplayName: socialState.DisplayName, Bio: socialState.Bio}

	contacts := make([]ContactView, 0, len(socialState.Contacts))
	for userID, c := range socialState.Contacts {
		cv := ContactView{ID: userID, Node: c.Node, Alias: c.Alias, DisplayName: c.Profile.DisplayName, Bio: c.Profile.Bio, Messages: []MessageView{}}
		for _, m := range socialState.Messages[userID] {
			mv := MessageView{ID: m.ID, Mine: m.Sender.ID == a.rt.UserID(), Body: m.Body, Unix: m.CreatedUnix, SenderID: m.Sender.ID}
			cv.Messages = append(cv.Messages, mv)
			if m.CreatedUnix >= cv.LastUnix {
				cv.LastUnix, cv.LastMessage = m.CreatedUnix, m.Body
			}
		}
		contacts = append(contacts, cv)
	}
	sort.Slice(contacts, func(i, j int) bool {
		if contacts[i].LastUnix == contacts[j].LastUnix {
			return strings.ToLower(contacts[i].DisplayName) < strings.ToLower(contacts[j].DisplayName)
		}
		return contacts[i].LastUnix > contacts[j].LastUnix
	})

	posts := make([]PostView, 0, len(socialState.Posts))
	for _, p := range socialState.Posts {
		posts = append(posts, PostView{ID: p.ID, AuthorID: p.Author.ID, Author: p.Author.DisplayName, Text: p.Text, Tags: append([]string(nil), p.Tags...), Unix: p.CreatedUnix, Mine: p.Author.ID == a.rt.UserID()})
	}

	catalog := []CatalogView{}
	network := NetworkView{NetworkID: cfg.NetworkID, Beacons: append([]string(nil), cfg.Discovery.Beacons...), CircuitHops: cfg.Privacy.CircuitHops, TransportMode: cfg.Transport.Mode, DirectFirst: cfg.Transport.DirectFirst, Peers: []overlay.PeerStatus{}, Routes: []overlay.RouteStatus{}}
	for _, ep := range cfg.Transport.Endpoints {
		if ep.Enabled && ep.Type == "socks5" {
			network.XraySOCKS = ep.Endpoint
			break
		}
	}
	developer := DeveloperView{NodeDomain: a.rt.NodeDomain(), Listen: []string{}, Services: []overlay.ServiceStatus{}, KnownServices: []overlay.KnownServiceStatus{}, Aliases: []overlay.AliasStatus{}, Forwards: []overlay.ForwardStatus{}, Events: []overlay.Event{}}
	overview := OverviewView{State: "offline", StateDetail: "Сеть остановлена", Transport: cfg.Transport.Mode, NodeDomain: a.rt.NodeDomain(), NetworkID: cfg.NetworkID}
	if running {
		overview.State = "starting"
		overview.StateDetail = "Узел запущен · ищем соседей"
	}
	if hasStatus {
		network.Peers = append([]overlay.PeerStatus(nil), st.Peers...)
		network.Routes = append([]overlay.RouteStatus(nil), st.Routes...)
		network.Attempts = st.Transport.Attempts
		developer.NodeID, developer.NodeDomain = st.NodeID, st.Domain
		developer.Listen = append([]string(nil), st.Listen...)
		developer.Services = append([]overlay.ServiceStatus(nil), st.Services...)
		developer.KnownServices = append([]overlay.KnownServiceStatus(nil), st.KnownServices...)
		developer.Aliases = append([]overlay.AliasStatus(nil), st.Aliases...)
		developer.Forwards = append([]overlay.ForwardStatus(nil), st.Forwards...)
		developer.Events = append([]overlay.Event(nil), st.Events...)
		developer.ActiveStreams, developer.ActiveCircuits, developer.Descriptors = st.ActiveStreams, st.ActiveCircuits, st.Descriptors
		overview.PeerCount, overview.RouteCount, overview.ServiceCount = len(st.Peers), len(st.Routes), len(st.KnownServices)+len(st.Services)
		overview.BytesSent, overview.BytesRecv = st.BytesSent, st.BytesReceived
		overview.Transport = st.Transport.LastSelected
		if overview.Transport == "" {
			overview.Transport = st.Transport.Mode
		}
		if len(st.Peers) > 0 {
			overview.State = "online"
			overview.StateDetail = fmt.Sprintf("Связь работает · %d соседей · %d маршрутов", len(st.Peers), len(st.Routes))
		}
		catalog = buildCatalog(st)
	}

	ca := CAView{Enabled: cfg.CA.Enabled, InterceptHTTPS: cfg.CA.InterceptHTTPS, ValidityDays: cfg.CA.ValidityDays, CommonName: cfg.CA.Subject.CommonName, Organization: cloneStrings(cfg.CA.Subject.Organization), OrganizationalUnit: cloneStrings(cfg.CA.Subject.OrganizationalUnit), Country: cloneStrings(cfg.CA.Subject.Country), Province: cloneStrings(cfg.CA.Subject.Province), Locality: cloneStrings(cfg.CA.Subject.Locality), StreetAddress: cloneStrings(cfg.CA.Subject.StreetAddress), PostalCode: cloneStrings(cfg.CA.Subject.PostalCode)}
	a.mu.RLock()
	info := a.caInfo
	a.mu.RUnlock()
	if info != nil {
		ca.Fingerprint, ca.Subject, ca.Issuer, ca.NotAfter = info.Fingerprint, info.Subject, info.Issuer, info.NotAfter.Format(time.RFC3339)
	}
	a.mu.RLock()
	operation, operationErr := a.operation, a.operationErr
	integration, startupOn := a.integration, a.startupOn
	a.mu.RUnlock()
	if operationErr == "" {
		operationErr = a.rt.LastError()
	}
	return DesktopSnapshot{
		Version: overlay.Version, Running: running, Operation: operation, Error: operationErr,
		Integration: integration, Startup: startupOn, Overview: overview, Profile: profile, Contacts: contacts, Posts: posts, Catalog: catalog, Network: network, Developer: developer, CA: ca,
		Paths: PathView{Data: a.dataDir, Config: a.rt.ConfigPath(), Identity: cfg.IdentityFile, UserIdentity: filepath.Join(a.dataDir, "user-identity.json"), Social: filepath.Join(a.dataDir, "social.json"), CA: cfg.CA.Directory},
	}
}

func buildCatalog(st overlay.Status) []CatalogView {
	out := make([]CatalogView, 0)
	seen := map[string]bool{}
	for _, route := range st.Routes {
		for _, service := range route.Services {
			domain := service + "." + route.Domain
			if seen[domain] {
				continue
			}
			seen[domain] = true
			out = append(out, CatalogView{Kind: "node", Title: service, Description: "Сервис узла " + route.ShortID, Domain: domain, URL: "https://" + domain + "/", Tags: []string{"node-service"}, Hops: route.Hops})
		}
	}
	for _, svc := range st.KnownServices {
		if svc.Domain == "" || seen[svc.Domain] {
			continue
		}
		seen[svc.Domain] = true
		title := svc.Metadata["title"]
		if title == "" {
			title = svc.Metadata["name"]
		}
		if title == "" {
			title = svc.Domain
		}
		scheme := strings.ToLower(strings.TrimSpace(svc.Metadata["scheme"]))
		if scheme != "http" {
			scheme = "https"
		}
		tags := splitCSV(svc.Metadata["tags"])
		description := svc.Metadata["description"]
		if description == "" {
			description = "Переносимая identity сервиса"
		}
		out = append(out, CatalogView{Kind: "identity", Title: title, Description: description, Domain: svc.Domain, URL: scheme + "://" + svc.Domain + "/", Tags: tags})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title) })
	return out
}

func (a *App) ToggleNetwork() DesktopSnapshot {
	a.setOperation("Меняем состояние сети…", nil)
	if a.rt.Running() {
		a.rt.Stop()
		a.setOperation("", nil)
	} else {
		a.mu.RLock()
		ctx := a.nodeCtx
		a.mu.RUnlock()
		if ctx == nil {
			ctx = context.Background()
		}
		if err := a.rt.Start(ctx); err != nil {
			a.setOperation("", err)
		} else {
			a.setOperation("", nil)
		}
	}
	a.emitSnapshot(true)
	return a.snapshot()
}

func (a *App) SetBrowserIntegration(enabled bool) (DesktopSnapshot, error) {
	a.setOperation("Настраиваем .knot в браузерах…", nil)
	var err error
	if enabled {
		err = enableBrowserIntegration(a.rt, a.dataDir)
	} else {
		err = disableBrowserIntegration(a.rt, a.dataDir)
	}
	if err == nil {
		a.mu.Lock()
		a.integration = enabled
		a.mu.Unlock()
	}
	a.setOperation("", err)
	a.emitSnapshot(true)
	return a.snapshot(), err
}

func (a *App) SaveProfile(displayName, bio string) (DesktopSnapshot, error) {
	err := a.rt.SetUserProfile(displayName, bio)
	a.setOperation("", err)
	a.emitSnapshot(true)
	return a.snapshot(), err
}

func (a *App) AddContact(nodeReference, alias string) (DesktopSnapshot, error) {
	ctx, cancel := context.WithTimeout(a.context(), 25*time.Second)
	defer cancel()
	a.setOperation("Получаем профиль контакта…", nil)
	_, err := a.rt.AddContact(ctx, nodeReference, alias)
	a.setOperation("", err)
	a.emitSnapshot(true)
	return a.snapshot(), err
}

func (a *App) SendMessage(userID, body string) (DesktopSnapshot, error) {
	ctx, cancel := context.WithTimeout(a.context(), 30*time.Second)
	defer cancel()
	a.setOperation("Отправляем сообщение…", nil)
	_, err := a.rt.SendMessage(ctx, userID, body)
	a.setOperation("", err)
	a.emitSnapshot(true)
	return a.snapshot(), err
}

func (a *App) CreatePost(text string, tags []string) (DesktopSnapshot, error) {
	_, err := a.rt.CreatePost(text, tags)
	a.setOperation("", err)
	a.emitSnapshot(true)
	return a.snapshot(), err
}

func (a *App) RefreshContactFeed(userID string) (DesktopSnapshot, error) {
	ctx, cancel := context.WithTimeout(a.context(), 30*time.Second)
	defer cancel()
	a.setOperation("Обновляем ленту…", nil)
	_, err := a.rt.FetchContactFeed(ctx, userID)
	a.setOperation("", err)
	a.emitSnapshot(true)
	return a.snapshot(), err
}

func (a *App) SaveNetwork(in NetworkInput) (DesktopSnapshot, error) {
	cfg := a.rt.Config()
	cfg.NetworkID = strings.TrimSpace(in.NetworkID)
	cfg.Discovery.Beacons = cleanStrings(in.Beacons)
	if in.CircuitHops > 0 {
		cfg.Privacy.CircuitHops = in.CircuitHops
	}
	mode := strings.ToLower(strings.TrimSpace(in.TransportMode))
	if mode != "direct" && mode != "proxy" {
		mode = "auto"
	}
	cfg.Transport.Mode = mode
	cfg.Transport.DirectFirst = in.DirectFirst
	cfg.Transport.FallbackDirect = mode != "proxy"
	cfg.Transport.Endpoints = nil
	if socks := strings.TrimSpace(in.XraySOCKS); socks != "" {
		cfg.Transport.Endpoints = []config.TransportEndpoint{{Name: "xray", Type: "socks5", Endpoint: socks, Enabled: true, Priority: 10}}
	}
	a.setOperation("Сохраняем сеть и перезапускаем runtime…", nil)
	err := a.rt.SaveConfig(cfg)
	if err == nil {
		err = a.rt.Restart(a.nodeContext())
	}
	a.setOperation("", err)
	a.emitSnapshot(true)
	return a.snapshot(), err
}

func (a *App) SaveCA(in CAInput) (DesktopSnapshot, error) {
	cfg := a.rt.Config()
	cfg.CA.Enabled, cfg.CA.InterceptHTTPS, cfg.CA.ValidityDays = in.Enabled, in.InterceptHTTPS, in.ValidityDays
	cfg.CA.Subject = config.CertificateSubject{CommonName: strings.TrimSpace(in.CommonName), Organization: cleanStrings(in.Organization), OrganizationalUnit: cleanStrings(in.OrganizationalUnit), Country: cleanStrings(in.Country), Province: cleanStrings(in.Province), Locality: cleanStrings(in.Locality), StreetAddress: cleanStrings(in.StreetAddress), PostalCode: cleanStrings(in.PostalCode)}
	err := a.rt.SaveConfig(cfg)
	a.setOperation("", err)
	a.emitSnapshot(true)
	return a.snapshot(), err
}

func (a *App) RotateCA(confirmed bool) (DesktopSnapshot, error) {
	if !confirmed {
		return a.snapshot(), errors.New("rotation confirmation is required")
	}
	a.setOperation("Перевыпускаем Root CA…", nil)
	info, err := a.rt.RotateCA()
	if err == nil {
		a.mu.Lock()
		a.caInfo = &info
		a.mu.Unlock()
		err = a.rt.InstallCA()
	}
	if err == nil {
		err = a.rt.Restart(a.nodeContext())
	}
	a.setOperation("", err)
	a.emitSnapshot(true)
	return a.snapshot(), err
}

func (a *App) InstallCA() (DesktopSnapshot, error) {
	err := a.rt.InstallCA()
	a.setOperation("", err)
	a.emitSnapshot(true)
	return a.snapshot(), err
}

func (a *App) ExportCA() (string, error) {
	pem, err := a.rt.RootCAPEM()
	if err != nil {
		return "", err
	}
	path, err := wailsruntime.SaveFileDialog(a.context(), wailsruntime.SaveDialogOptions{Title: "Экспорт KnotRoute Root CA", DefaultFilename: "knotroute-root-ca.crt", Filters: []wailsruntime.FileFilter{{DisplayName: "Certificate (*.crt)", Pattern: "*.crt"}, {DisplayName: "PEM (*.pem)", Pattern: "*.pem"}}})
	if err != nil || path == "" {
		return path, err
	}
	return path, os.WriteFile(path, pem, 0o644)
}

func (a *App) SetStartup(enabled bool) (DesktopSnapshot, error) {
	err := setStartup(enabled)
	if err == nil {
		a.mu.Lock()
		a.startupOn = enabled
		a.mu.Unlock()
	}
	a.setOperation("", err)
	a.emitSnapshot(true)
	return a.snapshot(), err
}

func (a *App) OpenURL(url string) error   { wailsruntime.BrowserOpenURL(a.context(), url); return nil }
func (a *App) OpenDataDir() error         { return shellOpenPath(a.dataDir) }
func (a *App) CopyText(text string) error { return wailsruntime.ClipboardSetText(a.context(), text) }

func (a *App) WriteDiagnostics() (string, error) {
	var b strings.Builder
	snap := a.snapshot()
	fmt.Fprintf(&b, "KnotRoute v4 desktop diagnostics\r\ncreated: %s\r\nversion: %s\r\n\r\n", time.Now().Format(time.RFC3339), snap.Version)
	raw, _ := json.MarshalIndent(snap, "", "  ")
	b.Write(raw)
	b.WriteString("\r\n")
	path := filepath.Join(a.dataDir, "diagnostics-"+time.Now().Format("20060102-150405")+".txt")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return "", err
	}
	return path, shellOpenPath(path)
}

func (a *App) context() context.Context {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.uiCtx != nil {
		return a.uiCtx
	}
	return context.Background()
}

func (a *App) nodeContext() context.Context {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.nodeCtx != nil {
		return a.nodeCtx
	}
	return context.Background()
}
func cloneStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return append([]string(nil), v...)
}
func cleanStrings(v []string) []string {
	out := make([]string, 0, len(v))
	for _, s := range v {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
func splitCSV(v string) []string { return cleanStrings(strings.Split(v, ",")) }
