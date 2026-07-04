---
name: "engineering-delivery"
description: "Delivers code in an engineering-ready way. Invoke when users ask to create, scaffold, modify, or complete runnable projects, services, scripts, or features."
---

# Engineering Delivery

## Purpose

This skill makes coding tasks default to an engineering-ready delivery standard instead of a "single file plus instructions" outcome.

Use it when the user asks to:

- create a new project, service, API, script, app, or website
- scaffold or implement a feature that should run in the current workspace
- generate code that should include dependencies, config, structure, and validation
- finish a partially-created project and make it runnable

## Core Rules

- Prefer runnable project structure over a single demo file.
- Reuse existing repository conventions when the workspace already has structure.
- When the workspace is empty or incomplete, create the minimum reasonable layout for the target language/framework.
- After writing code, continue through initialization, dependency installation, and basic verification when safe and feasible.
- Do not stop at "here are the commands to run" if those commands can be executed safely in the workspace.
- Report exactly which files were created or changed, which commands were run, what passed, and what still failed or remains unverified.

## Delivery Standard

For coding tasks, aim to leave the workspace in a state that is:

- runnable
- dependency-resolved
- minimally validated
- understandable to the user

That usually means producing:

- source files
- dependency manifest
- config or env examples when needed
- reasonable directory structure
- short run/test instructions

## Default Workflow

1. Inspect the workspace structure before creating files.
2. Read existing config or entrypoint files if they likely affect the task.
3. Create or update the necessary project files, not just the main code file.
4. Run safe initialization commands for the target stack.
5. Install or sync dependencies.
6. Run basic verification commands.
7. If verification fails, fix the code or config and retry when practical.
8. Return a concise engineering handoff summary.

## Language Guidance

### Go

- Ensure `go.mod` exists.
- Use `go mod init` when starting a new module.
- Run `go mod tidy`.
- Prefer validating with:
  - `go test ./...`
  - `go vet ./...`
  - `go build ./...`

### Node.js / TypeScript

- Ensure `package.json` exists.
- Use `npm init -y` when appropriate.
- Install declared dependencies.
- Run the best available validation commands, typically:
  - `npm test`
  - `npm run build`

### Python

- Ensure dependency metadata exists, such as `requirements.txt` or `pyproject.toml`.
- Install dependencies when safe.
- Run lightweight validation, such as:
  - `pytest`
  - `python -m compileall .`
  - `python -m py_compile ...`

## Safety Rules

- Avoid destructive commands unless the user explicitly asks for them.
- Avoid long-running foreground processes as part of automatic verification.
- Prefer checks that terminate, such as install, test, lint, vet, or build.
- If a task requires a persistent server, validate as much as possible before handing off the startup step.

## Response Pattern

When finishing a task, summarize:

- files created or updated
- commands executed
- checks that passed
- checks that failed
- remaining manual steps, if any

## Example Outcome

If the user asks for "a simple Gin API service", do not stop after writing `main.go`.

Also aim to:

- create `go.mod` if missing
- install/sync dependencies
- verify with `go test ./...`, `go vet ./...`, and `go build ./...`
- tell the user the result of those commands
