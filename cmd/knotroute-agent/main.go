package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/localzet/knotroute/internal/discovery"
	"github.com/localzet/knotroute/internal/ops"
)

type identityDisk struct {
	PrivateKey string `json:"private_key"`
	AgentID    string `json:"agent_id,omitempty"`
}

type agent struct {
	controlURL  string
	enrollToken string
	name        string
	tags        []string
	dataDir     string
	workDir     string
	poll        time.Duration
	docker      bool
	imageTag    string
	private     ed25519.PrivateKey
	public      ed25519.PublicKey
	id          string
	http        *http.Client
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	control := strings.TrimRight(strings.TrimSpace(os.Getenv("KNOTROUTE_CONTROL_URL")), "/")
	if control == "" {
		return errors.New("KNOTROUTE_CONTROL_URL is required")
	}
	dataDir := env("KNOTROUTE_AGENT_DATA", "/data")
	name := env("KNOTROUTE_AGENT_NAME", hostname())
	poll := envDuration("KNOTROUTE_AGENT_POLL", 10*time.Second)
	dockerEnabled := envBool("KNOTROUTE_AGENT_DOCKER", false)
	a := &agent{controlURL: control, enrollToken: strings.TrimSpace(os.Getenv("KNOTROUTE_CONTROL_ENROLL_TOKEN")), name: name, tags: splitCSV(os.Getenv("KNOTROUTE_AGENT_TAGS")), dataDir: dataDir, workDir: filepath.Join(dataDir, "stacks"), poll: poll, docker: dockerEnabled, imageTag: env("KNOTROUTE_AGENT_IMAGE_TAG", managedImageTag(ops.Version)), http: &http.Client{Timeout: 30 * time.Second}}
	if err := a.loadIdentity(); err != nil {
		return err
	}
	if a.id == "" {
		if a.enrollToken == "" {
			return errors.New("KNOTROUTE_CONTROL_ENROLL_TOKEN is required for first enrollment")
		}
		if err := a.enroll(); err != nil {
			return err
		}
	}
	log.Printf("KnotRoute Agent %s started as %s (%s)", ops.Version, a.name, a.id)
	if a.docker {
		if _, err := exec.LookPath("docker"); err != nil {
			return errors.New("Docker management enabled but docker CLI is unavailable")
		}
	}
	ticker := time.NewTicker(a.poll)
	defer ticker.Stop()
	for {
		if err := a.heartbeat(); err != nil {
			log.Printf("heartbeat: %v", err)
		}
		if err := a.pollJob(); err != nil {
			log.Printf("job poll: %v", err)
		}
		<-ticker.C
	}
}

func (a *agent) loadIdentity() error {
	if err := os.MkdirAll(a.dataDir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(a.dataDir, "agent-identity.json")
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		a.public = pub
		a.private = priv
		return a.saveIdentity()
	}
	if err != nil {
		return err
	}
	var disk identityDisk
	if err := json.Unmarshal(raw, &disk); err != nil {
		return err
	}
	priv, err := base64.RawStdEncoding.DecodeString(disk.PrivateKey)
	if err != nil || len(priv) != ed25519.PrivateKeySize {
		return errors.New("invalid agent identity")
	}
	a.private = ed25519.PrivateKey(priv)
	a.public = a.private.Public().(ed25519.PublicKey)
	a.id = disk.AgentID
	return nil
}
func (a *agent) saveIdentity() error {
	raw, err := json.MarshalIndent(identityDisk{PrivateKey: base64.RawStdEncoding.EncodeToString(a.private), AgentID: a.id}, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	path := filepath.Join(a.dataDir, "agent-identity.json")
	tmp, err := os.CreateTemp(a.dataDir, ".agent-identity-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func (a *agent) enroll() error {
	body, _ := json.Marshal(ops.EnrollRequest{Name: a.name, PublicKey: base64.RawStdEncoding.EncodeToString(a.public), Token: a.enrollToken, Tags: a.tags})
	req, err := http.NewRequest(http.MethodPost, a.controlURL+"/api/v1/agents/enroll", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		return fmt.Errorf("enrollment failed: %s: %s", resp.Status, readSmall(resp.Body))
	}
	var out ops.EnrollResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if out.AgentID != ops.AgentID(a.public) {
		return errors.New("control returned unexpected agent id")
	}
	a.id = out.AgentID
	return a.saveIdentity()
}

func (a *agent) heartbeat() error {
	dockerVersion := ""
	dockerAvailable := false
	components := []ops.Component{}
	if a.docker {
		if out, err := command(10*time.Second, "docker", "version", "--format", "{{.Client.Version}}/{{.Server.Version}}"); err == nil {
			dockerAvailable = true
			dockerVersion = strings.TrimSpace(out)
		}
		components = a.inspectComponents()
	}
	hb := ops.Heartbeat{Name: a.name, Hostname: hostname(), Version: ops.Version, DockerAvailable: dockerAvailable, DockerVersion: dockerVersion, Tags: a.tags, Components: components}
	return a.signedJSON("/api/v1/agents/heartbeat", hb, nil)
}
func (a *agent) pollJob() error {
	var job ops.Job
	status, err := a.signedJSONStatus("/api/v1/agents/jobs/next", map[string]any{}, &job)
	if err != nil {
		return err
	}
	if status == http.StatusNoContent {
		return nil
	}
	result := a.execute(job)
	return a.signedJSON("/api/v1/agents/jobs/result", result, nil)
}
func (a *agent) execute(job ops.Job) ops.JobResult {
	result := ops.JobResult{JobID: job.ID, Status: "succeeded"}
	var err error
	switch job.Kind {
	case "deploy_beacon":
		result.Result, err = a.deployBeacon(job)
	case "deploy_sidecar":
		result.Result, err = a.deploySidecar(job)
	case "restart_component":
		result.Result, err = a.restartComponent(job)
	case "remove_component":
		result.Result, err = a.removeComponent(job)
	default:
		err = errors.New("unsupported job")
	}
	if err != nil {
		result.Status = "failed"
		result.Result = err.Error()
	}
	return result
}

func (a *agent) deployBeacon(job ops.Job) (string, error) {
	if !a.docker {
		return "", errors.New("Docker management is disabled on this agent")
	}
	p := job.Payload
	name, err := requiredString(p, "name")
	if err != nil {
		return "", err
	}
	network, err := requiredString(p, "network_id")
	if err != nil || !validNetworkID(network) {
		return "", errors.New("invalid network_id")
	}
	beaconURL, err := requiredString(p, "beacon_url")
	if err != nil {
		return "", err
	}
	beaconURL, err = discovery.ValidateBeaconURL(beaconURL)
	if err != nil {
		return "", fmt.Errorf("beacon_url: %w", err)
	}
	advertise, err := requiredString(p, "advertise")
	if err != nil {
		return "", err
	}
	httpPort, err := boundedNumber(p, "http_port", 18080, 1, 65535)
	if err != nil {
		return "", err
	}
	_, advertisedPort, err := net.SplitHostPort(advertise)
	if err != nil {
		return "", fmt.Errorf("advertise must be host:port: %w", err)
	}
	relayPort, err := strconv.Atoi(advertisedPort)
	if err != nil || relayPort < 1 || relayPort > 65535 {
		return "", errors.New("advertise port must be 1..65535")
	}
	slug, err := slug(name)
	if err != nil {
		return "", err
	}
	image := "ghcr.io/localzet/knotroute-beacon:" + a.imageTag
	if v := optionalString(p, "image"); v != "" {
		if !strings.HasPrefix(v, "ghcr.io/localzet/knotroute-beacon:") {
			return "", errors.New("unsupported beacon image")
		}
		image = v
	}
	dir := filepath.Join(a.workDir, "beacon-"+slug)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	compose := renderBeaconCompose(image, "beacon-"+slug, network, beaconURL, httpPort, relayPort, advertise)
	if err := writeCompose(dir, compose); err != nil {
		return "", err
	}
	out, err := a.compose(dir, "up", "-d", "--pull", "always")
	return out, err
}

func (a *agent) deploySidecar(job ops.Job) (string, error) {
	if !a.docker {
		return "", errors.New("Docker management is disabled on this agent")
	}
	p := job.Payload
	name, err := requiredString(p, "name")
	if err != nil {
		return "", err
	}
	networkID, err := requiredString(p, "network_id")
	if err != nil || !validNetworkID(networkID) {
		return "", errors.New("invalid network_id")
	}
	target, err := requiredString(p, "target")
	if err != nil || !validTarget(target) {
		return "", errors.New("target must be container-or-host:port")
	}
	dockerNetwork, err := requiredString(p, "docker_network")
	if err != nil {
		return "", err
	}
	if _, err := slug(dockerNetwork); err != nil {
		return "", errors.New("invalid Docker network name")
	}
	beacons := stringSlice(p, "beacons")
	for i, raw := range beacons {
		normalized, err := discovery.ValidateBeaconURL(raw)
		if err != nil {
			return "", fmt.Errorf("beacon %d: %w", i+1, err)
		}
		beacons[i] = normalized
	}
	if len(beacons) == 0 {
		return "", errors.New("network has no Beacon HTTP URLs; add the already-running Beacon to the network profile in Control, or configure another discovery path")
	}
	slugName, err := slug(name)
	if err != nil {
		return "", err
	}
	image := "ghcr.io/localzet/knotroute-sidecar:" + a.imageTag
	if v := optionalString(p, "image"); v != "" {
		if !strings.HasPrefix(v, "ghcr.io/localzet/knotroute-sidecar:") {
			return "", errors.New("unsupported sidecar image")
		}
		image = v
	}
	advertise := optionalString(p, "advertise")
	ports := ""
	if advertise != "" {
		_, hostPort, splitErr := net.SplitHostPort(advertise)
		if splitErr != nil {
			return "", fmt.Errorf("advertise must be host:port: %w", splitErr)
		}
		portNumber, convErr := strconv.Atoi(hostPort)
		if convErr != nil || portNumber < 1 || portNumber > 65535 {
			return "", errors.New("advertise port must be 1..65535")
		}
		ports = fmt.Sprintf("    ports:\n      - \"%d:7447\"\n", portNumber)
	}
	dir := filepath.Join(a.workDir, "sidecar-"+slugName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	compose := renderSidecarCompose(image, "sidecar-"+slugName, networkID, name, target, ports, beacons, advertise, dockerNetwork)
	if err := writeCompose(dir, compose); err != nil {
		return "", err
	}
	out, err := a.compose(dir, "up", "-d", "--pull", "always")
	return out, err
}

func renderBeaconCompose(image, componentID, networkID, beaconURL string, httpPort, relayPort int, advertise string) string {
	return fmt.Sprintf(`services:
  beacon:
    image: %q
    restart: unless-stopped
    labels:
      io.knotroute.managed: "true"
      io.knotroute.component: "beacon"
      io.knotroute.component-id: %q
      io.knotroute.network-id: %q
      io.knotroute.public-url: %q
    ports:
      - %q
      - %q
    volumes:
      - data:/data
    environment:
      KNOTROUTE_NETWORK_ID: %q
      KNOTROUTE_BEACON_LISTEN: "0.0.0.0:8080"
      KNOTROUTE_BEACON_RELAY: "true"
      KNOTROUTE_BEACON_RELAY_LISTEN: "0.0.0.0:7447"
      KNOTROUTE_BEACON_RELAY_ADVERTISE: %q
      KNOTROUTE_BEACON_DATA: "/data"
volumes:
  data:
`, image, componentID, networkID, beaconURL, fmt.Sprintf("%d:8080", httpPort), fmt.Sprintf("%d:7447", relayPort), networkID, advertise)
}

func renderSidecarCompose(image, componentID, networkID, name, target, ports string, beacons []string, advertise, dockerNetwork string) string {
	return fmt.Sprintf(`services:
  knotroute:
    image: %q
    restart: unless-stopped
    labels:
      io.knotroute.managed: "true"
      io.knotroute.component: "sidecar"
      io.knotroute.component-id: %q
      io.knotroute.network-id: %q
      io.knotroute.service-name: %q
      io.knotroute.target: %q
    networks:
      - target
%s    volumes:
      - data:/data
    environment:
      KNOTROUTE_CONFIG_FROM_ENV: "true"
      KNOTROUTE_NETWORK_ID: %q
      KNOTROUTE_BEACONS: %q
      KNOTROUTE_SERVICE_NAME: %q
      KNOTROUTE_SERVICE_TARGET: %q
      KNOTROUTE_ADVERTISE: %q
networks:
  target:
    external: true
    name: %q
volumes:
  data:
`, image, componentID, networkID, name, target, ports, networkID, strings.Join(beacons, ","), name, target, advertise, dockerNetwork)
}

func (a *agent) restartComponent(job ops.Job) (string, error) {
	id := optionalString(job.Payload, "component_id")
	if id == "" {
		return "", errors.New("component_id required")
	}
	container, err := a.containerForComponent(id)
	if err != nil {
		return "", err
	}
	return command(30*time.Second, "docker", "restart", container)
}
func (a *agent) removeComponent(job ops.Job) (string, error) {
	id := optionalString(job.Payload, "component_id")
	if id == "" {
		return "", errors.New("component_id required")
	}
	kind := strings.TrimPrefix(strings.Split(id, "-")[0], "/")
	var dir string
	switch kind {
	case "beacon":
		dir = filepath.Join(a.workDir, id)
	case "sidecar":
		dir = filepath.Join(a.workDir, id)
	default:
		if strings.HasPrefix(id, "beacon-") {
			dir = filepath.Join(a.workDir, id)
		} else if strings.HasPrefix(id, "sidecar-") {
			dir = filepath.Join(a.workDir, id)
		} else {
			return "", errors.New("component is not managed by this agent")
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "compose.yaml")); err != nil {
		return "", errors.New("managed stack not found")
	}
	return a.compose(dir, "down")
}
func (a *agent) compose(dir string, args ...string) (string, error) {
	all := append([]string{"compose", "-f", filepath.Join(dir, "compose.yaml"), "-p", "kr-" + filepath.Base(dir)}, args...)
	return command(2*time.Minute, "docker", all...)
}
func (a *agent) containerForComponent(id string) (string, error) {
	out, err := command(15*time.Second, "docker", "ps", "-a", "--filter", "label=io.knotroute.managed=true", "--filter", "label=io.knotroute.component-id="+id, "--format", "{{.Names}}")
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(strings.Split(out, "\n")[0])
	if name == "" {
		return "", errors.New("managed component container not found")
	}
	return name, nil
}
func (a *agent) inspectComponents() []ops.Component {
	out, err := command(15*time.Second, "docker", "ps", "-a", "--filter", "label=io.knotroute.managed=true", "--format", "{{json .}}")
	if err != nil {
		return nil
	}
	var result []ops.Component
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var row map[string]string
		if json.Unmarshal([]byte(line), &row) != nil {
			continue
		}
		labels := parseLabels(row["Labels"])
		c := ops.Component{ID: labels["io.knotroute.component-id"], Kind: labels["io.knotroute.component"], Name: labels["io.knotroute.service-name"], Container: row["Names"], Image: row["Image"], Status: normalizeStatus(row["State"], row["Status"]), Address: labels["io.knotroute.public-url"], Target: labels["io.knotroute.target"], Labels: labels, UpdatedAt: time.Now().UTC()}
		c.Version = imageVersion(c.Image)
		if c.Kind == "sidecar" && c.Container != "" {
			c.Service = discoverServiceAddress(c.Container)
		}
		if c.Name == "" {
			c.Name = c.ID
		}
		if c.ID != "" {
			result = append(result, c)
		}
	}
	return result
}

func (a *agent) signedJSON(path string, in, out any) error {
	_, err := a.signedJSONStatus(path, in, out)
	return err
}
func (a *agent) signedJSONStatus(path string, in, out any) (int, error) {
	body, _ := json.Marshal(in)
	stamp := time.Now().Unix()
	req, err := http.NewRequest(http.MethodPost, a.controlURL+path, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Knot-Agent", a.id)
	req.Header.Set("X-Knot-Timestamp", strconv.FormatInt(stamp, 10))
	req.Header.Set("X-Knot-Signature", ops.SignRequest(a.private, http.MethodPost, path, stamp, body))
	resp, err := a.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return resp.StatusCode, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("control returned %s: %s", resp.Status, readSmall(resp.Body))
	}
	if out != nil {
		return resp.StatusCode, json.NewDecoder(resp.Body).Decode(out)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

func writeCompose(dir, raw string) error {
	path := filepath.Join(dir, "compose.yaml")
	tmp, err := os.CreateTemp(dir, ".compose-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func command(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	text := strings.TrimSpace(out.String())
	if ctx.Err() == context.DeadlineExceeded {
		return text, errors.New("command timed out")
	}
	if err != nil {
		return text, fmt.Errorf("%s %v: %w: %s", name, args, err, text)
	}
	return text, nil
}

func managedImageTag(version string) string {
	version = strings.TrimSpace(strings.TrimPrefix(version, "v"))
	if version == "" || version == "dev" || strings.HasSuffix(version, "-dev") {
		return "latest"
	}
	return version
}

func imageVersion(image string) string {
	idx := strings.LastIndex(image, ":")
	if idx < 0 || idx == len(image)-1 {
		return ""
	}
	return image[idx+1:]
}

func discoverServiceAddress(container string) string {
	out, err := command(5*time.Second, "docker", "logs", "--tail", "80", container)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "publishing ") || !strings.Contains(line, " as ") {
			continue
		}
		parts := strings.SplitN(line, " as ", 2)
		if len(parts) != 2 {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) > 0 && strings.HasSuffix(fields[0], ".knot") {
			return fields[0]
		}
	}
	return ""
}

func parseLabels(raw string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			out[kv[0]] = kv[1]
		}
	}
	return out
}
func normalizeStatus(state, status string) string {
	state = strings.ToLower(strings.TrimSpace(state))
	if state == "running" {
		return "running"
	}
	if state != "" {
		return state
	}
	fields := strings.Fields(status)
	if len(fields) == 0 {
		return "unknown"
	}
	return strings.ToLower(fields[0])
}
func requiredString(m map[string]any, k string) (string, error) {
	v := optionalString(m, k)
	if v == "" {
		return "", fmt.Errorf("%s is required", k)
	}
	return v, nil
}
func optionalString(m map[string]any, k string) string {
	v, ok := m[k].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}
func stringSlice(m map[string]any, k string) []string {
	raw, ok := m[k].([]any)
	if !ok {
		if s, ok := m[k].([]string); ok {
			return s
		}
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}
func boundedNumber(m map[string]any, k string, d, min, max int) (int, error) {
	v, ok := m[k]
	if !ok || v == nil {
		return d, nil
	}
	number, ok := v.(float64)
	if !ok || number != float64(int(number)) {
		return 0, fmt.Errorf("%s must be an integer", k)
	}
	n := int(number)
	if n < min || n > max {
		return 0, fmt.Errorf("%s must be %d..%d", k, min, max)
	}
	return n, nil
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
func validTarget(v string) bool {
	parts := strings.Split(v, ":")
	if len(parts) != 2 {
		return false
	}
	if _, err := slug(parts[0]); err != nil {
		return false
	}
	p, err := strconv.Atoi(parts[1])
	return err == nil && p > 0 && p < 65536
}
func slug(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" || len(v) > 80 {
		return "", errors.New("invalid name")
	}
	for _, r := range v {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '-' && r != '_' && r != '.' {
			return "", errors.New("name contains unsupported characters")
		}
	}
	return strings.ToLower(v), nil
}
func readSmall(r io.Reader) string {
	raw, _ := io.ReadAll(io.LimitReader(r, 16<<10))
	return string(raw)
}
func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return runtime.GOOS
	}
	return h
}
func env(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}
func envBool(k string, d bool) bool {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return d
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return d
	}
	return b
}
func envDuration(k string, d time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return d
	}
	x, err := time.ParseDuration(v)
	if err != nil {
		return d
	}
	return x
}
func splitCSV(v string) []string {
	var out []string
	for _, x := range strings.Split(v, ",") {
		x = strings.TrimSpace(x)
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}
