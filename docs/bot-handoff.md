# Bot Handoff Guide

This file is the fastest orientation path for another coding agent picking up `frorage`.

## Product Intent

`frorage` is a private cloud storage MVP: cheap object-storage-backed files, a decoupled Go API, a React web client, server-managed encryption, and required admin recovery.

V1 scope:

- private files and folders only;
- no sharing;
- no native mobile app yet;
- platform-owned S3-compatible object storage first;
- usage metering with admin-configured provider costs;
- server-side recovery for every user.

## Key Architecture Decisions

- Frorage is not strict zero-knowledge storage.
- Users do not receive recovery keys, recovery files, or account master keys.
- The Go API owns account master keys, encrypted at rest with `MASTER_KEY_ENCRYPTION_SECRET`.
- The API encrypts plaintext uploads before writing to object storage.
- The API decrypts bytes and metadata for normal preview/download and admin recovery.
- The TypeScript SDK is now an API client plus password verifier helper, not the encryption owner.
- The server stores the email/user-to-storage-prefix mapping.
- Current persistence is in-memory for local iteration. The Postgres migration is the production schema target.

## File Map

- `README.md` - human-facing overview, repo layout, privacy model, and local dev commands.
- `docs/architecture.md` - conceptual architecture boundaries.
- `docs/flows/storage-and-encryption.md` - storage/encryption Mermaid flow diagrams.
- `docs/openapi.yaml` - public HTTP API contract.
- `apps/api/internal/httpapi` - auth, files, upload, preview/download, admin recovery, and usage routes.
- `apps/api/internal/store` - repository interface, in-memory repository, models, ID generation, and store tests.
- `apps/api/internal/cryptoutil` - AES-GCM helpers for account-key and object encryption.
- `apps/api/internal/objectstore` - object-storage abstraction, S3-compatible implementation, and memory test store.
- `packages/sdk/src/api.ts` - browser/client API wrapper.
- `packages/sdk/src/crypto.ts` - password verifier helper.
- `apps/web/src/main.tsx` - React app plus `/admin` recovery page.

## Current Run Modes

Without Docker:

```bash
cd /Users/shivanshk/Documents/pdev/frorage/apps/api
go run ./cmd/server
```

In another terminal:

```bash
cd /Users/shivanshk/Documents/pdev/frorage
npm run dev:web
```

Uploads require an S3-compatible object store at the configured endpoint. Without Docker, install/run MinIO locally or point the API at a real S3-compatible bucket.

Admin recovery page:

```text
http://localhost:5173/admin
```

Use `ADMIN_TOKEN` from the API environment.

## Verification Commands

Backend:

```bash
cd /Users/shivanshk/Documents/pdev/frorage/apps/api
go test ./...
```

SDK/web:

```bash
cd /Users/shivanshk/Documents/pdev/frorage
npm -w @frorage/sdk test
npm run build
```

## Known Gaps / Next Best Tasks

- Implement durable SQLite or Postgres repository behind `store.Repository`.
- Add object deletion to the storage layer when file records are deleted.
- Replace MVP reset-token response with real email delivery.
- Improve auth verifier design before production.
- Add multipart/chunked upload support for large files.
- Add Playwright end-to-end tests for user vault and admin recovery.

## Security Notes For Future Agents

- Do not store account master keys raw in the database.
- Keep `MASTER_KEY_ENCRYPTION_SECRET`, `ADMIN_TOKEN`, `.env`, and MinIO credentials out of Git.
- Treat admin recovery APIs as sensitive production surfaces.
