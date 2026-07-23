package router

import (
	"net/http"

	"github.com/mistryviral05/dockyard/internal/http/modules/user"
)

type Modules struct {
	User *user.Handler
}

func New(m Modules) http.Handler {
	mux := http.NewServeMux()
	user.RegisterRoutes(mux, m.User)

	return mux
}
