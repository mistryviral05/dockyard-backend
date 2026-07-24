-- Widen id primary keys (and the FK columns that reference them) from
-- VARCHAR(32) to VARCHAR(64). Non-id VARCHAR(32) columns (account_type,
-- repository_selection, trigger_source, ssl_status) are left unchanged.

-- Users / Workspaces
ALTER TABLE users      ALTER COLUMN id      TYPE VARCHAR(64);
ALTER TABLE workspaces ALTER COLUMN id      TYPE VARCHAR(64);
ALTER TABLE workspaces ALTER COLUMN user_id TYPE VARCHAR(64);

-- Projects / Envs
ALTER TABLE projects ALTER COLUMN id         TYPE VARCHAR(64);
ALTER TABLE projects ALTER COLUMN wsp_id     TYPE VARCHAR(64);
ALTER TABLE envs     ALTER COLUMN id         TYPE VARCHAR(64);
ALTER TABLE envs     ALTER COLUMN project_id TYPE VARCHAR(64);

-- GitHub integration
ALTER TABLE github_installations ALTER COLUMN id              TYPE VARCHAR(64);
ALTER TABLE github_installations ALTER COLUMN user_id         TYPE VARCHAR(64);
ALTER TABLE repositories         ALTER COLUMN id              TYPE VARCHAR(64);
ALTER TABLE repositories         ALTER COLUMN installation_id TYPE VARCHAR(64);
ALTER TABLE repositories         ALTER COLUMN project_id      TYPE VARCHAR(64);
ALTER TABLE webhook_events       ALTER COLUMN id              TYPE VARCHAR(64);
ALTER TABLE webhook_events       ALTER COLUMN repository_id   TYPE VARCHAR(64);

-- Agents
ALTER TABLE agents        ALTER COLUMN id         TYPE VARCHAR(64);
ALTER TABLE agents        ALTER COLUMN user_id    TYPE VARCHAR(64);
ALTER TABLE agent_tokens  ALTER COLUMN id         TYPE VARCHAR(64);
ALTER TABLE agent_tokens  ALTER COLUMN agent_id   TYPE VARCHAR(64);
ALTER TABLE agent_metrics ALTER COLUMN id         TYPE VARCHAR(64);
ALTER TABLE agent_metrics ALTER COLUMN agent_id   TYPE VARCHAR(64);
ALTER TABLE project_agent ALTER COLUMN project_id TYPE VARCHAR(64);
ALTER TABLE project_agent ALTER COLUMN agent_id   TYPE VARCHAR(64);

-- Deployments
ALTER TABLE deployments     ALTER COLUMN id            TYPE VARCHAR(64);
ALTER TABLE deployments     ALTER COLUMN project_id    TYPE VARCHAR(64);
ALTER TABLE deployments     ALTER COLUMN agent_id      TYPE VARCHAR(64);
ALTER TABLE deployments     ALTER COLUMN triggered_by  TYPE VARCHAR(64);
ALTER TABLE deployment_jobs ALTER COLUMN id            TYPE VARCHAR(64);
ALTER TABLE deployment_jobs ALTER COLUMN deployment_id TYPE VARCHAR(64);
ALTER TABLE deployment_logs ALTER COLUMN id            TYPE VARCHAR(64);
ALTER TABLE deployment_logs ALTER COLUMN deployment_id TYPE VARCHAR(64);

-- Domains
ALTER TABLE domains ALTER COLUMN id         TYPE VARCHAR(64);
ALTER TABLE domains ALTER COLUMN project_id TYPE VARCHAR(64);
