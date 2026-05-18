# Architecture

## Backend Boundary

The Go API is intentionally client-agnostic. Browser, mobile, desktop, or CLI clients only need the HTTP API in `docs/openapi.yaml`.

The backend owns:

- account records and auth tokens;
- opaque encrypted metadata records;
- quota and usage accounting;
- presigned object-store upload/download URLs;
- provider cost tables for metered billing.

The backend does not own plaintext file bytes, filenames, folder names, or account master keys.

## Storage Boundary

`ObjectStore` is the storage port. The first adapter generates S3-compatible SigV4 presigned URLs and works with MinIO, AWS S3, Cloudflare R2, Backblaze B2, Oracle Object Storage, and other S3-compatible providers.

Azure Blob should be added as a second adapter because it uses its own native API rather than the S3 API shape.

## E2EE Boundary

The TypeScript SDK owns encryption:

- generate account master key;
- derive a password wrapping key;
- wrap the master key for password unlock;
- wrap the same master key for recovery phrase and recovery file unlock;
- encrypt file bytes and metadata before upload;
- decrypt file bytes and metadata after download.

Password reset can update login credentials immediately, but old files are only recoverable when the client can unwrap the existing master key with a recovery phrase or recovery file.

## Persistence

The current Go repository implementation is in-memory for fast local iteration and tests. The migration in `apps/api/migrations/001_initial.sql` is the Postgres schema target for the production repository implementation.
