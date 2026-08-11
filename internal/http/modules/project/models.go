package project

import "database/sql"

type Project struct {
	Id               string
	ProjectName      sql.NullString
	ProjectStack     sql.NullString
	RepoName         sql.NullString
	DeploymentStatus sql.NullString
}
