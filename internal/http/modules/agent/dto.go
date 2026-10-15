package agent

import (
	"math"
	"strings"
	"time"
)

type CreateAgentRequest struct {
	Name string `json:"name" validate:"required,min=2,max=255"`
	Port string `json:"port" validate:"omitempty,numeric,min=1,max=5"`
}

type CreateAgentDto struct {
	ClerkId   string
	AgentId   string
	Name      string
	Port      string
	TokenHash string
	TokenExp  time.Time
}

type CreateAgentResponse struct {
	Id             string    `json:"id"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	Port           string    `json:"port"`
	CreatedAt      time.Time `json:"created_at"`
	Token          string    `json:"token"`
	TokenExpiresAt time.Time `json:"token_expires_at"`
	TokenExpiresIn int64     `json:"token_expires_in"`
	RunCommand     string    `json:"run_command"`
}

type HeartbeatRequest struct {
	CPUUsage    *float64  `json:"cpu_usage" validate:"omitempty,gte=0,lte=100"`
	MemoryUsage *float64  `json:"ram_usage" validate:"omitempty,gte=0,lte=100"`
	DiskUsage   *float64  `json:"disk_usage" validate:"omitempty,gte=0,lte=100"`
	Load1       *float64  `json:"load_1" validate:"omitempty,gte=0"`
	Load5       *float64  `json:"load_5" validate:"omitempty,gte=0"`
	Load15      *float64  `json:"load_15" validate:"omitempty,gte=0"`
	CPUCores    int       `json:"cpu_cores" validate:"omitempty,gte=0,lte=4096"`
	UptimeSec   *uint64   `json:"uptime_seconds" validate:"omitempty"`
	OSInfo      string    `json:"os_info" validate:"omitempty,max=255"`
	CollectedAt time.Time `json:"collected_at"`
}

type HeartbeatDto struct {
	AgentID     string
	PublicIP    string
	CPUUsage    *float64
	MemoryUsage *float64
	DiskUsage   *float64
	Load1       *float64
	Load5       *float64
	Load15      *float64
	CPUCores    *int
	UptimeSec   *int64
	OSInfo      string
	RecordedAt  time.Time
}

func (r *HeartbeatRequest) ToDto(agentID, publicIP string) *HeartbeatDto {
	dto := &HeartbeatDto{
		AgentID:     agentID,
		PublicIP:    publicIP,
		CPUUsage:    r.CPUUsage,
		MemoryUsage: r.MemoryUsage,
		DiskUsage:   r.DiskUsage,
		Load1:       r.Load1,
		Load5:       r.Load5,
		Load15:      r.Load15,
		OSInfo:      strings.TrimSpace(r.OSInfo),
		RecordedAt:  time.Now().UTC(),
	}

	if r.CPUCores > 0 {
		cores := r.CPUCores
		dto.CPUCores = &cores
	}

	if r.UptimeSec != nil && *r.UptimeSec <= math.MaxInt64 {
		uptime := int64(*r.UptimeSec)
		dto.UptimeSec = &uptime
	}

	return dto
}

type HeartbeatResponse struct {
	AgentID           string    `json:"agent_id"`
	Status            string    `json:"status"`
	RecordedAt        time.Time `json:"recorded_at"`
	HeartbeatInterval int       `json:"heartbeat_interval_seconds"`
}

type MetricsQuery struct {
	Window time.Duration
	Since  time.Time
}

type MetricsSampleDto struct {
	CPUUsage    *float64  `json:"cpu_usage"`
	MemoryUsage *float64  `json:"ram_usage"`
	DiskUsage   *float64  `json:"disk_usage"`
	Load1       *float64  `json:"load_1"`
	Load5       *float64  `json:"load_5"`
	Load15      *float64  `json:"load_15"`
	LoadPerCore *float64  `json:"load_per_core"`
	RecordedAt  time.Time `json:"recorded_at"`
	AgeSeconds  int64     `json:"age_seconds"`
}

type MetricPointDto struct {
	T    time.Time `json:"t"`
	CPU  *float64  `json:"cpu"`
	RAM  *float64  `json:"ram"`
	Disk *float64  `json:"disk"`
	Load *float64  `json:"load"`
}

type AgentMetricsDto struct {
	AgentID         string     `json:"agent_id"`
	Name            string     `json:"name"`
	Status          string     `json:"status"`
	Live            bool       `json:"live"`
	PublicIP        *string    `json:"public_ip"`
	OSInfo          *string    `json:"os_info"`
	CPUCores        *int       `json:"cpu_cores"`
	UptimeSec       *int64     `json:"uptime_seconds"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at"`

	Current *MetricsSampleDto `json:"current"`

	Series        []*MetricPointDto `json:"series"`
	WindowSeconds int               `json:"window_seconds"`
	BucketSeconds int               `json:"bucket_seconds"`
	PollSeconds   int               `json:"poll_seconds"`
}

type AgentDto struct {
	Id        string  `json:"id"`
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	Subdomain *string `json:"subdomain"`
	Port      string  `json:"port"`
}

// AgentDetailDto agent page ka data hai. Token yahan kabhi nahi aata: wo
// sirf create ke response me ek baar dikhta hai aur db me hash hi rehta hai.
type AgentDetailDto struct {
	Id              string     `json:"id"`
	Name            string     `json:"name"`
	Status          string     `json:"status"`
	Subdomain       *string    `json:"subdomain"`
	Port            string     `json:"port"`
	ProjectCount    int        `json:"project_count"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at"`
	CreatedAt       time.Time  `json:"created_at"`
}
