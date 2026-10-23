package deployment

import "errors"

var (
	ErrProjectNotFound    = errors.New("project not found")
	ErrAgentNotSet        = errors.New("agent not set")
	ErrDeployInProgress   = errors.New("deployment already in progress")
	ErrDeploymentNotFound = errors.New("deployment not found")
	ErrRepoNotLinked      = errors.New("repository not linked")

	ErrNoAgentForProject  = errors.New("no agent is attached to this project")
	ErrAgentNotRegistered = errors.New("the agent has not finished registration")
)
