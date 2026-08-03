package github

type ConnectRequest struct {
	InstallationId string `json:"installation_id" validate:"required"`
	State          string `json:"state" validate:"required"`
	ClerkId        string `validate:"required"`
}

type ConnectDto struct {
	ClerkId             string
	InstallationId      string
	AccountLogin        string
	AccountType         string
	RepositorySelection string
	Permissions         string
}

type InstallationDto struct {
	Id           string `json:"id"`
	AccountLogin string `json:"account_login"`
}
