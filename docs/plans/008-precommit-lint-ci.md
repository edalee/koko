# Plan 008: Pre-commit Hooks, Linting, and CI

## Context
Koko has no pre-commit hooks, no frontend linting, and no CI. Adding quality gates before commits to catch issues early.

## Approach
- **lefthook** for pre-commit hooks (Go-based, fast, simple YAML)
- **Biome** for frontend lint + format (single tool, fast, replaces ESLint+Prettier)
- **golangci-lint** for Go linting (already in Makefile)
- **tsc --noEmit** for TypeScript type checking
- **GitHub Actions** CI workflow

## New Files

### `lefthook.yml`
Pre-commit: runs go-lint, biome check, and tsc in parallel. Commit-msg: validates conventional commits via regex.

### `frontend/biome.json`
Minimal config: recommended rules, 2-space indent, double quotes, 100 line width.

### `.golangci.yml`
Minimal config: errcheck, govet, staticcheck, unused, ineffassign, gosimple.

### `.github/workflows/ci.yml`
Two parallel jobs: `go` (lint + test + build) and `frontend` (biome check + tsc).

## Modified Files

### `.gitignore`
Add `.DS_Store` and `.claude/` entries.

### `frontend/package.json`
Add `@biomejs/biome` devDep + `lint`, `lint:fix`, `typecheck` scripts.

### `Makefile`
Add `check`, `lint-fe`, `setup` targets.

## Verification
- `make check` runs all lints + typecheck
- `git commit -m "bad"` should be rejected by commit-msg hook
- `git commit -m "chore: test"` should pass
