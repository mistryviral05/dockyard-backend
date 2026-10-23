package agent

import (
	"database/sql"
	"time"
)

type Agent struct {
	Id        string
	UserId    string
	AgentName sql.NullString
	Port      string
	Status    string
	Subdomain sql.NullString
	TokenExp  sql.NullTime
	CreatedAt time.Time

	// Sirf detail query bharti hai, list me ye zero rehte hain.
	LastHeartbeatAt sql.NullTime
	ProjectCount    int
}

type MetricPoint struct {
	At   time.Time
	CPU  *float64
	RAM  *float64
	Disk *float64
	Load *float64
}

type AgentMetrics struct {
	AgentID         string
	AgentName       sql.NullString
	Status          string
	LastHeartbeatAt sql.NullTime
	PublicIP        sql.NullString
	OSInfo          sql.NullString

	CPUUsage    *float64
	MemoryUsage *float64
	DiskUsage   *float64
	Load1       *float64
	Load5       *float64
	Load15      *float64
	CPUCores    *int
	UptimeSec   *int64
	RecordedAt  sql.NullTime
}
