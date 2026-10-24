-- Postgres cannot drop a value from an enum. Rolling this back would mean
-- recreating tech_stack and rewriting every column that uses it, so the value
-- is left in place on purpose.
SELECT 1;
