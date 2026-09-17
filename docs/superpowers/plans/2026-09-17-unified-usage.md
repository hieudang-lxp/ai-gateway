# Unified Usage Implementation Plan

**Goal:** Run the existing gateway under Docker with continuous Claude Code, Codex and Cursor collection across IDEs.
**Architecture:** Read-only host collectors + authenticated Cursor usage polling + separate usage ledger + local summary API and React dashboard.
**Tech stack:** Go, SQLite, React, TypeScript, Docker Compose. No additional runtime dependencies.
**Spec:** docs/superpowers/specs/2026-09-17-unified-usage-design.md

- [x] Add parser/accounting tests for exclusive token categories, response deduplication, mixed legacy/direct Codex records, Cursor account scoping and HTTP pagination.
- [x] Add store/usage.go with nullable costs, atomic idempotent import and unified aggregates; preserve proxy budgets.
- [x] Implement Codex/Claude JSONL collectors and Cursor session/API collector. Read only metadata and allowed session keys, surface errors, poll continuously.
- [x] Add GET /_usage and /healthz, collector path/interval flags, and /dashboard/ static serving.
- [x] Add source statuses, coverage, unknown-cost notices and model/token breakdown to the existing frontend.
- [x] Add Dockerfile.local, compose.yaml and restricted build context; move public dependency resolutions off the private Nexus registry for reproducible builds.
- [x] Run backend and frontend verification, validate real data with read-only Docker mounts, back up the database, disable old launchd service, and run Docker on 8788.
