package router

import (
	"net/http"

	"github.com/mistryviral05/dockyard/internal/http/modules/github"
	"github.com/mistryviral05/dockyard/internal/http/modules/user"
	"github.com/mistryviral05/dockyard/internal/http/modules/workspace"
)

type Modules struct {
	User      *user.Handler
	Workspace *workspace.Handler
	Auth      func(http.Handler) http.Handler // clerk auth middleware
	Github    *github.Handler
}

func New(m Modules) http.Handler {
	mux := http.NewServeMux()
	const base = "/api"
	user.RegisterRoutes(mux, m.User, base)
	workspace.RegisterRoutes(mux, m.Workspace, m.Auth, base)
	github.RegisterRoutes(mux, m.Github, m.Auth, base)
	return mux
}
