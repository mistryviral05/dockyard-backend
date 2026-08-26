-- up: add_system_log_type
--
-- Worker "system" type ki lines bhejta hai (cloning, build started, failed).
-- Enum me ye value thi hi nahi, isliye persist karte hi insert fail hota.
--
-- Postgres 12+ me ADD VALUE transaction ke andar chal jaata hai, bas usi
-- transaction me us value ko use nahi kar sakte. Migration sirf value jodti
-- hai, isliye theek hai.

ALTER TYPE log_type ADD VALUE IF NOT EXISTS 'system';
