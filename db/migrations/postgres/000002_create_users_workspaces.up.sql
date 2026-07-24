-- Users / Workspaces
-- id columns are app-generated cuids (VARCHAR(32)).

CREATE TABLE users (
    id          VARCHAR(32) PRIMARY KEY,
    clerk_id    VARCHAR(255) NOT NULL,
    email       VARCHAR(255) NOT NULL,
    name        VARCHAR(255),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT uq_users_clerk_id UNIQUE (clerk_id),
    CONSTRAINT uq_users_email    UNIQUE (email)
);

CREATE TABLE workspaces (
    id          VARCHAR(32) PRIMARY KEY,
    user_id     VARCHAR(32) NOT NULL,
    wsp_name    VARCHAR(255),
    wsp_slug    VARCHAR(255),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_workspaces_wsp_slug UNIQUE (wsp_slug),
    CONSTRAINT fk_workspaces_user_id FOREIGN KEY (user_id)
        REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX idx_workspaces_user_id ON workspaces (user_id);
