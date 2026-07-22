-- GitHub App integration: installations, repositories, webhook events.
-- App-level credentials (app id, private key, webhook secret) live in
-- env/secrets, NOT in the database.

CREATE TABLE github_installations (
    id                          VARCHAR(32) PRIMARY KEY,
    user_id                     VARCHAR(32) NOT NULL,      -- user who completed the install flow
    installation_id             VARCHAR(255) NOT NULL,     -- GitHub App installation id
    account_login               VARCHAR(255),              -- github org/user the app is installed on
    account_type                VARCHAR(32),               -- 'User' | 'Organization'
    repository_selection        VARCHAR(32),               -- 'all' | 'selected'
    permissions                 JSONB,                     -- granted scopes snapshot
    cached_installation_token   TEXT,                      -- short-lived token cache (encrypted)
    cached_token_expires_at     TIMESTAMPTZ,               -- refresh before this (~1hr tokens)
    suspended_at                TIMESTAMPTZ,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_github_installations_installation_id UNIQUE (installation_id),
    CONSTRAINT fk_github_installations_user_id FOREIGN KEY (user_id)
        REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX idx_github_installations_user_id ON github_installations (user_id);

CREATE TABLE repositories (
    id               VARCHAR(32) PRIMARY KEY,
    installation_id  VARCHAR(32) NOT NULL,
    project_id       VARCHAR(32),               -- 1:1 — a project tracks exactly one repo
    github_repo_id   VARCHAR(64),               -- numeric GitHub repo id, stable across renames
    repo_name        VARCHAR(255),
    repo_full_name   VARCHAR(255),              -- e.g. viralmistry/equitycase
    repo_url         TEXT,
    default_branch   VARCHAR(255) NOT NULL DEFAULT 'main',
    is_private       BOOLEAN     NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_repositories_github_repo_id UNIQUE (github_repo_id),
    CONSTRAINT uq_repositories_project_id     UNIQUE (project_id), -- enforces 1:1
    CONSTRAINT fk_repositories_installation_id FOREIGN KEY (installation_id)
        REFERENCES github_installations (id) ON DELETE CASCADE,
    CONSTRAINT fk_repositories_project_id FOREIGN KEY (project_id)
        REFERENCES projects (id) ON DELETE SET NULL
);

CREATE INDEX idx_repositories_installation_id ON repositories (installation_id);

CREATE TABLE webhook_events (
    id               VARCHAR(32) PRIMARY KEY,
    repository_id    VARCHAR(32) NOT NULL,
    event_type       VARCHAR(64),               -- 'push', 'pull_request', etc.
    delivery_id      VARCHAR(255),              -- GitHub X-GitHub-Delivery header (idempotency)
    payload          JSONB,
    signature_valid  BOOLEAN,
    processed_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_webhook_events_delivery_id UNIQUE (delivery_id),
    CONSTRAINT fk_webhook_events_repository_id FOREIGN KEY (repository_id)
        REFERENCES repositories (id) ON DELETE CASCADE
);

CREATE INDEX idx_webhook_events_repository_id ON webhook_events (repository_id);
CREATE INDEX idx_webhook_events_event_type    ON webhook_events (event_type);
