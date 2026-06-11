CREATE TABLE entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    entry_id TEXT NOT NULL,
    encrypted_data BYTEA NOT NULL,
    encrypted_meta BYTEA,
    version BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, entry_id)
);
CREATE INDEX idx_entries_user_version ON entries(user_id, version);