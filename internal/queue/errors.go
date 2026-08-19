package queue

import "errors"

var (
	errNoDockerfile = errors.New("no Dockerfile found in the build context")
)
