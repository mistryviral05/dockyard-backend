package agent

import "time"

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
