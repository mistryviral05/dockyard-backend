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
}
