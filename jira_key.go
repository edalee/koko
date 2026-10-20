package main

import "regexp"

// jiraKeyPattern matches a Jira issue key such as ACME-123. koko-worker has
// its own copy in cmd/koko-worker/jobs.go, because it is a separate module.
var jiraKeyPattern = regexp.MustCompile(`\b[A-Z][A-Z0-9]+-\d+\b`)

// firstJiraKey returns the first Jira key found in the texts, taken in order,
// or "".
func firstJiraKey(texts ...string) string {
	for _, t := range texts {
		if k := jiraKeyPattern.FindString(t); k != "" {
			return k
		}
	}
	return ""
}
