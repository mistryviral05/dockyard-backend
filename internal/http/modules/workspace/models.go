package workspace
// keep your existing CreateWorkspaceDto; add:


type CreateWorkspaceRequest struct {
	Name string `json:"name" validate:"required"`
	Slug string `json:"slug" validate:"required"`
}