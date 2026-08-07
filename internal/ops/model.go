package ops

import "time"

const Version = "3.1.0"

type Network struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Beacons     []string  `json:"beacons"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Component struct {
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`
	Name      string            `json:"name"`
	Container string            `json:"container,omitempty"`
	Image     string            `json:"image,omitempty"`
	Status    string            `json:"status,omitempty"`
	Version   string            `json:"version,omitempty"`
	Address   string            `json:"address,omitempty"`
	Service   string            `json:"service,omitempty"`
	Target    string            `json:"target,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type Agent struct {
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	PublicKey       string      `json:"public_key"`
	Hostname        string      `json:"hostname,omitempty"`
	Version         string      `json:"version,omitempty"`
	DockerAvailable bool        `json:"docker_available"`
	DockerVersion   string      `json:"docker_version,omitempty"`
	Tags            []string    `json:"tags,omitempty"`
	Components      []Component `json:"components,omitempty"`
	EnrolledAt      time.Time   `json:"enrolled_at"`
	LastSeen        time.Time   `json:"last_seen"`
}

type Job struct {
	ID        string         `json:"id"`
	AgentID   string         `json:"agent_id"`
	Kind      string         `json:"kind"`
	Payload   map[string]any `json:"payload"`
	Status    string         `json:"status"`
	Result    string         `json:"result,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	StartedAt *time.Time     `json:"started_at,omitempty"`
	EndedAt   *time.Time     `json:"ended_at,omitempty"`
}

type State struct {
	Networks map[string]Network `json:"networks"`
	Agents   map[string]Agent   `json:"agents"`
	Jobs     map[string]Job     `json:"jobs"`
}

type Heartbeat struct {
	Name            string      `json:"name"`
	Hostname        string      `json:"hostname"`
	Version         string      `json:"version"`
	DockerAvailable bool        `json:"docker_available"`
	DockerVersion   string      `json:"docker_version,omitempty"`
	Tags            []string    `json:"tags,omitempty"`
	Components      []Component `json:"components,omitempty"`
}

type EnrollRequest struct {
	Name      string   `json:"name"`
	PublicKey string   `json:"public_key"`
	Token     string   `json:"token"`
	Tags      []string `json:"tags,omitempty"`
}

type EnrollResponse struct {
	AgentID string `json:"agent_id"`
}

type JobResult struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
	Result string `json:"result,omitempty"`
}
