-- Enum types for the deployment automation system.
-- NOTE: Postgres-specific (CREATE TYPE). A future mysql/ migration dir
-- would inline these as ENUM(...) column definitions instead.

CREATE TYPE tech_stack AS ENUM (
    'nextjs',
    'nodejs',
    'react',
    'go',
    'fastapi',
    'python',
    'django',
    'express'
);

CREATE TYPE deployment_status AS ENUM (
    'pending',
    'queued',
    'building',
    'deploying',
    'running',
    'success',
    'failed',
    'stopped'
);

CREATE TYPE job_type AS ENUM (
    'build_deploy',
    'health_check',
    'log_stream',
    'agent_registration'
);

CREATE TYPE job_status AS ENUM (
    'pending',
    'active',
    'completed',
    'failed',
    'retrying'
);

CREATE TYPE log_type AS ENUM (
    'build',
    'runtime',
    'error'
);

CREATE TYPE agent_status AS ENUM (
    'pending',
    'active',
    'inactive',
    'offline'
);

CREATE TYPE token_type AS ENUM (
    'registration',
    'jwt'
);
