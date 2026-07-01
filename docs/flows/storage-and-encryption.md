# Storage and Encryption Flow

This document explains how Frorage stores files, folders, metadata, and server-side recovery material after the server-managed encryption pivot.

## Definitions

- **Browser**: The Frorage web app running on the user's machine. It uploads plaintext over HTTPS and receives plaintext previews/downloads from the API.
- **API**: The trusted Frorage backend. It owns account keys, encrypts/decrypts files, and talks to MinIO/S3.
- **MinIO/S3**: Object storage. It stores encrypted file bytes only.
- **Account master key**: A random per-user secret created by the API at signup. It encrypts that user's files and metadata.
- **Root encryption secret**: Server/admin secret configured as `MASTER_KEY_ENCRYPTION_SECRET`. It encrypts account master keys at rest.
- **Storage prefix**: Server-known bucket prefix for a user, such as `users/user_abc`. It maps email/user id to object storage.
- **Plaintext**: Readable original data, such as a filename or file bytes.
- **Ciphertext**: Encrypted data stored in MinIO/S3 or metadata records.
- **Admin token**: Secret bearer token used for admin recovery APIs.

## Core Idea

Frorage is now server-managed encrypted storage.

- The browser does not hold file encryption keys.
- The API encrypts before storing bytes in MinIO/S3.
- The API decrypts for authenticated preview/download.
- Admin recovery is required for all users and is handled by server-side key custody.

## Signup and Key Setup

```mermaid
sequenceDiagram
  participant Browser
  participant API
  participant Store

  Browser->>API: Signup email and password verifier
  API->>API: Generate account master key
  API->>API: Encrypt master key with root secret
  API->>Store: Store email, verifier, storagePrefix, encrypted key
  API-->>Browser: Return auth token
```

Users do not receive a recovery phrase, recovery file, or account master key.

## Upload Flow

```mermaid
sequenceDiagram
  participant Browser
  participant API
  participant MinIO
  participant Store

  Browser->>API: Upload plaintext file over HTTPS
  API->>API: Decrypt user's account master key
  API->>API: Encrypt file bytes and metadata
  API->>MinIO: Store encrypted bytes under storagePrefix
  API->>Store: Store file record and encrypted metadata
  API-->>Browser: Return file record with plaintext display metadata
```

Object keys are intentionally server-owned:

Example: `frorage/users/<user-id>/objects/<object-id>`

The bucket does not mirror the visible folder tree. Folders are metadata records.

## Folder Flow

```mermaid
flowchart TD
  A["User creates folder"] --> B["API encrypts folder metadata"]
  B --> C["API stores folder record"]
  C --> D["Folder has id, parentId, encryptedMetadata"]
  D --> E["Files and folders reference parentId"]
```

Moving a file or folder changes only the record's `parentId`. The encrypted object bytes usually stay under the same object key.

## Preview and Download Flow

```mermaid
sequenceDiagram
  participant Browser
  participant API
  participant MinIO

  Browser->>API: Request preview or download
  API->>API: Verify auth and ownership
  API->>MinIO: Read encrypted object bytes
  MinIO-->>API: Return encrypted bytes
  API->>API: Decrypt bytes and metadata
  API-->>Browser: Return plaintext bytes
```

The browser renders supported previews for images, videos, and PDFs. Other file types remain download-only.

## Admin Recovery Flow

```mermaid
sequenceDiagram
  participant Admin
  participant API
  participant MinIO

  Admin->>API: Search user by email with admin token
  API-->>Admin: Return user and storagePrefix
  Admin->>API: Browse user's files
  API-->>Admin: Return decrypted display metadata
  Admin->>API: Preview or download file
  API->>MinIO: Read encrypted object
  API->>API: Decrypt with user's account master key
  API-->>Admin: Return plaintext bytes
```

Admin recovery is a product feature, not a hidden zero-knowledge bypass. Frorage operators with valid admin credentials can recover user files.

## Persistence Requirement

The current MVP in-memory API repository is not durable. If the API process restarts, user/account/file metadata is lost even if encrypted blobs still exist in MinIO.

Durable metadata must include:

- users and email-to-user id mapping;
- password verifier;
- storage prefix;
- encrypted account master key and nonce;
- file/folder records;
- parent folder relationships;
- encrypted metadata;
- object keys;
- usage events and password reset records.
