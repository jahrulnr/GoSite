CREATE UNIQUE INDEX IF NOT EXISTS idx_log_events_line_hash ON log_events (line_hash) WHERE line_hash <> '';
