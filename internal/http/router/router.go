package router

import (
	"net/http"

	"github.com/mistryviral05/dockyard/internal/http/modules/agent"
	"github.com/mistryviral05/dockyard/internal/http/modules/deployment"
	"github.com/mistryviral05/dockyard/internal/http/modules/github"
	"github.com/mistryviral05/dockyard/internal/http/modules/project"
	"github.com/mistryviral05/dockyard/internal/http/modules/realtime"
	"github.com/mistryviral05/dockyard/internal/http/modules/user"
	"github.com/mistryviral05/dockyard/internal/http/modules/workspace"
	"github.com/mistryviral05/dockyard/internal/http/utils/response"
)

type Modules struct {
	User       *user.Handler
	Workspace  *workspace.Handler
	Auth       func(http.Handler) http.Handler // clerk auth middleware
	Github     *github.Handler
	Agent      *agent.Handler
	Project    *project.Handler
	Deployment *deployment.Handler
	Realtime   *realtime.Handler
}

func New(m Modules) http.Handler {
	mux := http.NewServeMux()
	const base = "/api"

	mux.HandleFunc("GET /api/helthz", func(w http.ResponseWriter, r *http.Request) {
		response.WriteJson(w, http.StatusOK, map[string]string{
			"status": "helthy",
		})
	})

	user.RegisterRoutes(mux, m.User, base)
	workspace.RegisterRoutes(mux, m.Workspace, m.Auth, base)
	github.RegisterRoutes(mux, m.Github, m.Auth, base)
	agent.RegisterRoutes(mux, m.Agent, m.Auth, base)
	project.RegisterRoutes(mux, m.Project, m.Auth, base)
	deployment.RegisterRoutes(mux, m.Deployment, m.Auth, base)
	realtime.RegisterRoutes(mux, m.Realtime, m.Auth, base)
	return mux
}
