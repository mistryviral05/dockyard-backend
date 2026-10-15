// internal/pkg/id/id.go
package id

import (
	"log"

	"github.com/nrednav/cuid2"
)

var generate func() string

func init() {
	var err error

	generate, err = cuid2.Init(
		cuid2.WithLength(32),
	)
	if err != nil {
		log.Fatalf("failed to initialize cuid2 generator: %v", err)
	}
}

func UserID() string {
	return "user_" + generate()
}

func WorkspaceID() string {
	return "workspace_" + generate()
}

func ProjectID() string {
	return "project_" + generate()
}

func AgentID() string {
	return "agent_" + generate()
}

func AgentMetricID() string {
	return "metric_" + generate()
}

func DeploymentID() string {
	return "deployment_" + generate()
}
func InstallationID() string {
	return "installation_" + generate()
}

func RepositoryID() string {
	return "repository_" + generate()
}

func LogID() string {
	return "log_" + generate()
}
