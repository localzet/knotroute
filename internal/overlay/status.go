package overlay

import (
	"time"

	transportpkg "github.com/localzet/knotroute/internal/transport"
)

type PeerStatus struct {
	ID         string   `json:"id"`
	ShortID    string   `json:"short_id"`
	Direction  string   `json:"direction"`
	RemoteAddr string   `json:"remote_addr"`
	Advertise  []string `json:"advertise"`
}

type RouteStatus struct {
	Destination string   `json:"destination"`
	Domain      string   `json:"domain"`
	ShortID     string   `json:"short_id"`
	NextHop     string   `json:"next_hop"`
	Hops        int      `json:"hops"`
	Path        []string `json:"path"`
	Services    []string `json:"services"`
}

type ServiceStatus struct {
	Name         string   `json:"name"`
	Target       string   `json:"target"`
	Description  string   `json:"description,omitempty"`
	Published    bool     `json:"published"`
	ServiceID    string   `json:"service_id,omitempty"`
	Domain       string   `json:"domain,omitempty"`
	Introduction []string `json:"introduction_points,omitempty"`
}

type KnownServiceStatus struct {
	ServiceID          string            `json:"service_id"`
	Domain             string            `json:"domain"`
	Metadata           map[string]string `json:"metadata"`
	Revision           uint64            `json:"revision"`
	ExpiresUnix        int64             `json:"expires_unix"`
	IntroductionPoints []string          `json:"introduction_points"`
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
	Name           string               `json:"name"`
	Version        string               `json:"version"`
	NetworkID      string               `json:"network_id"`
	NodeID         string               `json:"node_id"`
	Domain         string               `json:"domain"`
	ShortID        string               `json:"short_id"`
	StartedAt      time.Time            `json:"started_at"`
	Listen         []string             `json:"listen"`
	Peers          []PeerStatus         `json:"peers"`
	Routes         []RouteStatus        `json:"routes"`
	Services       []ServiceStatus      `json:"services"`
	KnownServices  []KnownServiceStatus `json:"known_services"`
	Forwards       []ForwardStatus      `json:"forwards"`
	Aliases        []AliasStatus        `json:"aliases"`
	Proxy          ProxyStatus          `json:"proxy"`
	Transport      transportpkg.Status  `json:"transport"`
	ActiveStreams  int                  `json:"active_streams"`
	ActiveCircuits int                  `json:"active_circuits"`
	Descriptors    int                  `json:"descriptors"`
	BytesSent      uint64               `json:"bytes_sent"`
	BytesReceived  uint64               `json:"bytes_received"`
	FramesSent     uint64               `json:"frames_sent"`
	FramesReceived uint64               `json:"frames_received"`
	Events         []Event              `json:"events"`
}

type AliasStatus struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Target      string `json:"target"`
	Domain      string `json:"domain"`
	Description string `json:"description,omitempty"`
}

type ProxyStatus struct {
	SOCKS     string   `json:"socks"`
	HTTP      string   `json:"http"`
	PAC       string   `json:"pac"`
	Direct    bool     `json:"direct"`
	Listeners []string `json:"listeners"`
}
