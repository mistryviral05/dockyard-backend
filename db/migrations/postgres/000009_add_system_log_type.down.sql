-- down: add_system_log_type
--
-- Postgres enum se value hata nahi sakte, isliye poora type dobara banana
-- padta hai. Jo rows 'system' pe hain unhe pehle 'build' kar dete hain,
-- warna naye type me cast fail ho jayega.

UPDATE deployment_logs SET log_type = 'build' WHERE log_type = 'system';

ALTER TYPE log_type RENAME TO log_type_old;

CREATE TYPE log_type AS ENUM ('build', 'runtime', 'error');

ALTER TABLE deployment_logs
    ALTER COLUMN log_type TYPE log_type USING log_type::text::log_type;

DROP TYPE log_type_old;
