package queue


const (
	TypeDebugPing = "debug:ping"

)

const (
	QueueCritical = "critical"
	QueueDefault  = "default"
	QueueLow      = "low"
)


type DebugPingPayload struct {
	Message string `json:"message"`
}