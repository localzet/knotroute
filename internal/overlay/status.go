package overlay

import "time"

type PeerStatus struct {
	ID         string   `json:"id"`
	ShortID    string   `json:"short_id"`
	Direction  string   `json:"direction"`
	RemoteAddr string   `json:"remote_addr"`
	Advertise  []string `json:"advertise"`
}

type RouteStatus struct {
	Destination string   `json:"destination"`
	ShortID     string   `json:"short_id"`
	NextHop     string   `json:"next_hop"`
	Hops        int      `json:"hops"`
	Path        []string `json:"path"`
	Services    []string `json:"services"`
}

type ServiceStatus struct {
	Name        string `json:"name"`
	Target      string `json:"target"`
	Description string `json:"description,omitempty"`
}

type ForwardStatus struct {
	Listen  string `json:"listen"`
	Node    string `json:"node"`
	Service string `json:"service"`
	Active  bool   `json:"active"`
	Error   string `json:"error,omitempty"`
}

type Event struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

type Status struct {
	Name           string          `json:"name"`
	Version        string          `json:"version"`
	NodeID         string          `json:"node_id"`
	ShortID        string          `json:"short_id"`
	StartedAt      time.Time       `json:"started_at"`
	Listen         []string        `json:"listen"`
	Peers          []PeerStatus    `json:"peers"`
	Routes         []RouteStatus   `json:"routes"`
	Services       []ServiceStatus `json:"services"`
	Forwards       []ForwardStatus `json:"forwards"`
	ActiveStreams  int             `json:"active_streams"`
	BytesSent      uint64          `json:"bytes_sent"`
	BytesReceived  uint64          `json:"bytes_received"`
	FramesSent     uint64          `json:"frames_sent"`
	FramesReceived uint64          `json:"frames_received"`
	Events         []Event         `json:"events"`
}
