package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
)

type GeneralErrorResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

type GeneralResponse struct {
	Status  string      `json:"status"`
	Message interface{} `json:"message"`
}

const (
	StatusSuccess = "success"
	StatusError   = "error"
)

func WriteJson(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

func GeneralError(err error) GeneralErrorResponse {
	return GeneralErrorResponse{
		Status: StatusError,
		Error:  err.Error(),
	}
}

func GeneralMessage(msg string) GeneralResponse {
	return GeneralResponse{
		Status:  StatusSuccess,
		Message: msg,
	}
}

func ValidationError(errs validator.ValidationErrors) GeneralErrorResponse {
	var errorMsgs []string

	for _, err := range errs {
		switch err.ActualTag() {
		case "required":
			errorMsgs = append(errorMsgs, fmt.Sprintf("%s is required", err.Field()))
		default:
			errorMsgs = append(errorMsgs, fmt.Sprintf("%s is not valid", err.Field()))
		}
	}

	return GeneralErrorResponse{
		Status: StatusError,
		Error:  strings.Join(errorMsgs, ", "),
	}
}
