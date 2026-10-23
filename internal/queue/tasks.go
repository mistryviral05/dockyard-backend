package queue

const (
	TypeDebugPing   = "debug:ping"
	TypeDeployBuild = "deploy:build"
)

const (
	QueueCritical = "critical"
	QueueDefault  = "default"
	QueueLow      = "low"
)

type DebugPingPayload struct {
	Message string `json:"message"`
}

// Only the id travels: the task can sit in Redis for a while, so the
// worker reads fresh state from Postgres instead of trusting the payload.
type DeployBuildPayload struct {
	DeploymentId string `json:"deployment_id"`
}
