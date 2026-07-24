-- Deployments: core entity + job queue + logs.
-- agent_id / triggered_by use ON DELETE SET NULL so deployment history
-- survives agent or user deletion.

CREATE TABLE deployments (
    id              VARCHAR(32) PRIMARY KEY,
    project_id      VARCHAR(32) NOT NULL,
    agent_id        VARCHAR(32),
    triggered_by    VARCHAR(32),
    trigger_source  VARCHAR(32),               -- 'manual' | 'github_push' | 'webhook'
    commit_sha      VARCHAR(64),
    commit_message  TEXT,
    branch          VARCHAR(255),
    status          deployment_status NOT NULL DEFAULT 'pending',
    image_tag       VARCHAR(255),
    container_id    VARCHAR(128),
    error_message   TEXT,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_deployments_project_id FOREIGN KEY (project_id)
        REFERENCES projects (id) ON DELETE CASCADE,
    CONSTRAINT fk_deployments_agent_id FOREIGN KEY (agent_id)
        REFERENCES agents (id) ON DELETE SET NULL,
    CONSTRAINT fk_deployments_triggered_by FOREIGN KEY (triggered_by)
        REFERENCES users (id) ON DELETE SET NULL
);

CREATE INDEX idx_deployments_project_id ON deployments (project_id);
CREATE INDEX idx_deployments_agent_id   ON deployments (agent_id);
CREATE INDEX idx_deployments_status     ON deployments (status);
CREATE INDEX idx_deployments_created_at ON deployments (created_at);

CREATE TABLE deployment_jobs (
    id             VARCHAR(32) PRIMARY KEY,
    deployment_id  VARCHAR(32) NOT NULL,
    job_type       job_type    NOT NULL,
    status         job_status  NOT NULL DEFAULT 'pending',
    attempts       INTEGER     NOT NULL DEFAULT 0,
    max_attempts   INTEGER     NOT NULL DEFAULT 3,
    payload        JSONB,
    error_message  TEXT,
    queued_at      TIMESTAMPTZ,
    processed_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_deployment_jobs_deployment_id FOREIGN KEY (deployment_id)
        REFERENCES deployments (id) ON DELETE CASCADE
);

CREATE INDEX idx_deployment_jobs_deployment_id ON deployment_jobs (deployment_id);
-- worker polling: pick up pending/retrying jobs
CREATE INDEX idx_deployment_jobs_status ON deployment_jobs (status);

CREATE TABLE deployment_logs (
    id             VARCHAR(32) PRIMARY KEY,
    deployment_id  VARCHAR(32) NOT NULL,
    log_type       log_type    NOT NULL,
    message        TEXT,
    logged_at      TIMESTAMPTZ NOT NULL DEFAULT now(), -- 'timestamp' in DBML, renamed (SQL keyword)

    CONSTRAINT fk_deployment_logs_deployment_id FOREIGN KEY (deployment_id)
        REFERENCES deployments (id) ON DELETE CASCADE
);

-- log streaming: ordered fetch per deployment
CREATE INDEX idx_deployment_logs_deployment_id_logged_at ON deployment_logs (deployment_id, logged_at);
