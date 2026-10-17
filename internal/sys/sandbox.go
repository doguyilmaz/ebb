package sys

import "errors"

var ErrSandbox = errors.New("not run in sandbox")

var sandboxAllowed = map[string]bool{"plutil": true, "sw_vers": true}

func Sandboxed(env Env) bool { return env.Getenv(SandboxEnv) == "1" }

// SandboxRun refuses, in a sandboxed session, every external tool that could
// change the machine or make results depend on it.
func SandboxRun(env Env, name string) error {
	if Sandboxed(env) && !sandboxAllowed[name] {
		return ErrSandbox
	}
	return nil
}
