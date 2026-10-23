package workspace

type CreateWorkspaceDto struct {
	ClerkId string `json:"clerk_id"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
}
