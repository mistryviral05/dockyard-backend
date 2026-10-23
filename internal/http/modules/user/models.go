package user

import "encoding/json"

type ClerkEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type ClerkUserData struct {
	ID                    string         `json:"id"`
	FirstName             string         `json:"first_name"`
	LastName              string         `json:"last_name"`
	PrimaryEmailAddressID string         `json:"primary_email_address_id"`
	EmailAddresses        []EmailAddress `json:"email_addresses"`
}

type EmailAddress struct {
	ID           string `json:"id"`
	EmailAddress string `json:"email_address"`
}
