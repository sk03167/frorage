# Private Cloud Storage

Greenfield MVP for cheap, private cloud storage with a Go backend, React web app, shared TypeScript SDK, S3-compatible object storage, and client-side end-to-end encryption.

## Repo Layout

- `apps/api` - Go HTTP API for auth, opaque file metadata, billing usage events, and object-store presigned URLs.
- `apps/web` - React + Vite browser app.
- `packages/sdk` - Shared TypeScript SDK for API calls, E2EE key wrapping, metadata encryption, and recovery kits.
- `docs/openapi.yaml` - Public API contract for web, mobile, CLI, or desktop clients.
- `docker-compose.yml` - Local Postgres + MinIO dependencies.

## MVP Privacy Model

The server never receives plaintext file bytes, plaintext filenames, folder names, or the account master key. The SDK generates a random account master key in the browser, wraps it with a password-derived key and recovery secrets, and encrypts metadata before sending it to the backend.

Password reset restores account login. Restoring old encrypted files requires the recovery phrase or recovery file created during signup.

## Local Development

```bash
docker compose up -d
cd apps/api && go test ./...
cd ../../packages/sdk && npm install && npm test
cd ../../apps/web && npm install && npm run dev
```

The API defaults are configured for MinIO at `http://localhost:9000` with bucket `private-cloud`.
