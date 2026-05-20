# Bot Handoff Guide

This file is the fastest orientation path for another coding agent picking up `frorage`.

## Product Intent

`frorage` is a private cloud storage MVP: cheap object-storage-backed files, a decoupled Go API, a React web client, and client-side end-to-end encryption. The backend should be usable by future browser, mobile, desktop, or CLI clients through the HTTP API.

V1 scope:

- private files and folders only;
- no sharing;
- no native mobile app yet;
- platform-owned object storage;
- S3-compatible storage first;
- Azure Blob later through a separate adapter;
- usage metering with admin-configured provider costs;
- strict E2EE with recovery phrase and recovery file.

## Key Architecture Decisions

- The backend never receives plaintext file bytes, filenames, folder names, or account master keys.
- The TypeScript SDK owns encryption, recovery wrapping, metadata encryption, and client-side file encryption.
- The Go API owns auth, opaque encrypted metadata records, quotas, usage events, and presigned object-storage URLs.
- Storage is behind an `ObjectStore` interface. The first implementation is S3-compatible SigV4 presigning.
- Password reset only restores account login. Old files require recovery phrase/file to unwrap the existing account master key.
- Current persistence is in-memory for local iteration. The Postgres migration is the production schema target.
- Billing is currently internal metering plus provider cost tables. Stripe is planned, not fully implemented.

## File Map

### Root

- `README.md` - human-facing overview, repo layout, privacy model, and local dev commands.
- `.gitignore` - ignores local env files, key material, build output, coverage, and accidental Go binaries.
- `.env.example` - root-level environment reference.
- `go.work` - Go workspace including `apps/api`.
- `package.json` - npm workspace root for `apps/web` and `packages/sdk`.
- `docker-compose.yml` - optional local Postgres + MinIO dependencies, plus MinIO bucket initialization.

### Docs

- `docs/architecture.md` - conceptual architecture boundaries: backend, storage, E2EE, persistence.
- `docs/openapi.yaml` - public HTTP API contract for web/mobile/CLI clients.
- `docs/bot-handoff.md` - this file; implementation map and agent handoff notes.

### Backend: `apps/api`

- `apps/api/cmd/server/main.go` - API entrypoint; loads config, creates repository/object-store implementations, starts HTTP server.
- `apps/api/internal/config` - environment-driven API, token, quota, TTL, and S3-compatible storage config.
- `apps/api/internal/httpapi` - HTTP routes and handlers for auth, recovery, files, uploads, downloads, and usage.
- `apps/api/internal/store` - repository interface, in-memory repository, models, ID generation, and store tests.
- `apps/api/internal/objectstore` - object-storage abstraction and S3-compatible presigned URL implementation.
- `apps/api/internal/auth` - HMAC bearer token signing and verification.
- `apps/api/internal/billing` - usage-event summarization and margin calculation.
- `apps/api/migrations/001_initial.sql` - intended Postgres schema for users, files, uploads, usage events, and provider costs.
- `apps/api/Dockerfile` - container build for the Go API.
- `apps/api/.env.example` - backend-specific environment variables.

### SDK: `packages/sdk`

- `packages/sdk/src/crypto.ts` - account master key creation, password wrapping, recovery phrase/file wrapping, metadata encryption, file byte encryption, and recovery rewrap helpers.
- `packages/sdk/src/api.ts` - browser/client API wrapper and upload/download orchestration.
- `packages/sdk/src/types.ts` - shared TypeScript types for key bundles, recovery kits, files, and auth responses.
- `packages/sdk/src/encoding.ts` - base64/base64url/UTF-8/byte helpers.
- `packages/sdk/src/crypto.test.ts` - crypto and recovery behavior tests.
- `packages/sdk/package.json` - SDK package metadata and build/test scripts.

### Web App: `apps/web`

- `apps/web/src/main.tsx` - React app: signup, login, recovery kit display, folder creation, encrypted upload, and encrypted file listing.
- `apps/web/src/styles.css` - app styling.
- `apps/web/vite.config.ts` - Vite dev server config.
- `apps/web/.env.example` - web-specific API base URL.

## Current Run Modes

Without Docker:

```bash
cd /Users/shivanshk/Documents/pdev/frorage/apps/api
go run ./cmd/server
```

In another terminal:

```bash
cd /Users/shivanshk/Documents/pdev/frorage
npm install
npm run dev:web
```

Uploads require an S3-compatible object store at the configured endpoint. Without Docker, install/run MinIO locally or point the API at a real S3-compatible bucket.

With Docker available:

```bash
cd /Users/shivanshk/Documents/pdev/frorage
docker compose up -d
```

## Verification Commands

Backend:

```bash
cd /Users/shivanshk/Documents/pdev/frorage/apps/api
go test ./...
go build -o /tmp/frorage-api ./cmd/server
```

SDK/web, when npm is available:

```bash
cd /Users/shivanshk/Documents/pdev/frorage
npm install
npm -w @frorage/sdk test
npm run build
```

## Known Gaps / Next Best Tasks

- Implement the Postgres repository behind the existing `store.Repository` interface.
- Add real Stripe metered billing export from usage summaries.
- Add password-reset email flow; current recovery endpoint assumes the client has already recovered and rewrapped key material.
- Improve auth verifier design before production. Current auth avoids plaintext passwords but is still a simple verifier flow, not a full PAKE/OPAQUE design.
- Add object deletion to the storage layer when file records are deleted.
- Add multipart upload support for large files.
- Add Playwright end-to-end tests once the web toolchain is installed.
- Add passkey/trusted-device recovery later without changing the master-key hierarchy.

## Security Notes For Future Agents

- Do not send plaintext filenames, folder names, file bytes, or account master keys to the backend.
- Keep `.env`, private keys, and recovery files out of Git.
- Treat `docs/openapi.yaml` as the client contract and update it with API changes.
- Keep Azure support as a separate storage adapter; do not force Azure Blob into the S3-compatible path.
