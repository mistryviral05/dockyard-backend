package deployment

const (
	StatusPending   = "pending"
	StatusQueued    = "queued"
	StatusBuilding  = "building"
	StatusDeploying = "deploying"
	StatusRunning   = "running"
	StatusFailed    = "failed"
	StatusStopped   = "stopped"
)

const (
	TriggerManual     = "manual"
	TriggerGitHubPush = "github_push"
	TriggerWebhook    = "webhook"
)

var allowedFrom = map[string][]string{
	StatusQueued:    {StatusPending},
	StatusBuilding:  {StatusQueued},
	StatusDeploying: {StatusBuilding},
	StatusRunning:   {StatusDeploying},
	StatusStopped:   {StatusRunning},
	StatusFailed:    {StatusPending, StatusQueued, StatusBuilding, StatusDeploying},
}

var terminal = map[string]bool{
	StatusRunning: true,
	StatusFailed:  true,
	StatusStopped: true,
}

var active = map[string]bool{
	StatusPending:   true,
	StatusQueued:    true,
	StatusBuilding:  true,
	StatusDeploying: true,
}

func PredecessorsOf(to string) ([]string, bool) {
	from, ok := allowedFrom[to]
	return from, ok
}

func IsTerminal(status string) bool {
	return terminal[status]
}

func IsActive(status string) bool {
	return active[status]
}

func IsValidStatus(status string) bool {
	return terminal[status] || active[status]
}

func ActiveStatuses() []string {
	return []string{StatusPending, StatusQueued, StatusBuilding, StatusDeploying}
}
