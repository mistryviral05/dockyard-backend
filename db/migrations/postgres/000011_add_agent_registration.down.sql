ALTER TABLE agents
    DROP COLUMN IF EXISTS signing_secret,
    DROP COLUMN IF EXISTS registered_at;
