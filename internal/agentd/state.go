package agentd

import "time"

type State struct {
	AgentID           string    `json:"agent_id"`
	ControlPlaneToken string    `json:"control_plane_token"` // bearer
	SigningSecret     string    `json:"signing_secret"`      // HMAC, symmetric
	RegisteredAt      time.Time `json:"registered_at"`
}

func LoadState(path string) (*State, error) {
	return nil, nil
}
func SaveState(path string, s *State) error {
	return nil
}

func (s *State) IsRegistered() bool {
	return true
}
