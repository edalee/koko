package main

import (
	"slices"
	"testing"
)

func TestParentSessionVarsAreDropped(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "abc")
	// A setting of your own stays.
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")

	tm := &TerminalManager{}
	for name, env := range map[string][]string{"cleanEnv": cleanEnv(), "buildEnv": tm.buildEnv()} {
		for _, kv := range env {
			if fromParentSession(kv) {
				t.Errorf("%s kept %s", name, kv)
			}
		}
		if !slices.Contains(env, "CLAUDE_CODE_USE_BEDROCK=1") {
			t.Errorf("%s dropped CLAUDE_CODE_USE_BEDROCK", name)
		}
	}
}
