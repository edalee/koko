package main

import (
	"regexp"
	"strings"
)

// jiraKeyPattern matches a Jira issue key such as ACME-123: a project of 2 to
// 10 letters, a dash and a number. koko-worker has its own looser copy in
// cmd/koko-worker/jobs.go, because it is a separate module.
var jiraKeyPattern = regexp.MustCompile(`\b[A-Z]{2,10}-\d+\b`)

// notJiraProjects are common technical terms shaped like a key, such as
// UTF-8, SHA-256 and GPT-4. They are never a ticket.
var notJiraProjects = map[string]bool{
	"UTF": true, "SHA": true, "ISO": true, "GPT": true, "CVE": true, "RFC": true,
	"HTTP": true, "TLS": true, "SSL": true, "MD": true, "AES": true, "RSA": true,
	"ECMA": true, "UTC": true, "GMT": true, "PEP": true, "IEEE": true, "ES": true,
	"IE": true, "X": true, "H": true,
}

// firstJiraKey returns the first Jira key found in the texts, taken in order,
// or "".
func firstJiraKey(texts ...string) string {
	for _, t := range texts {
		for _, k := range jiraKeyPattern.FindAllString(t, -1) {
			project, _, _ := strings.Cut(k, "-")
			if !notJiraProjects[project] {
				return k
			}
		}
	}
	return ""
}
