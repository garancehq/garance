CREATE SCHEMA IF NOT EXISTS garance_storage;

CREATE TABLE garance_storage.buckets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    max_file_size BIGINT, -- in bytes, NULL = no limit
    allowed_mime_types TEXT[], -- NULL = all types allowed
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE garance_storage.files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bucket_id UUID NOT NULL REFERENCES garance_storage.buckets(id) ON DELETE CASCADE,
    name TEXT NOT NULL, -- full path within bucket, e.g. "user-123/photo.jpg"
    size BIGINT NOT NULL,
    mime_type TEXT NOT NULL,
    owner_id UUID, -- user who uploaded the file
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (bucket_id, name)
);

CREATE INDEX idx_files_bucket_id ON garance_storage.files(bucket_id);
CREATE INDEX idx_files_owner_id ON garance_storage.files(owner_id);
