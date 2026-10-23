CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY);
CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS jobs(id TEXT PRIMARY KEY,kind TEXT NOT NULL,state TEXT NOT NULL,created TEXT NOT NULL,finished TEXT,summary TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS diagnostic_results(job_id TEXT PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,result TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS tool_inventory(name TEXT PRIMARY KEY,version TEXT NOT NULL,checked TEXT NOT NULL);
INSERT OR IGNORE INTO schema_migrations VALUES(1);
INSERT INTO settings VALUES('preferences','{"language":"pt-BR","retention_days":14}');
INSERT INTO jobs VALUES('fixture-job','tcp','completed','2026-01-15T00:00:00Z','2026-01-15T00:00:01Z','fixture kept');
