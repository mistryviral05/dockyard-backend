package middleware

import (
	"net/http"

	"github.com/rs/cors"
)

func CORS(origins []string) func(http.Handler) http.Handler {

	c := cors.New(cors.Options{
		AllowedOrigins: origins,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
			http.MethodPatch,
		},
		AllowedHeaders: []string{
			"Authorization",
			"Content-Type",
			"X-Requested-With",
			"Accept",
		},
	})
	return c.Handler
}
