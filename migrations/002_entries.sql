CREATE TABLE entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entry_id VARCHAR(255) NOT NULL,
    encrypted_data BYTEA NOT NULL,
    encrypted_meta BYTEA,
    version BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, entry_id)
);

-- Индекс для быстрого Pull (поиск по версии)
CREATE INDEX idx_entries_user_version ON entries(user_id, version);