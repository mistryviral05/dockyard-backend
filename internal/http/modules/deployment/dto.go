package deployment

type CreateDeploymentDto struct {
	DeploymentId  string
	ProjectId     string
	AgentId       string
	TriggeredBy   *string
	TriggerSource string
	Branch        string
	CommitSha     *string
	CommitMessage *string
}
