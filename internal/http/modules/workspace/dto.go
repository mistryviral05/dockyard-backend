package workspace

type CreateWorkspaceDto struct {
	UserId string `json:"user_id"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
}
