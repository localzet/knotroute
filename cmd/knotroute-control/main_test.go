package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/localzet/knotroute/internal/ops"
)

func testControlServer(t *testing.T) *server {
	t.Helper()
	store, err := ops.OpenStore(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return &server{store: store, admin: "test-admin", enrollToken: "test-enroll", sessions: map[string]time.Time{}, loginFails: map[string][]time.Time{}}
}

func TestOnboarding(t *testing.T) {
	n := ops.Network{ID: "kn_abcdefghijklmnopqrstuvwxyz234567", Name: "Prod", Beacons: []string{"https://beacon.example"}}
	out, err := buildOnboarding(n, "android", "", "en")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.URI, "knotroute://join?") {
		t.Fatalf("bad uri %q", out.URI)
	}
	if !strings.Contains(out.Instructions, n.ID) {
		t.Fatal("network id missing")
	}
}

func TestSaveNetworkAllowsExistingOrNoBeacon(t *testing.T) {
	s := testControlServer(t)
	for _, tc := range []struct {
		name    string
		beacons []string
	}{
		{name: "without beacon", beacons: nil},
		{name: "external beacon", beacons: []string{"http://beacon.example:8080/"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(networkRequest{ID: ops.NewNetworkID(), Name: "Production", Beacons: tc.beacons})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/networks", strings.NewReader(string(body)))
			rec := httptest.NewRecorder()
			s.saveNetwork(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("save network: status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestSaveNetworkRejectsRelayPortAsBeacon(t *testing.T) {
	s := testControlServer(t)
	body, _ := json.Marshal(networkRequest{ID: ops.NewNetworkID(), Name: "Production", Beacons: []string{"http://beacon.example:7447"}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/networks", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()
	s.saveNetwork(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	var problem map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem["code"] != "beacon_url_invalid" || !strings.Contains(strings.ToLower(problem["hint"].(string)), "7447") {
		t.Fatalf("unexpected problem: %#v", problem)
	}
}

func TestCheckBeaconHealth(t *testing.T) {
	beacon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer beacon.Close()

	s := testControlServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/beacons/check?url="+url.QueryEscape(beacon.URL), nil)
	rec := httptest.NewRecorder()
	s.checkBeacon(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("check beacon: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func signedAgentRequest(t *testing.T, s *server, private ed25519.PrivateKey, agentID, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Unix()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Knot-Agent", agentID)
	req.Header.Set("X-Knot-Timestamp", strconv.FormatInt(stamp, 10))
	req.Header.Set("X-Knot-Signature", ops.SignRequest(private, http.MethodPost, path, stamp, body))
	rec := httptest.NewRecorder()
	var handler http.HandlerFunc
	switch path {
	case "/api/v1/agents/heartbeat":
		handler = s.agentSigned(s.heartbeat)
	case "/api/v1/agents/jobs/next":
		handler = s.agentSigned(s.nextJob)
	case "/api/v1/agents/jobs/result":
		handler = s.agentSigned(s.jobResult)
	default:
		t.Fatalf("unsupported signed test path %q", path)
	}
	handler(rec, req)
	return rec
}

func TestAgentLifecycleEnrollHeartbeatJobResult(t *testing.T) {
	s := testControlServer(t)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	enrollBody, _ := json.Marshal(ops.EnrollRequest{
		Name:      "srv-01",
		PublicKey: base64.RawStdEncoding.EncodeToString(public),
		Token:     "test-enroll",
		Tags:      []string{"prod", "docker"},
	})
	enrollReq := httptest.NewRequest(http.MethodPost, "/api/v1/agents/enroll", strings.NewReader(string(enrollBody)))
	enrollRec := httptest.NewRecorder()
	s.enroll(enrollRec, enrollReq)
	if enrollRec.Code != http.StatusCreated {
		t.Fatalf("enroll: status=%d body=%s", enrollRec.Code, enrollRec.Body.String())
	}
	var enrolled ops.EnrollResponse
	if err := json.Unmarshal(enrollRec.Body.Bytes(), &enrolled); err != nil {
		t.Fatal(err)
	}
	if enrolled.AgentID != ops.AgentID(public) {
		t.Fatalf("agent id=%q want=%q", enrolled.AgentID, ops.AgentID(public))
	}

	hbRec := signedAgentRequest(t, s, private, enrolled.AgentID, "/api/v1/agents/heartbeat", ops.Heartbeat{
		Name:            "srv-01",
		Hostname:        "host-a",
		Version:         "3.1.0-test",
		DockerAvailable: true,
		DockerVersion:   "28.0/test",
		Components:      []ops.Component{{ID: "beacon-a", Kind: "beacon", Name: "beacon-a", Status: "running"}},
	})
	if hbRec.Code != http.StatusOK {
		t.Fatalf("heartbeat: status=%d body=%s", hbRec.Code, hbRec.Body.String())
	}

	jobBody, _ := json.Marshal(jobRequest{
		AgentID: enrolled.AgentID,
		Kind:    "restart_component",
		Payload: map[string]any{"component_id": "beacon-a"},
	})
	jobReq := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(string(jobBody)))
	jobRec := httptest.NewRecorder()
	s.createJob(jobRec, jobReq)
	if jobRec.Code != http.StatusCreated {
		t.Fatalf("create job: status=%d body=%s", jobRec.Code, jobRec.Body.String())
	}
	var created ops.Job
	if err := json.Unmarshal(jobRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	nextRec := signedAgentRequest(t, s, private, enrolled.AgentID, "/api/v1/agents/jobs/next", map[string]any{})
	if nextRec.Code != http.StatusOK {
		t.Fatalf("next job: status=%d body=%s", nextRec.Code, nextRec.Body.String())
	}
	var delivered ops.Job
	if err := json.Unmarshal(nextRec.Body.Bytes(), &delivered); err != nil {
		t.Fatal(err)
	}
	if delivered.ID != created.ID || delivered.Status != "running" || delivered.StartedAt == nil {
		t.Fatalf("unexpected delivered job: %#v", delivered)
	}

	resultRec := signedAgentRequest(t, s, private, enrolled.AgentID, "/api/v1/agents/jobs/result", ops.JobResult{
		JobID: created.ID, Status: "succeeded", Result: "restarted",
	})
	if resultRec.Code != http.StatusOK {
		t.Fatalf("job result: status=%d body=%s", resultRec.Code, resultRec.Body.String())
	}

	var snapshot ops.State
	s.store.View(func(st ops.State) { snapshot = st })
	agent := snapshot.Agents[enrolled.AgentID]
	if agent.Hostname != "host-a" || !agent.DockerAvailable || len(agent.Components) != 1 {
		t.Fatalf("unexpected agent after heartbeat: %#v", agent)
	}
	final := snapshot.Jobs[created.ID]
	if final.Status != "succeeded" || final.Result != "restarted" || final.EndedAt == nil {
		t.Fatalf("unexpected final job: %#v", final)
	}

	emptyRec := signedAgentRequest(t, s, private, enrolled.AgentID, "/api/v1/agents/jobs/next", map[string]any{})
	if emptyRec.Code != http.StatusNoContent {
		t.Fatalf("expected no content after completed queue, got %d: %s", emptyRec.Code, emptyRec.Body.String())
	}
}

func TestAgentSignedRejectsTamperedBody(t *testing.T) {
	s := testControlServer(t)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := ops.AgentID(public)
	if err := s.store.Update(func(st *ops.State) error {
		st.Agents[id] = ops.Agent{ID: id, PublicKey: base64.RawStdEncoding.EncodeToString(public), Name: "agent"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	original := []byte(`{"name":"safe"}`)
	stamp := time.Now().Unix()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/heartbeat", strings.NewReader(`{"name":"tampered"}`))
	req.Header.Set("X-Knot-Agent", id)
	req.Header.Set("X-Knot-Timestamp", strconv.FormatInt(stamp, 10))
	req.Header.Set("X-Knot-Signature", ops.SignRequest(private, http.MethodPost, "/api/v1/agents/heartbeat", stamp, original))
	rec := httptest.NewRecorder()
	s.agentSigned(s.heartbeat)(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("tampered request accepted: status=%d body=%s", rec.Code, rec.Body.String())
	}
}
