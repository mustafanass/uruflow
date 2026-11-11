/*
 * Copyright (C) 2026 Mustafa Naseer (Mustafa Gaeed)
 *
 * This file is part of uruflow.
 *
 * uruflow is free software: you can redistribute it and/or modify
 * it under the terms of the MIT License as described in the
 * LICENSE file distributed with this project.
 *
 * uruflow is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * MIT License for more details.
 *
 * You should have received a copy of the MIT License
 * along with uruflow. If not, see the LICENSE file in the project root.
 */

package sqlite

const schema = `
CREATE TABLE IF NOT EXISTS console_sessions (
	token_hash BLOB PRIMARY KEY,
	username TEXT NOT NULL,
	csrf TEXT NOT NULL,
	expires_at INTEGER NOT NULL,
	last_active INTEGER NOT NULL,
	credentials BLOB NOT NULL,
	FOREIGN KEY (username) REFERENCES console_users(username) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_console_sessions_user ON console_sessions(username);
CREATE INDEX IF NOT EXISTS idx_console_sessions_expiry ON console_sessions(expires_at);
CREATE TABLE IF NOT EXISTS observation_revisions (name TEXT PRIMARY KEY, revision INTEGER NOT NULL);
INSERT OR IGNORE INTO observation_revisions(name,revision) VALUES('releases',0);
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS console_users (
    username TEXT PRIMARY KEY,
    role TEXT NOT NULL CHECK(role IN ('admin','operator','viewer')),
    password_hash BLOB NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS audit_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    resource TEXT NOT NULL,
    outcome TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS agents (
	id             TEXT PRIMARY KEY,
	name           TEXT NOT NULL UNIQUE,
	auth_key       TEXT NOT NULL,
	roles          TEXT NOT NULL DEFAULT '[]',
	host           TEXT NOT NULL DEFAULT '',
	hostname       TEXT NOT NULL DEFAULT '',
	version        TEXT NOT NULL DEFAULT '',
	platform       TEXT NOT NULL DEFAULT '',
	status         TEXT NOT NULL DEFAULT 'offline',
	cpu_percent    REAL NOT NULL DEFAULT 0,
	memory_percent REAL NOT NULL DEFAULT 0,
	memory_used    INTEGER NOT NULL DEFAULT 0,
	memory_total   INTEGER NOT NULL DEFAULT 0,
	disk_percent   REAL NOT NULL DEFAULT 0,
	disk_used      INTEGER NOT NULL DEFAULT 0,
	disk_total     INTEGER NOT NULL DEFAULT 0,
	uptime         INTEGER NOT NULL DEFAULT 0,
	last_seen      DATETIME,
	registered_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS projects (
	name        TEXT PRIMARY KEY,
	git_url     TEXT NOT NULL,
	branch      TEXT NOT NULL DEFAULT 'main',
	dockerfile  TEXT NOT NULL DEFAULT '',
	context     TEXT NOT NULL DEFAULT '',
	build_args  TEXT NOT NULL DEFAULT '{}',
	builder     TEXT NOT NULL DEFAULT '',
	runners     TEXT NOT NULL DEFAULT '[]',
	auto_deploy INTEGER NOT NULL DEFAULT 1,
	workflow    TEXT NOT NULL DEFAULT '',
	timeout_ns  INTEGER NOT NULL DEFAULT 0,
	runtime     TEXT NOT NULL DEFAULT '{}',
	services    TEXT NOT NULL DEFAULT '[]',
	env         TEXT NOT NULL DEFAULT '',
	resources   TEXT NOT NULL DEFAULT '{}',
	source      TEXT NOT NULL DEFAULT '',
	created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS releases (
	artifacts_expired INTEGER NOT NULL DEFAULT 0,
	id           TEXT PRIMARY KEY,
	project      TEXT NOT NULL,
	branch       TEXT NOT NULL DEFAULT '',
	commit_sha   TEXT NOT NULL DEFAULT '',
	commits      TEXT NOT NULL DEFAULT '{}',
	image        TEXT NOT NULL DEFAULT '',
	images       TEXT NOT NULL DEFAULT '{}',
	digest       TEXT NOT NULL DEFAULT '',
	status       TEXT NOT NULL DEFAULT 'pending',
	builder      TEXT NOT NULL DEFAULT '',
	builder_name TEXT NOT NULL DEFAULT '',
	trigger_type TEXT NOT NULL DEFAULT 'manual',
		message      TEXT NOT NULL DEFAULT '',
		spec         TEXT NOT NULL DEFAULT '{}',
		started_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	ended_at     DATETIME,
	duration_ms  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS release_targets (
	containers TEXT NOT NULL DEFAULT '{}',
	release_id TEXT NOT NULL,
	agent_id   TEXT NOT NULL,
	agent_name TEXT NOT NULL DEFAULT '',
	status     TEXT NOT NULL DEFAULT 'pending',
	message    TEXT NOT NULL DEFAULT '',
	ended_at   DATETIME,
	PRIMARY KEY (release_id, agent_id),
	FOREIGN KEY (release_id) REFERENCES releases(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS release_logs (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	release_id TEXT NOT NULL,
	stage      TEXT NOT NULL DEFAULT 'build',
	agent_name TEXT NOT NULL DEFAULT '',
	stream     TEXT NOT NULL DEFAULT 'stdout',
	line       TEXT NOT NULL,
	timestamp  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (release_id) REFERENCES releases(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS containers (
	id            TEXT NOT NULL,
	agent_id      TEXT NOT NULL,
	name          TEXT NOT NULL DEFAULT '',
	project       TEXT NOT NULL DEFAULT '',
	service       TEXT NOT NULL DEFAULT '',
	image         TEXT NOT NULL DEFAULT '',
	state         TEXT NOT NULL DEFAULT '',
	health        TEXT NOT NULL DEFAULT '',
	cpu_percent   REAL NOT NULL DEFAULT 0,
	memory_usage  INTEGER NOT NULL DEFAULT 0,
	memory_limit  INTEGER NOT NULL DEFAULT 0,
	network_rx    INTEGER NOT NULL DEFAULT 0,
	network_tx    INTEGER NOT NULL DEFAULT 0,
	restart_count INTEGER NOT NULL DEFAULT 0,
	started_at    DATETIME,
	PRIMARY KEY (agent_id, id),
	FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS alerts (
	id          TEXT PRIMARY KEY,
	agent_id    TEXT NOT NULL DEFAULT '',
	agent_name  TEXT NOT NULL DEFAULT '',
	type        TEXT NOT NULL DEFAULT '',
	message     TEXT NOT NULL,
	severity    TEXT NOT NULL DEFAULT 'warning',
	resolved    INTEGER NOT NULL DEFAULT 0,
	created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	resolved_at DATETIME
);

CREATE TABLE IF NOT EXISTS secrets (
	name       TEXT PRIMARY KEY,
	value      BLOB NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
	provider    TEXT NOT NULL,
	delivery_id TEXT NOT NULL,
	received_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	repository  TEXT NOT NULL DEFAULT '',
	branch      TEXT NOT NULL DEFAULT '',
	commit_sha  TEXT NOT NULL DEFAULT '',
	result      TEXT NOT NULL DEFAULT '',
	detail      TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (provider, delivery_id)
);

CREATE TABLE IF NOT EXISTS settings (
	name       TEXT PRIMARY KEY,
	value      TEXT NOT NULL,
	revision   INTEGER NOT NULL DEFAULT 1,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS push_sources (
	project    TEXT NOT NULL,
	repository TEXT NOT NULL,
	branch     TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (project, repository, branch),
	FOREIGN KEY (project) REFERENCES projects(name) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_releases_project ON releases(project);
CREATE TRIGGER IF NOT EXISTS release_observe_insert AFTER INSERT ON releases BEGIN UPDATE observation_revisions SET revision=revision+1 WHERE name='releases'; END;
CREATE TRIGGER IF NOT EXISTS release_observe_update AFTER UPDATE ON releases BEGIN UPDATE observation_revisions SET revision=revision+1 WHERE name='releases'; END;
CREATE TRIGGER IF NOT EXISTS release_observe_delete AFTER DELETE ON releases BEGIN UPDATE observation_revisions SET revision=revision+1 WHERE name='releases'; END;
CREATE TRIGGER IF NOT EXISTS target_observe_insert AFTER INSERT ON release_targets BEGIN UPDATE observation_revisions SET revision=revision+1 WHERE name='releases'; END;
CREATE TRIGGER IF NOT EXISTS target_observe_update AFTER UPDATE ON release_targets BEGIN UPDATE observation_revisions SET revision=revision+1 WHERE name='releases'; END;
CREATE TRIGGER IF NOT EXISTS target_observe_delete AFTER DELETE ON release_targets BEGIN UPDATE observation_revisions SET revision=revision+1 WHERE name='releases'; END;
CREATE UNIQUE INDEX IF NOT EXISTS idx_releases_one_active_project ON releases(project)
	WHERE status IN ('pending', 'building', 'releasing');
CREATE INDEX IF NOT EXISTS idx_releases_started ON releases(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_release_logs_release ON release_logs(release_id, id);
CREATE INDEX IF NOT EXISTS idx_containers_agent ON containers(agent_id);
CREATE INDEX IF NOT EXISTS idx_alerts_resolved ON alerts(resolved, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_received ON webhook_deliveries(received_at);
`
