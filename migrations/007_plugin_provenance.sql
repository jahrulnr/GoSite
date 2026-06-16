-- 007_plugin_provenance.sql: remote install provenance and install audit

ALTER TABLE plugin_versions ADD COLUMN source_type TEXT NOT NULL DEFAULT 'upload';
ALTER TABLE plugin_versions ADD COLUMN source_ref TEXT NOT NULL DEFAULT '';
ALTER TABLE plugin_versions ADD COLUMN resolved_url TEXT NOT NULL DEFAULT '';
ALTER TABLE plugin_versions ADD COLUMN resolved_digest TEXT NOT NULL DEFAULT '';
ALTER TABLE plugin_versions ADD COLUMN signing_key_id TEXT NOT NULL DEFAULT '';
ALTER TABLE plugin_versions ADD COLUMN source_commit TEXT NOT NULL DEFAULT '';
ALTER TABLE plugin_versions ADD COLUMN builder_image_digest TEXT NOT NULL DEFAULT '';
ALTER TABLE plugin_versions ADD COLUMN source_repository TEXT NOT NULL DEFAULT '';
ALTER TABLE plugin_versions ADD COLUMN install_path TEXT NOT NULL DEFAULT 'upload';
ALTER TABLE plugin_versions ADD COLUMN permissions_ack_at DATETIME;
ALTER TABLE plugin_versions ADD COLUMN permissions_acked_caps TEXT NOT NULL DEFAULT '';
ALTER TABLE plugin_versions ADD COLUMN install_log TEXT NOT NULL DEFAULT '[]';
