# Architecture

## Backend Boundary

The Go API is the trusted encryption boundary for Frorage. Browser, mobile, desktop, or CLI clients use the HTTP API and do not receive account master keys.

The backend owns:

- account records and auth tokens;
- encrypted account master keys;
- plaintext upload/download handling over HTTPS;
- file/folder metadata encryption and decryption;
- object-store reads and writes;
- quota and usage accounting;
- admin recovery APIs.

Because server-side recovery is required, Frorage is **not** a strict zero-knowledge system. The server can decrypt user files for normal downloads, previews, and admin recovery.

## Storage Boundary

`ObjectStore` is the storage port. The first adapter uses S3-compatible SigV4 URLs internally and works with MinIO and other S3-compatible providers.

MinIO/S3 stores encrypted object bytes under server-generated object keys. Users see files and folders through API metadata, not bucket paths.

## Encryption Boundary

Each user has a random account master key created by the API at signup. The account master key is encrypted at rest with `MASTER_KEY_ENCRYPTION_SECRET`.

The API decrypts the account master key when it needs to:

- encrypt uploaded file bytes and metadata;
- decrypt metadata for file lists;
- decrypt file bytes for preview/download;
- support admin recovery.

Users only need email/password for login. Forgot password resets login credentials; it does not rotate or recover user-held keys because users do not hold file recovery keys.

## Persistence

The current Go repository implementation is in-memory for fast local iteration and tests. The migration in `apps/api/migrations/001_initial.sql` is the Postgres schema target for the production repository implementation.
