CREATE TABLE users (
  id TEXT PRIMARY KEY,
  email TEXT UNIQUE NOT NULL,
  password_verifier TEXT NOT NULL,
  password_wrapped_master_key TEXT NOT NULL,
  password_kdf_salt TEXT NOT NULL,
  recovery_phrase_wrapped_master_key TEXT NOT NULL,
  recovery_file_wrapped_master_key TEXT NOT NULL,
  quota_bytes BIGINT NOT NULL,
  used_bytes BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE files (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind TEXT NOT NULL CHECK (kind IN ('file', 'folder')),
  parent_id TEXT NULL REFERENCES files(id) ON DELETE SET NULL,
  encrypted_metadata TEXT NOT NULL,
  ciphertext_size BIGINT NOT NULL DEFAULT 0,
  object_key TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE upload_sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  parent_id TEXT NULL REFERENCES files(id) ON DELETE SET NULL,
  encrypted_metadata TEXT NOT NULL,
  ciphertext_size BIGINT NOT NULL,
  object_key TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  committed BOOLEAN NOT NULL DEFAULT false
);

CREATE TABLE usage_events (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  metric TEXT NOT NULL,
  quantity BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE provider_costs (
  provider TEXT PRIMARY KEY,
  storage_gb_month_micros BIGINT NOT NULL,
  egress_gb_micros BIGINT NOT NULL,
  operation_10k_micros BIGINT NOT NULL,
  margin_bps BIGINT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO provider_costs (
  provider,
  storage_gb_month_micros,
  egress_gb_micros,
  operation_10k_micros,
  margin_bps
) VALUES (
  's3-compatible',
  18000,
  90000,
  4000,
  300
);
