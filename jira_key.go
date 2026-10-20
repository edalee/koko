package main

import (
	"regexp"
	"strings"
)

// jiraKeyPattern matches a Jira issue key such as ACME-123: a project of 2 to
// 10 letters, a dash and a number. firstJiraKey checks the characters either
// side, because \b would count "_" as part of a word and hide CON-184_retry.
// koko-worker has its own, looser pattern in cmd/koko-worker/jobs.go.
var jiraKeyPattern = regexp.MustCompile(`[A-Z]{2,10}-\d+`)

// notJiraProjects are common technical terms shaped like a key, such as
// UTF-8, SHA-256 and GPT-4. They are never a ticket.
var notJiraProjects = map[string]bool{
	"UTF": true, "SHA": true, "ISO": true, "GPT": true, "CVE": true, "RFC": true,
	"HTTP": true, "TLS": true, "SSL": true, "MD": true, "AES": true, "RSA": true,
	"ECMA": true, "UTC": true, "GMT": true, "PEP": true, "IEEE": true, "ES": true,
	"IE": true,
}

// firstJiraKey returns the first Jira key found in the texts, taken in order,
// or "".
func firstJiraKey(texts ...string) string {
	for _, t := range texts {
		for _, m := range jiraKeyPattern.FindAllStringIndex(t, -1) {
			if !keyEdge(t, m[0]-1) || !keyEdge(t, m[1]) {
				continue
			}
			k := t[m[0]:m[1]]
			project, _, _ := strings.Cut(k, "-")
			if !notJiraProjects[project] {
				return k
			}
		}
	}
	return ""
}

// keyEdge reports whether the byte at i can sit beside a key: the text's end,
// or anything but a letter or digit. "_", "/" and "-" all count as edges.
func keyEdge(t string, i int) bool {
	if i < 0 || i >= len(t) {
		return true
	}
	c := t[i]
	letter := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
	digit := c >= '0' && c <= '9'
	return !letter && !digit
}
