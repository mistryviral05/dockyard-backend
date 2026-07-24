package user

import "strings"

func (c ClerkUserData) ToCreateUserDto() *CreateUserDto {
	var email string

	for _, e := range c.EmailAddresses {
		if e.ID == c.PrimaryEmailAddressID {
			email = e.EmailAddress
			break
		}
	}

	name := strings.TrimSpace(c.FirstName + " " + c.LastName)

	return &CreateUserDto{
		ClerkId: c.ID,
		Name:    name,
		Email:   email,
	}

}
