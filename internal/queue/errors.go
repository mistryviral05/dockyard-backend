package queue

import "errors"

var (
	errNoDockerfile  = errors.New("no Dockerfile found in the build context")
	errRegistryLogin = errors.New("could not sign in to the container registry")
)

// buildFailed docker ke exit code ke saath wo line bhi le kar chalta hai jo
// build ke girne ki asli wajah thi. Reason khali bhi ho sakta hai - har failure
// apni wajah print nahi karti.
type buildFailed struct {
	Reason string
	err    error
}

func (e *buildFailed) Error() string {
	if e.Reason == "" {
		return e.err.Error()
	}
	return e.Reason + " (" + e.err.Error() + ")"
}

func (e *buildFailed) Unwrap() error { return e.err }
