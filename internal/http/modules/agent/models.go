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
}
