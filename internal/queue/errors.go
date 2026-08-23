package queue

import "errors"

var (
	errNoDockerfile  = errors.New("no Dockerfile found in the build context")
	errRegistryLogin = errors.New("could not sign in to the container registry")
)
