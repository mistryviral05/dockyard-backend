package project

import (
	"database/sql"
	"time"
)

type Project struct {
	Id               string
	ProjectName      sql.NullString
	ProjectStack     sql.NullString
	RepoName         sql.NullString
	DeploymentStatus sql.NullString
}

// ProjectDetail ek hi row hai jisme project ke saath uska repo, agent aur
// sabse naya deployment bhi aata hai. Ye teeno LEFT JOIN se aate hain,
// isliye sab Null* hain.
type ProjectDetail struct {
	Id            string
	ProjectName   sql.NullString
	ProjectStack  sql.NullString
	DefaultBranch string
	IsDeployed    bool
	CreatedAt     time.Time

	RepoName      sql.NullString
	RepoFullName  sql.NullString
	RepoUrl       sql.NullString
	RepoIsPrivate sql.NullBool

	AgentId     sql.NullString
	AgentName   sql.NullString
	AgentStatus sql.NullString
	AgentPort   sql.NullString

	DeploymentId        sql.NullString
	DeploymentStatus    sql.NullString
	DeploymentBranch    sql.NullString
	DeploymentCreatedAt sql.NullTime
}
