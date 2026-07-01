# Frorage

MVP for private cloud storage with a Go backend, React web app, shared TypeScript SDK, S3-compatible object storage, and server-managed encryption.

## Repo Layout

- `apps/api` - Go HTTP API for auth, encrypted file metadata, object storage, admin recovery, and usage events.
- `apps/web` - React + Vite browser app.
- `packages/sdk` - Shared TypeScript SDK for API calls and password verifier helpers.
- `docs/openapi.yaml` - Public API contract for web, mobile, CLI, or desktop clients.
- `docker-compose.yml` - Optional local Postgres + MinIO dependencies.

## Privacy Model

Frorage is server-managed encrypted storage, not strict zero-knowledge storage. Users sign in with email/password; the server owns encrypted account master keys and can decrypt files for normal preview/download and required admin recovery.

The object store only receives encrypted bytes. The API keeps the user-to-storage-prefix mapping, encrypted account keys, file/folder records, and encrypted metadata.

## Local Development

Without Docker, run MinIO locally, then start:

```bash
cd apps/api
go run ./cmd/server
```

In another terminal:

```bash
cd /Users/shivanshk/Documents/pdev/frorage
npm run dev:web
```

The API defaults are configured for MinIO at `http://localhost:9000` with bucket `frorage`.
