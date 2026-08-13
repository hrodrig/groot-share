---
phase: 03-identity
plan: 01
subsystem: auth
tags: [sqlite, bcrypt, session, api-key]

requires:
  - phase: 02-process
    provides: store.Open, HTTP mux, slog, fail-closed config
provides:
  - users/sessions/api_keys schema
  - bootstrap admin from GFS_BOOTSTRAP_*
  - login cookie and hashed api_key
affects: [vps-home, housekeeping]

actuals:
  tokens: 0
  tasks: 6
  commits: 1

tech-stack:
  added: [golang.org/x/crypto/bcrypt]
  patterns: [bcrypt passwords, SHA-256 api_key, HttpOnly session cookie]

key-files:
  created:
    - internal/auth/auth.go
    - internal/store/identity.go
    - internal/server/identity.go
  modified:
    - cmd/gfs/main.go
    - internal/store/store.go
    - internal/config/config.go

key-decisions:
  - "Bootstrap env-once, fail closed if empty user table"
  - "bcrypt for passwords; SHA-256 for api_key and session token"
  - "api_key only from Bearer / X-API-Key, never query string"

patterns-established:
  - "authenticate() session or api_key for /v1/* (Phase 4 upload reuses this)"

requirements-completed: [AUTH-01, AUTH-02, AUTH-03]

coverage:
  - id: D1
    description: Login sets HttpOnly cookie; wrong password 401
    requirement: AUTH-01
    verification:
      - kind: unit
        ref: internal/server/identity_test.go#TestLoginSetsCookieAndMe
        status: pass
    human_judgment: false
  - id: D2
    description: api_key shown once, hashed at rest, Bearer works; query string ignored
    requirement: AUTH-02
    verification:
      - kind: unit
        ref: internal/server/identity_test.go#TestAPIKeyShownOnceAndBearerMe
        status: pass
    human_judgment: false
  - id: D3
    description: Logs never contain the password
    requirement: AUTH-03
    verification:
      - kind: unit
        ref: internal/server/identity_test.go#TestLoginDoesNotLogPassword
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-08-12
status: complete
---

# Phase 3: Identity Summary

**Login, bootstrap admin, and hashed api_key work; empty DB without GFS_BOOTSTRAP_* refuses start.**

## Accomplishments

- Schema users/sessions/api_keys; bcrypt; SHA-256 keys
- `EnsureAdmin` fail-closed
- GET/POST `/login`, POST `/logout`, GET `/v1/me`, POST `/v1/api-keys`, POST `/v1/users`
- `make ci` green; cover 74.7% ≥ 60

## Self-Check: PASSED
