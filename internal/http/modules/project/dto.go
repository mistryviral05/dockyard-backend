package project

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
}

type CreateProjectDto struct {
	ClerkId     string
	ProjectId   string
	WorkspaceId string
	Name        string
	Stack       string
}

type CreateRepositoryDto struct {
	ClerkId      string
	RepositoryId string
	ProjectId    string
	RepoOwner    string
	GithubRepoId string
	RepoName     string
	RepoFullName string
	RepoUrl      string
	IsPrivate    bool
}
