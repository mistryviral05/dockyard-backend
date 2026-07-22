-- Agents: registration, tokens, metrics, project assignment.

CREATE TABLE agents (
    id                 VARCHAR(32) PRIMARY KEY,
    user_id            VARCHAR(32) NOT NULL,
    agent_name         VARCHAR(255),
    public_ip          VARCHAR(64),
    port               VARCHAR(8)  NOT NULL DEFAULT '8999',
    subdomain          VARCHAR(255),              -- <agent>.viralmistry.dev, set at registration
    status             agent_status NOT NULL DEFAULT 'pending',
    temp_token         TEXT,                      -- one-time registration token
    token_exp          TIMESTAMPTZ,
    last_heartbeat_at  TIMESTAMPTZ,
    os_info            TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_agents_subdomain UNIQUE (subdomain),
    CONSTRAINT fk_agents_user_id FOREIGN KEY (user_id)
        REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX idx_agents_user_id ON agents (user_id);
CREATE INDEX idx_agents_status  ON agents (status);

CREATE TABLE agent_tokens (
    id          VARCHAR(32) PRIMARY KEY,
    agent_id    VARCHAR(32) NOT NULL,
    token       TEXT        NOT NULL,
    type        token_type  NOT NULL,
    expires_at  TIMESTAMPTZ,               -- NULL for non-expiring long-lived JWT
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_agent_tokens_token UNIQUE (token),
    CONSTRAINT fk_agent_tokens_agent_id FOREIGN KEY (agent_id)
        REFERENCES agents (id) ON DELETE CASCADE
);

CREATE INDEX idx_agent_tokens_agent_id ON agent_tokens (agent_id);

CREATE TABLE agent_metrics (
    id           VARCHAR(32) PRIMARY KEY,
    agent_id     VARCHAR(32) NOT NULL,
    cpu_usage    DOUBLE PRECISION,
    ram_usage    DOUBLE PRECISION,
    disk_usage   DOUBLE PRECISION,
    recorded_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_agent_metrics_agent_id FOREIGN KEY (agent_id)
        REFERENCES agents (id) ON DELETE CASCADE
);

-- time-series lookup: latest metrics per agent
CREATE INDEX idx_agent_metrics_agent_id_recorded_at ON agent_metrics (agent_id, recorded_at);

CREATE TABLE project_agent (
    project_id  VARCHAR(32) NOT NULL,
    agent_id    VARCHAR(32) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT pk_project_agent PRIMARY KEY (project_id, agent_id),
    CONSTRAINT fk_project_agent_project_id FOREIGN KEY (project_id)
        REFERENCES projects (id) ON DELETE CASCADE,
    CONSTRAINT fk_project_agent_agent_id FOREIGN KEY (agent_id)
        REFERENCES agents (id) ON DELETE CASCADE
);

-- reverse lookup: which projects run on an agent
CREATE INDEX idx_project_agent_agent_id ON project_agent (agent_id);
