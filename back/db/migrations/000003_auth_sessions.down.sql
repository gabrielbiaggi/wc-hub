ALTER TABLE hosts DROP COLUMN IF EXISTS created_by;
ALTER TABLE integrations DROP COLUMN IF EXISTS created_by;
DROP TABLE IF EXISTS auth_sessions;

