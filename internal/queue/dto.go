package queue

import (
	"github.com/mistryviral05/dockyard/internal/http/modules/deployment"
	"github.com/mistryviral05/dockyard/internal/http/utils/gh"
)

type BuildSource struct {
	Dir        string
	Commit     *gh.Commit
	Build      *deployment.BuildContext
	Dockerfile string

	// nixpacks ke generate kiye Dockerfile ko chahiye plan variables
	// (NIXPACKS_UV_VERSION jaise). Repo ka apna Dockerfile ho to nil.
	BuildArgs map[string]string
}
