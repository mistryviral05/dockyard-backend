ALTER TABLE agent_metrics
    DROP COLUMN IF EXISTS load_1,
    DROP COLUMN IF EXISTS load_5,
    DROP COLUMN IF EXISTS load_15,
    DROP COLUMN IF EXISTS cpu_cores,
    DROP COLUMN IF EXISTS uptime_seconds;
