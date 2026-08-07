package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/localzet/knotroute/internal/ops"
	"github.com/localzet/knotroute/internal/ops/controlweb"
)

const sessionTTL = 12 * time.Hour

type server struct {
	store       *ops.Store
	admin       string
	enrollToken string
	publicURL   string
	sessionsMu  sync.Mutex
	sessions    map[string]time.Time
	loginMu     sync.Mutex
	loginFails  map[string][]time.Time
}

type loginRequest struct {
	Password string `json:"password"`
}
type networkRequest struct {
	ID, Name, Description string
	Beacons               []string `json:"beacons"`
}
type jobRequest struct {
	AgentID string         `json:"agent_id"`
	Kind    string         `json:"kind"`
	Payload map[string]any `json:"payload"`
}
type onboardingRequest struct {
	NetworkID string `json:"network_id"`
	Platform  string `json:"platform"`
	Name      string `json:"name"`
	Language  string `json:"language"`
}
type onboardingResponse struct {
	Title        string `json:"title"`
	Instructions string `json:"instructions"`
	URI          string `json:"uri,omitempty"`
}

func main() {
	listen := env("KNOTROUTE_CONTROL_LISTEN", "0.0.0.0:8080")
	dataDir := env("KNOTROUTE_CONTROL_DATA", "/data")
	admin := strings.TrimSpace(os.Getenv("KNOTROUTE_CONTROL_ADMIN_PASSWORD"))
	enroll := strings.TrimSpace(os.Getenv("KNOTROUTE_CONTROL_ENROLL_TOKEN"))
	if admin == "" || enroll == "" {
		log.Fatal("KNOTROUTE_CONTROL_ADMIN_PASSWORD and KNOTROUTE_CONTROL_ENROLL_TOKEN are required")
	}
	store, err := ops.OpenStore(filepath.Join(dataDir, "state.json"))
	if err != nil {
		log.Fatal(err)
	}
	s := &server{store: store, admin: admin, enrollToken: enroll, publicURL: strings.TrimRight(os.Getenv("KNOTROUTE_CONTROL_PUBLIC_URL"), "/"), sessions: map[string]time.Time{}, loginFails: map[string][]time.Time{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, http.StatusOK, map[string]any{"ok": true, "version": ops.Version})
	})
	mux.HandleFunc("POST /api/v1/agents/enroll", s.enroll)
	mux.HandleFunc("POST /api/v1/agents/heartbeat", s.agentSigned(s.heartbeat))
	mux.HandleFunc("POST /api/v1/agents/jobs/next", s.agentSigned(s.nextJob))
	mux.HandleFunc("POST /api/v1/agents/jobs/result", s.agentSigned(s.jobResult))
	mux.HandleFunc("GET /api/v1/session", s.adminOnly(s.sessionStatus))
	mux.HandleFunc("POST /api/v1/session", s.login)
	mux.HandleFunc("DELETE /api/v1/session", s.adminOnly(s.logout))
	mux.HandleFunc("GET /api/v1/overview", s.adminOnly(s.overview))
	mux.HandleFunc("POST /api/v1/networks", s.adminOnly(s.saveNetwork))
	mux.HandleFunc("POST /api/v1/jobs", s.adminOnly(s.createJob))
	mux.HandleFunc("POST /api/v1/onboarding/render", s.adminOnly(s.renderOnboarding))
	mux.HandleFunc("GET /api/v1/onboarding/qr", s.adminOnly(s.qr))
	mux.HandleFunc("GET /assets/", s.asset)
	mux.HandleFunc("GET /", s.index)
	h := securityHeaders(limitBody(mux, 2<<20))
	httpServer := &http.Server{Addr: listen, Handler: h, ReadHeaderTimeout: 7 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 1 << 20}
	log.Printf("KnotRoute Control %s listening on %s", ops.Version, listen)
	log.Fatal(httpServer.ListenAndServe())
}

func (s *server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	raw, _ := controlweb.FS.ReadFile("index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(raw)
}
func (s *server) asset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/assets/")
	if name != "app.js" && name != "style.css" {
		http.NotFound(w, r)
		return
	}
	raw, err := controlweb.FS.ReadFile(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(name)))
	w.Header().Set("Cache-Control", "public,max-age=3600")
	_, _ = w.Write(raw)
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.loginBlocked(ip) {
		jsonError(w, http.StatusTooManyRequests, "too many login attempts")
		return
	}
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.admin)) != 1 {
		s.recordLoginFailure(ip)
		jsonError(w, 401, "invalid credentials")
		return
	}
	token := randomToken(32)
	s.sessionsMu.Lock()
	s.sessions[token] = time.Now().Add(sessionTTL)
	s.sessionsMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "kr_control_session", Value: token, Path: "/", HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteStrictMode, MaxAge: int(sessionTTL.Seconds())})
	jsonOut(w, 200, map[string]any{"ok": true})
}
func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("kr_control_session"); err == nil {
		s.sessionsMu.Lock()
		delete(s.sessions, c.Value)
		s.sessionsMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "kr_control_session", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	jsonOut(w, 200, map[string]any{"ok": true})
}
func (s *server) sessionStatus(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, 200, map[string]any{"ok": true, "version": ops.Version})
}
func (s *server) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.validSession(r) {
			jsonError(w, 401, "authentication required")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && !sameOrigin(r) {
			jsonError(w, 403, "cross-origin management request rejected")
			return
		}
		next(w, r)
	}
}
func (s *server) validSession(r *http.Request) bool {
	c, err := r.Cookie("kr_control_session")
	if err != nil {
		return false
	}
	now := time.Now()
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	exp, ok := s.sessions[c.Value]
	if !ok || now.After(exp) {
		delete(s.sessions, c.Value)
		return false
	}
	return true
}

func (s *server) overview(w http.ResponseWriter, r *http.Request) {
	var snapshot ops.State
	s.store.View(func(st ops.State) { snapshot = st })
	jsonOut(w, 200, snapshot)
}
func (s *server) saveNetwork(w http.ResponseWriter, r *http.Request) {
	var req networkRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	req.Name = strings.TrimSpace(req.Name)
	if req.ID == "" {
		req.ID = ops.NewNetworkID()
	}
	if !validNetworkID(req.ID) || req.Name == "" || len(req.Name) > 80 {
		jsonError(w, 400, "invalid network id or name")
		return
	}
	beacons := make([]string, 0, len(req.Beacons))
	for _, b := range req.Beacons {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		u, err := url.Parse(b)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			jsonError(w, 400, "beacon URLs must be HTTPS URLs")
			return
		}
		beacons = append(beacons, strings.TrimRight(b, "/"))
	}
	now := time.Now().UTC()
	err := s.store.Update(func(st *ops.State) error {
		old, ok := st.Networks[req.ID]
		created := now
		if ok {
			created = old.CreatedAt
		}
		st.Networks[req.ID] = ops.Network{ID: req.ID, Name: req.Name, Description: strings.TrimSpace(req.Description), Beacons: beacons, CreatedAt: created, UpdatedAt: now}
		return nil
	})
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOut(w, 200, map[string]any{"ok": true, "network_id": req.ID})
}
func (s *server) createJob(w http.ResponseWriter, r *http.Request) {
	var req jobRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	if !allowedJob(req.Kind) {
		jsonError(w, 400, "unsupported job kind")
		return
	}
	var job ops.Job
	err := s.store.Update(func(st *ops.State) error {
		if _, ok := st.Agents[req.AgentID]; !ok {
			return errors.New("unknown agent")
		}
		job = ops.Job{ID: ops.NewID("job"), AgentID: req.AgentID, Kind: req.Kind, Payload: req.Payload, Status: "pending", CreatedAt: time.Now().UTC()}
		st.Jobs[job.ID] = job
		return nil
	})
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	jsonOut(w, 201, job)
}
func (s *server) renderOnboarding(w http.ResponseWriter, r *http.Request) {
	var req onboardingRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	var n ops.Network
	found := false
	s.store.View(func(st ops.State) { n, found = st.Networks[req.NetworkID] })
	if !found {
		jsonError(w, 404, "network not found")
		return
	}
	resp, err := buildOnboarding(n, req.Platform, req.Name, req.Language)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, resp)
}
func (s *server) qr(w http.ResponseWriter, r *http.Request) {
	payload := r.URL.Query().Get("payload")
	if payload == "" || len(payload) > 4096 {
		jsonError(w, 400, "invalid QR payload")
		return
	}
	cmd := exec.Command("qrencode", "-t", "SVG", "-o", "-", "-m", "2", "-s", "5", payload)
	raw, err := cmd.Output()
	if err != nil {
		jsonError(w, 501, "qrencode is not installed in this Control build")
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(raw)
}

func (s *server) enroll(w http.ResponseWriter, r *http.Request) {
	var req ops.EnrollRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.Token), []byte(s.enrollToken)) != 1 {
		jsonError(w, 401, "invalid enrollment token")
		return
	}
	pubRaw, err := base64.RawStdEncoding.DecodeString(req.PublicKey)
	if err != nil || len(pubRaw) != ed25519.PublicKeySize {
		jsonError(w, 400, "invalid agent public key")
		return
	}
	id := ops.AgentID(ed25519.PublicKey(pubRaw))
	now := time.Now().UTC()
	err = s.store.Update(func(st *ops.State) error {
		a := st.Agents[id]
		if a.ID != "" && a.PublicKey != req.PublicKey {
			return errors.New("agent id collision")
		}
		if a.EnrolledAt.IsZero() {
			a.EnrolledAt = now
		}
		a.ID = id
		a.PublicKey = req.PublicKey
		a.Name = cleanName(req.Name)
		a.Tags = cleanTags(req.Tags)
		a.LastSeen = now
		st.Agents[id] = a
		return nil
	})
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOut(w, 201, ops.EnrollResponse{AgentID: id})
}
func (s *server) agentSigned(next func(http.ResponseWriter, *http.Request, string, []byte)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			jsonError(w, 400, "read request")
			return
		}
		id := r.Header.Get("X-Knot-Agent")
		stamp, err := strconv.ParseInt(r.Header.Get("X-Knot-Timestamp"), 10, 64)
		if err != nil {
			jsonError(w, 401, "invalid timestamp")
			return
		}
		sig := r.Header.Get("X-Knot-Signature")
		var a ops.Agent
		found := false
		s.store.View(func(st ops.State) { a, found = st.Agents[id] })
		if !found {
			jsonError(w, 401, "unknown agent")
			return
		}
		pubRaw, err := base64.RawStdEncoding.DecodeString(a.PublicKey)
		if err != nil || ops.VerifyRequest(ed25519.PublicKey(pubRaw), r.Method, r.URL.Path, stamp, body, sig, time.Now()) != nil {
			jsonError(w, 401, "invalid agent signature")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		next(w, r, id, body)
	}
}
func (s *server) heartbeat(w http.ResponseWriter, r *http.Request, id string, body []byte) {
	var hb ops.Heartbeat
	if err := json.Unmarshal(body, &hb); err != nil {
		jsonError(w, 400, "invalid heartbeat")
		return
	}
	now := time.Now().UTC()
	err := s.store.Update(func(st *ops.State) error {
		a := st.Agents[id]
		a.Name = cleanName(hb.Name)
		a.Hostname = cleanName(hb.Hostname)
		a.Version = trim(hb.Version, 64)
		a.DockerAvailable = hb.DockerAvailable
		a.DockerVersion = trim(hb.DockerVersion, 120)
		a.Tags = cleanTags(hb.Tags)
		a.Components = sanitizeComponents(hb.Components, now)
		a.LastSeen = now
		st.Agents[id] = a
		return nil
	})
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOut(w, 200, map[string]any{"ok": true})
}
func (s *server) nextJob(w http.ResponseWriter, r *http.Request, id string, body []byte) {
	var selected *ops.Job
	now := time.Now().UTC()
	err := s.store.Update(func(st *ops.State) error {
		ids := make([]string, 0)
		for jid, j := range st.Jobs {
			if j.AgentID == id && j.Status == "pending" {
				ids = append(ids, jid)
			}
		}
		sort.Slice(ids, func(i, j int) bool { return st.Jobs[ids[i]].CreatedAt.Before(st.Jobs[ids[j]].CreatedAt) })
		if len(ids) == 0 {
			return nil
		}
		j := st.Jobs[ids[0]]
		j.Status = "running"
		j.StartedAt = &now
		st.Jobs[j.ID] = j
		selected = &j
		return nil
	})
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	if selected == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	jsonOut(w, 200, *selected)
}
func (s *server) jobResult(w http.ResponseWriter, r *http.Request, id string, body []byte) {
	var result ops.JobResult
	if err := json.Unmarshal(body, &result); err != nil {
		jsonError(w, 400, "invalid result")
		return
	}
	if result.Status != "succeeded" && result.Status != "failed" {
		jsonError(w, 400, "invalid result status")
		return
	}
	now := time.Now().UTC()
	err := s.store.Update(func(st *ops.State) error {
		j, ok := st.Jobs[result.JobID]
		if !ok || j.AgentID != id {
			return errors.New("unknown job")
		}
		j.Status = result.Status
		j.Result = trim(result.Result, 32<<10)
		j.EndedAt = &now
		st.Jobs[j.ID] = j
		return nil
	})
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]any{"ok": true})
}

func buildOnboarding(n ops.Network, platform, name, language string) (onboardingResponse, error) {
	if name == "" {
		name = n.Name
	}
	q := url.Values{}
	q.Set("network_id", n.ID)
	q.Set("name", name)
	for _, b := range n.Beacons {
		q.Add("beacon", b)
	}
	uri := "knotroute://join?" + q.Encode()
	beacons := strings.Join(n.Beacons, "\n")
	var text string
	ru := strings.EqualFold(language, "ru")
	if ru {
		switch platform {
		case "windows":
			text = fmt.Sprintf("1. Установите KnotRoute для Windows.\n2. Запустите knotroute-desktop.exe.\n3. В настройках сети укажите:\n\nNetwork ID:\n%s\n\nBeacon URL:\n%s\n\n4. В tray включите .knot integration.\n5. Откройте .knot-адрес в браузере.\n\nQR/profile URI содержит только параметры подключения и никогда не содержит приватные ключи.", n.ID, beacons)
		case "android":
			text = fmt.Sprintf("1. Установите Android-клиент KnotRoute.\n2. Отсканируйте QR системной камерой или откройте profile URI.\n3. Подтвердите добавление сети '%s'.\n4. Для HTTPS .knot-сайтов установите локальный CA через системный Android installer.\n5. Используйте встроенный браузер.\n\nNetwork ID: %s\nBeacon:\n%s", name, n.ID, beacons)
		case "linux":
			text = fmt.Sprintf("Создайте конфигурацию KnotRoute, установите network_id = %s и добавьте Beacon:\n%s\n\nЗатем:\n  knotroute doctor --config knotroute.json --probe\n  knotroute run --config knotroute.json\n\nДля приложений используйте SOCKS5/HTTP proxy или встроенный Go SDK.", n.ID, beacons)
		case "docker":
			text = fmt.Sprintf("Используйте KnotRoute sidecar и обязательно сохраняйте /data.\n\nKNOTROUTE_NETWORK_ID=%s\nKNOTROUTE_BEACONS=%s\n\nПодключите sidecar к Docker network, где находится target-сервис. Не публикуйте приватный порт приложения, если он не должен быть доступен из обычного Интернета.", n.ID, strings.Join(n.Beacons, ","))
		case "go":
			text = fmt.Sprintf("Встройте pkg/knotclient и задайте:\nNetworkID: %q\nBeacons: %#v\n\nПосле этого приложение сможет подключаться к canonical .knot service identity без отдельно установленного desktop-клиента.", n.ID, n.Beacons)
		default:
			return onboardingResponse{}, errors.New("unsupported platform")
		}
	} else {
		switch platform {
		case "windows":
			text = fmt.Sprintf("1. Install KnotRoute for Windows.\n2. Start knotroute-desktop.exe.\n3. Open Network settings and set:\n\nNetwork ID:\n%s\n\nBeacon URLs:\n%s\n\n4. Enable .knot system integration in the tray menu.\n5. Open a .knot address in your browser.\n\nThe QR/profile URI contains connection metadata only, never private keys.", n.ID, beacons)
		case "android":
			text = fmt.Sprintf("1. Install the KnotRoute Android client.\n2. Scan the QR code with the system camera or open the profile URI.\n3. Confirm network '%s'.\n4. For HTTPS .knot sites, install the local CA through Android's system certificate installer.\n5. Open the built-in browser.\n\nNetwork ID: %s\nBeacons:\n%s", name, n.ID, beacons)
		case "linux":
			text = fmt.Sprintf("Create a KnotRoute config, set network_id to %s and configure these discovery beacons:\n%s\n\nThen run:\n  knotroute doctor --config knotroute.json --probe\n  knotroute run --config knotroute.json\n\nFor applications use SOCKS5/HTTP proxy or embed the Go SDK.", n.ID, beacons)
		case "docker":
			text = fmt.Sprintf("Use the KnotRoute sidecar image and persist /data.\n\nKNOTROUTE_NETWORK_ID=%s\nKNOTROUTE_BEACONS=%s\n\nAttach the sidecar to the Docker network containing the target service. Do not publish the application's private port unless it is intentionally public.", n.ID, strings.Join(n.Beacons, ","))
		case "go":
			text = fmt.Sprintf("Embed pkg/knotclient and configure:\nNetworkID: %q\nBeacons: %#v\n\nThe application can then dial canonical .knot service identities without requiring the standalone desktop client.", n.ID, n.Beacons)
		default:
			return onboardingResponse{}, errors.New("unsupported platform")
		}
	}
	return onboardingResponse{Title: name + " · " + platform, Instructions: text, URI: uri}, nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}
func limitBody(next http.Handler, max int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, max)
		next.ServeHTTP(w, r)
	})
}
func decodeJSON(r *http.Request, out any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON value")
	}
	return nil
}
func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func jsonError(w http.ResponseWriter, status int, msg string) {
	jsonOut(w, status, map[string]string{"error": msg})
}
func env(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}
func randomToken(n int) string {
	raw := make([]byte, n)
	_, _ = rand.Read(raw)
	return base64.RawURLEncoding.EncodeToString(raw)
}
func validNetworkID(v string) bool {
	if !strings.HasPrefix(v, "kn_") || len(v) < 20 || len(v) > 90 {
		return false
	}
	for _, r := range strings.TrimPrefix(v, "kn_") {
		if !(r >= 'a' && r <= 'z') && !(r >= '2' && r <= '7') {
			return false
		}
	}
	return true
}
func allowedJob(v string) bool {
	switch v {
	case "deploy_beacon", "deploy_sidecar", "restart_component", "remove_component":
		return true
	}
	return false
}
func cleanName(v string) string { return trim(strings.TrimSpace(v), 120) }
func cleanTags(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = trim(strings.TrimSpace(v), 48)
		if v != "" {
			out = append(out, v)
		}
		if len(out) == 32 {
			break
		}
	}
	return out
}
func sanitizeComponents(in []ops.Component, now time.Time) []ops.Component {
	if len(in) > 512 {
		in = in[:512]
	}
	out := make([]ops.Component, 0, len(in))
	for _, c := range in {
		c.ID = trim(c.ID, 160)
		c.Kind = trim(c.Kind, 32)
		c.Name = trim(c.Name, 120)
		c.Container = trim(c.Container, 160)
		c.Image = trim(c.Image, 300)
		c.Status = trim(c.Status, 48)
		c.Version = trim(c.Version, 80)
		c.Address = trim(c.Address, 300)
		c.Service = trim(c.Service, 300)
		c.Target = trim(c.Target, 300)
		c.UpdatedAt = now
		if c.ID != "" {
			out = append(out, c)
		}
	}
	return out
}
func trim(v string, max int) string {
	if len(v) > max {
		return v[:max]
	}
	return v
}
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
func clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return host
}
func (s *server) loginBlocked(ip string) bool {
	now := time.Now()
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	items := s.loginFails[ip][:0]
	for _, t := range s.loginFails[ip] {
		if now.Sub(t) < 10*time.Minute {
			items = append(items, t)
		}
	}
	s.loginFails[ip] = items
	return len(items) >= 8
}
func (s *server) recordLoginFailure(ip string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	s.loginFails[ip] = append(s.loginFails[ip], time.Now())
}
