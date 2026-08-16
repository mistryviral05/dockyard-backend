package deployment


type CreateDeploymentDto struct {
	DeploymentId  string
	ProjectId     string
	AgentId       string
	TriggeredBy   interface{} // string ya nil (webhook pe user nahi hota)
	TriggerSource string
	Branch        string
	CommitSha     interface{}
	CommitMessage interface{}
}
