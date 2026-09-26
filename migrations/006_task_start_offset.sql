ALTER TABLE tasks
    ADD COLUMN start_offset_minutes SMALLINT NOT NULL DEFAULT 180
    CHECK (start_offset_minutes BETWEEN -840 AND 840);
