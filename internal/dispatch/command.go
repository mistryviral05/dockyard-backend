package dispatch

const CommandTTLSeconds = 300

type Command struct {
	DeploymentID  string            `json:"deployment_id"`
	ProjectID     string            `json:"project_id"`
	Image         string            `json:"image"`
	Domain        string            `json:"domain"`
	Port          int               `json:"port"`
	RegistryHost  string            `json:"registry_host"`
	RegistryUser  string            `json:"registry_user"`
	RegistryToken string            `json:"registry_token"`
	Env           map[string]string `json:"env,omitempty"`
}

type CallbackRequest struct {
	Status      string `json:"status"`
	ContainerID string `json:"container_id"`
	URL         string `json:"url"`
	Error       string `json:"error"`
}

const (
	StatusRunning = "running"
	StatusFailed  = "failed"
)
