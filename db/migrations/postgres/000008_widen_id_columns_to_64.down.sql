-- Revert id primary keys (and their FK columns) back to VARCHAR(32).

-- Domains
ALTER TABLE domains ALTER COLUMN project_id TYPE VARCHAR(32);
ALTER TABLE domains ALTER COLUMN id         TYPE VARCHAR(32);

-- Deployments
ALTER TABLE deployment_logs ALTER COLUMN deployment_id TYPE VARCHAR(32);
ALTER TABLE deployment_logs ALTER COLUMN id            TYPE VARCHAR(32);
ALTER TABLE deployment_jobs ALTER COLUMN deployment_id TYPE VARCHAR(32);
ALTER TABLE deployment_jobs ALTER COLUMN id            TYPE VARCHAR(32);
ALTER TABLE deployments     ALTER COLUMN triggered_by  TYPE VARCHAR(32);
ALTER TABLE deployments     ALTER COLUMN agent_id      TYPE VARCHAR(32);
ALTER TABLE deployments     ALTER COLUMN project_id    TYPE VARCHAR(32);
ALTER TABLE deployments     ALTER COLUMN id            TYPE VARCHAR(32);

-- Agents
ALTER TABLE project_agent ALTER COLUMN agent_id   TYPE VARCHAR(32);
ALTER TABLE project_agent ALTER COLUMN project_id TYPE VARCHAR(32);
ALTER TABLE agent_metrics ALTER COLUMN agent_id   TYPE VARCHAR(32);
ALTER TABLE agent_metrics ALTER COLUMN id         TYPE VARCHAR(32);
ALTER TABLE agent_tokens  ALTER COLUMN agent_id   TYPE VARCHAR(32);
ALTER TABLE agent_tokens  ALTER COLUMN id         TYPE VARCHAR(32);
ALTER TABLE agents        ALTER COLUMN user_id    TYPE VARCHAR(32);
ALTER TABLE agents        ALTER COLUMN id         TYPE VARCHAR(32);

-- GitHub integration
ALTER TABLE webhook_events       ALTER COLUMN repository_id   TYPE VARCHAR(32);
ALTER TABLE webhook_events       ALTER COLUMN id              TYPE VARCHAR(32);
ALTER TABLE repositories         ALTER COLUMN project_id      TYPE VARCHAR(32);
ALTER TABLE repositories         ALTER COLUMN installation_id TYPE VARCHAR(32);
ALTER TABLE repositories         ALTER COLUMN id              TYPE VARCHAR(32);
ALTER TABLE github_installations ALTER COLUMN user_id         TYPE VARCHAR(32);
ALTER TABLE github_installations ALTER COLUMN id              TYPE VARCHAR(32);

-- Projects / Envs
ALTER TABLE envs     ALTER COLUMN project_id TYPE VARCHAR(32);
ALTER TABLE envs     ALTER COLUMN id         TYPE VARCHAR(32);
ALTER TABLE projects ALTER COLUMN wsp_id     TYPE VARCHAR(32);
ALTER TABLE projects ALTER COLUMN id         TYPE VARCHAR(32);

-- Users / Workspaces
ALTER TABLE workspaces ALTER COLUMN user_id TYPE VARCHAR(32);
ALTER TABLE workspaces ALTER COLUMN id      TYPE VARCHAR(32);
ALTER TABLE users      ALTER COLUMN id      TYPE VARCHAR(32);
