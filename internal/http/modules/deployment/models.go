package deployment

import (
	"database/sql"
	"time"
)

type Deployment struct {
	Id            string
	ProjectId     string
	AgentId       sql.NullString
	TriggeredBy   sql.NullString
	TriggerSource sql.NullString
	CommitSha     sql.NullString
	CommitMessage sql.NullString
	Branch        sql.NullString
	Status        string
	ImageTag      sql.NullString
	ContainerId   sql.NullString
	ErrorMessage  sql.NullString
	StartedAt     sql.NullTime
	CompletedAt   sql.NullTime
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ProjectRef struct {
	ProjectId     string
	UserId        string
	DefaultBranch string
	AgentId       sql.NullString
}
type BuildContext struct {
	DeploymentId   string
	ProjectId      string
	Branch         string
	RepoFullName   string
	InstallationId string // github ka numeric id, humari row ka nahi
}
