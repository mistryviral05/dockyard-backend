package project

import "time"

type CreateProjectRequest struct {
	Name        string `json:"project_name" validate:"required,min=2,max=255"`
	Stack       string `json:"project_stack" validate:"required,oneof=nextjs nodejs react go fastapi python django express"`
	WorkspaceId string `json:"workspace_id" validate:"required,max=64"`

	RepositoryRequest
}

type RepositoryRequest struct {
	GithubRepoId string `json:"github_repo_id" validate:"required,numeric,max=64"`
	RepoName     string `json:"repo_name" validate:"required,max=255"`
	RepoFullName string `json:"repo_full_name" validate:"required,max=255"`
	RepoUrl      string `json:"repo_url" validate:"required,url,max=2048"`
	RepoOwner    string `json:"repo_owner" validate:"required,max=255"`
	IsPrivate    bool   `json:"is_private"`

	// Repo ka default branch. Khali aaya to service "main" maan leta hai,
	// isliye frontend me branch selector aane tak ye optional hi rahega.
	DefaultBranch string `json:"default_branch" validate:"omitempty,max=255"`
}

type ProjectDto struct {
	Id               string  `json:"id"`
	Name             string  `json:"project_name"`
	Stack            string  `json:"project_stack"`
	RepoName         string  `json:"repo_name"`
	DeploymentStatus *string `json:"deployment_status"`
}

type SetAgentRequest struct {
	AgentId string `json:"agent_id" validate:"required,max=64"`
}

// ProjectDetailDto project page ka poora data ek call me deta hai.
// repository, agent aur last_deployment null ho sakte hain: agent tab tak
// null rehta hai jab tak user use choose nahi karta, aur last_deployment
// pehle deploy se pehle null hi hota hai.
type ProjectDetailDto struct {
	Id            string    `json:"id"`
	Name          string    `json:"project_name"`
	Stack         string    `json:"project_stack"`
	DefaultBranch string    `json:"default_branch"`
	IsDeployed    bool      `json:"is_deployed"`
	CreatedAt     time.Time `json:"created_at"`

	Repository     *RepositoryDto     `json:"repository"`
	Agent          *ProjectAgentDto   `json:"agent"`
	LastDeployment *LastDeploymentDto `json:"last_deployment"`
}

type RepositoryDto struct {
	RepoName     string `json:"repo_name"`
	RepoFullName string `json:"repo_full_name"`
	RepoUrl      string `json:"repo_url"`
	IsPrivate    bool   `json:"is_private"`
}

type ProjectAgentDto struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Port   string `json:"port"`
}

type LastDeploymentDto struct {
	Id        string    `json:"id"`
	Status    string    `json:"status"`
	Branch    string    `json:"branch"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateProjectDto struct {
	ClerkId       string
	ProjectId     string
	WorkspaceId   string
	Name          string
	Stack         string
	DefaultBranch string
}

type CreateRepositoryDto struct {
	ClerkId       string
	RepositoryId  string
	ProjectId     string
	RepoOwner     string
	GithubRepoId  string
	RepoName      string
	RepoFullName  string
	RepoUrl       string
	IsPrivate     bool
	DefaultBranch string
}
