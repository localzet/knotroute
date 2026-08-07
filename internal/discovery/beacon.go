package discovery

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/networkid"
	"github.com/localzet/knotroute/internal/nodeid"
)

type Announcement struct {
	NetworkID string   `json:"network_id"`
	NodeID    string   `json:"node_id"`
	PublicKey string   `json:"public_key"`
	Endpoints []string `json:"endpoints"`
	TimeUnix  int64    `json:"time_unix"`
	Nonce     string   `json:"nonce,omitempty"`
	Signature string   `json:"signature"`
}

type Candidate struct {
	NodeID    string   `json:"node_id"`
	Endpoints []string `json:"endpoints"`
	SeenUnix  int64    `json:"seen_unix"`
}

type Response struct {
	Peers []Candidate `json:"peers"`
}

func SignAnnouncement(id *identity.Identity, network networkid.ID, endpoints []string, now time.Time) Announcement {
	a := Announcement{NetworkID: network.String(), NodeID: id.ID.String(), PublicKey: base64.StdEncoding.EncodeToString(id.PublicKey), Endpoints: normalizeEndpoints(endpoints), TimeUnix: now.Unix()}
	a.Signature = base64.StdEncoding.EncodeToString(id.Sign(a.message()))
	return a
}

func (a Announcement) Verify(now time.Time) (nodeid.ID, networkid.ID, error) {
	nid, err := nodeid.Parse(a.NodeID)
	if err != nil {
		return nodeid.ID{}, networkid.ID{}, err
	}
	network, err := networkid.Parse(a.NetworkID)
	if err != nil {
		return nodeid.ID{}, networkid.ID{}, err
	}
	if delta := now.Unix() - a.TimeUnix; delta > 180 || delta < -180 {
		return nodeid.ID{}, networkid.ID{}, errors.New("announcement timestamp outside allowed window")
	}
	pub, err := base64.StdEncoding.DecodeString(a.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nodeid.ID{}, networkid.ID{}, errors.New("invalid public key")
	}
	if nodeid.FromPublicKey(pub) != nid {
		return nodeid.ID{}, networkid.ID{}, errors.New("node id does not match public key")
	}
	sig, err := base64.StdEncoding.DecodeString(a.Signature)
	if err != nil || !identity.Verify(pub, a.message(), sig) {
		return nodeid.ID{}, networkid.ID{}, errors.New("invalid announcement signature")
	}
	if len(a.Endpoints) > 16 {
		return nodeid.ID{}, networkid.ID{}, errors.New("invalid endpoint count")
	}
	for _, ep := range a.Endpoints {
		if err := validateEndpoint(ep); err != nil {
			return nodeid.ID{}, networkid.ID{}, err
		}
	}
	return nid, network, nil
}

func (a Announcement) message() []byte {
	eps := normalizeEndpoints(a.Endpoints)
	return []byte("knotroute/beacon/v1|" + a.NetworkID + "|" + a.NodeID + "|" + a.PublicKey + "|" + strings.Join(eps, ",") + "|" + strconv.FormatInt(a.TimeUnix, 10))
}

func normalizeEndpoints(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
func validateEndpoint(ep string) error {
	host, port, err := net.SplitHostPort(ep)
	if err != nil || strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
		return fmt.Errorf("invalid endpoint %q", ep)
	}
	return nil
}

type record struct {
	candidate Candidate
	expires   time.Time
}
type bootstrapRecord struct {
	candidate    Candidate
	fallbackPort string
}

type rateRecord struct {
	tokens float64
	last   time.Time
}

type Server struct {
	mu            sync.Mutex
	ttl           time.Duration
	maxPerNetwork int
	records       map[string]map[string]record
	bootstrap     map[string]bootstrapRecord
	rate          map[string]rateRecord
	ratePerSecond float64
	rateBurst     float64
}

func NewServer(ttl time.Duration, max int) *Server {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	if max <= 0 {
		max = 10000
	}
	return &Server{
		ttl: ttl, maxPerNetwork: max, records: map[string]map[string]record{},
		bootstrap: map[string]bootstrapRecord{}, rate: map[string]rateRecord{},
		ratePerSecond: 2, rateBurst: 8,
	}
}

// SetBootstrap adds an optional always-returned relay candidate for one network.
// If endpoints is empty, the HTTP Host used by each caller is combined with
// fallbackPort. This lets a single Beacon container also act as a first relay.
func (s *Server) SetBootstrap(network networkid.ID, id nodeid.ID, endpoints []string, fallbackPort string) error {
	for _, ep := range endpoints {
		if err := validateEndpoint(ep); err != nil {
			return err
		}
	}
	if len(endpoints) == 0 {
		if _, err := strconv.Atoi(fallbackPort); err != nil {
			return fmt.Errorf("invalid bootstrap port %q", fallbackPort)
		}
	}
	s.mu.Lock()
	s.bootstrap[network.String()] = bootstrapRecord{candidate: Candidate{NodeID: id.String(), Endpoints: normalizeEndpoints(endpoints)}, fallbackPort: fallbackPort}
	s.mu.Unlock()
	return nil
}

func (s *Server) SetRateLimit(requestsPerSecond float64, burst int) {
	if requestsPerSecond <= 0 || burst <= 0 {
		return
	}
	s.mu.Lock()
	s.ratePerSecond = requestsPerSecond
	s.rateBurst = float64(burst)
	s.mu.Unlock()
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/v1/peers", s.handlePeers)
	return mux
}
func (s *Server) handlePeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.allowRequest(r.RemoteAddr, time.Now()) {
		w.Header().Set("Retry-After", "1")
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	defer r.Body.Close()
	var a Announcement
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	if err := dec.Decode(&a); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	nid, network, err := a.Verify(time.Now())
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	now := time.Now()
	key := network.String()
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket := s.records[key]
	if bucket == nil {
		bucket = map[string]record{}
		s.records[key] = bucket
	}
	for id, rec := range bucket {
		if now.After(rec.expires) {
			delete(bucket, id)
		}
	}
	if len(a.Endpoints) == 0 {
		delete(bucket, nid.String())
	} else {
		if _, exists := bucket[nid.String()]; !exists && len(bucket) >= s.maxPerNetwork {
			http.Error(w, "network capacity reached", 429)
			return
		}
		bucket[nid.String()] = record{candidate: Candidate{NodeID: nid.String(), Endpoints: normalizeEndpoints(a.Endpoints), SeenUnix: now.Unix()}, expires: now.Add(s.ttl)}
	}
	peers := make([]Candidate, 0, len(bucket)+1)
	seen := map[string]bool{}
	for id, rec := range bucket {
		if id != nid.String() {
			peers = append(peers, rec.candidate)
			seen[id] = true
		}
	}
	if boot, ok := s.bootstrap[key]; ok && boot.candidate.NodeID != nid.String() && !seen[boot.candidate.NodeID] {
		candidate := boot.candidate
		candidate.SeenUnix = now.Unix()
		if len(candidate.Endpoints) == 0 {
			host := requestHost(r.Host)
			if host != "" {
				candidate.Endpoints = []string{net.JoinHostPort(host, boot.fallbackPort)}
			}
		}
		if len(candidate.Endpoints) > 0 {
			peers = append(peers, candidate)
		}
	}
	sort.Slice(peers, func(i, j int) bool { return peers[i].SeenUnix > peers[j].SeenUnix })
	if len(peers) > 64 {
		peers = peers[:64]
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(Response{Peers: peers})
}

func requestHost(value string) string {
	if host, _, err := net.SplitHostPort(value); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(strings.TrimSpace(value), "[]")
}

func (s *Server) allowRequest(remote string, now time.Time) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	if host == "" {
		host = "unknown"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.rate[host]
	if rec.last.IsZero() {
		rec.tokens = s.rateBurst
		rec.last = now
	}
	elapsed := now.Sub(rec.last).Seconds()
	rec.tokens += elapsed * s.ratePerSecond
	if rec.tokens > s.rateBurst {
		rec.tokens = s.rateBurst
	}
	rec.last = now
	if rec.tokens < 1 {
		s.rate[host] = rec
		return false
	}
	rec.tokens--
	s.rate[host] = rec
	if len(s.rate) > 4096 {
		cutoff := now.Add(-10 * time.Minute)
		for key, value := range s.rate {
			if value.last.Before(cutoff) {
				delete(s.rate, key)
			}
		}
	}
	return true
}
