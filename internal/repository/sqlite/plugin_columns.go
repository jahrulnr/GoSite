package sqlite

// Shared column list for plugin_versions (includes migration 007 provenance).
const pluginVersionColumns = `
	id, plugin_id, version, name, tier, api_version, min_gosite_version,
	rpc_version, config_version, manifest_json, capabilities_json, ui_json,
	artifact_digest, artifact_path, state, failure_class, failure_message,
	failure_at, config_deleted_at, created_at, updated_at,
	source_type, source_ref, resolved_url, resolved_digest, signing_key_id,
	source_commit, builder_image_digest, source_repository, install_path,
	permissions_ack_at, permissions_acked_caps, install_log`
