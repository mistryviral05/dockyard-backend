package deployment

import "errors"

var (
	ErrProjectNotFound    = errors.New("project not found")
	ErrAgentNotSet        = errors.New("agent not set")
	ErrDeployInProgress   = errors.New("deployment already in progress")
	ErrDeploymentNotFound = errors.New("deployment not found")
)
