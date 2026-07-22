-- Domains: auto subdomains + user custom domains (reverse proxy / SSL).

CREATE TABLE domains (
    id           VARCHAR(32) PRIMARY KEY,
    project_id   VARCHAR(32) NOT NULL,
    domain       VARCHAR(255) NOT NULL,
    is_custom    BOOLEAN     NOT NULL DEFAULT false,     -- false = auto subdomain, true = user's own
    ssl_status   VARCHAR(32) NOT NULL DEFAULT 'pending', -- pending | issued | failed
    verified_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_domains_domain UNIQUE (domain),
    CONSTRAINT fk_domains_project_id FOREIGN KEY (project_id)
        REFERENCES projects (id) ON DELETE CASCADE
);

CREATE INDEX idx_domains_project_id ON domains (project_id);
