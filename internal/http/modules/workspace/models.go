package workspace
// keep your existing CreateWorkspaceDto; add:


type CreateWorkspaceRequest struct {
	Name string `json:"name" validate:"required"`
	Slug string `json:"slug" validate:"required"`
}

type Workspace struct {
	Id    string `json:"id"`
	WspName  string `json:"wsp_name"`
	WspSlug  string `json:"wsp_slug"`
}