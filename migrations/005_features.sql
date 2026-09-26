ALTER TABLE tasks ADD COLUMN field_values JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE tasks ADD COLUMN field_schema JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE tasks ADD COLUMN latitude DOUBLE PRECISION;
ALTER TABLE tasks ADD COLUMN longitude DOUBLE PRECISION;
ALTER TABLE max_webhook_inbox ADD COLUMN processed_at TIMESTAMPTZ;
ALTER TABLE max_webhook_inbox ADD COLUMN last_error TEXT;
ALTER TABLE task_events ADD COLUMN subject_user_id BIGINT REFERENCES users(id);

CREATE TABLE task_field_definitions (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 category TEXT NOT NULL,
 key TEXT NOT NULL,
 label TEXT NOT NULL,
 type TEXT NOT NULL CHECK (type IN ('text','number','select','date','boolean')),
 required BOOLEAN NOT NULL DEFAULT false,
 options JSONB NOT NULL DEFAULT '[]'::jsonb,
 UNIQUE(company_id,category,key)
);
CREATE TABLE task_ratings (
 task_id BIGINT PRIMARY KEY REFERENCES tasks(id) ON DELETE CASCADE,
 score SMALLINT NOT NULL CHECK (score BETWEEN 1 AND 5),
 comment TEXT NOT NULL CHECK (length(btrim(comment)) BETWEEN 1 AND 2000),
 author_member_id BIGINT NOT NULL REFERENCES company_members(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE task_attachments (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 task_id BIGINT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
 filename TEXT NOT NULL,
 content_type TEXT NOT NULL,
 object_key TEXT NOT NULL UNIQUE,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
