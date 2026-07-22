-- Projects / Environment variables

CREATE TABLE projects (
    id               VARCHAR(32) PRIMARY KEY,
    wsp_id           VARCHAR(32) NOT NULL,
    project_name     VARCHAR(255),
    project_stack    tech_stack,
    default_branch   VARCHAR(255) NOT NULL DEFAULT 'main',
    build_command    TEXT,
    install_command  TEXT,
    output_dir       VARCHAR(255),
    is_deployed      BOOLEAN     NOT NULL DEFAULT false,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_projects_wsp_id FOREIGN KEY (wsp_id)
        REFERENCES workspaces (id) ON DELETE CASCADE
);

CREATE INDEX idx_projects_wsp_id ON projects (wsp_id);

CREATE TABLE envs (
    id          VARCHAR(32) PRIMARY KEY,
    project_id  VARCHAR(32) NOT NULL,
    env_key     VARCHAR(255) NOT NULL,
    env_value   TEXT, -- encrypted at rest (AES) by the app layer
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- one value per key per project
    CONSTRAINT uq_envs_project_id_env_key UNIQUE (project_id, env_key),
    CONSTRAINT fk_envs_project_id FOREIGN KEY (project_id)
        REFERENCES projects (id) ON DELETE CASCADE
);

CREATE INDEX idx_envs_project_id ON envs (project_id);
