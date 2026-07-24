package response

import (
	"errors"
	"net/http"
)

func NewError(w http.ResponseWriter, s int, m string) error {
	return WriteJson(w, s, GeneralError(errors.New(m)))
}

func InternalError(w http.ResponseWriter, m string) error {
	return WriteJson(w, http.StatusInternalServerError, GeneralError(errors.New(m)))
}

func BadRequest(w http.ResponseWriter, m string) error {
	return WriteJson(w, http.StatusBadRequest, GeneralError(errors.New(m)))
}
