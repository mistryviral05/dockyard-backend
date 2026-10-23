ALTER TABLE deployments
    DROP COLUMN IF EXISTS url;

ALTER TABLE projects
    DROP COLUMN IF EXISTS app_port;
