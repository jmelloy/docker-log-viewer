-- Example database schema for testing EXPLAIN feature
-- This creates a simple schema that matches the queries in logs.sample.txt

CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(255) PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    name VARCHAR(255),
    email VARCHAR(255),
    workspace_app_id VARCHAR(255)
);

CREATE TABLE IF NOT EXISTS workspaces (
    id VARCHAR(255) PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    name VARCHAR(255),
    messageable_by VARCHAR(50)
);

CREATE TABLE IF NOT EXISTS workspace_apps (
    id VARCHAR(255) PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    user_id VARCHAR(255),
    workspace_id VARCHAR(255),
    app_id VARCHAR(255),
    app_user_id VARCHAR(255),
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id)
);

CREATE TABLE IF NOT EXISTS workspace_app_webhooks (
    id VARCHAR(255) PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    workspace_app_id VARCHAR(255),
    auth_token VARCHAR(255),
    FOREIGN KEY (workspace_app_id) REFERENCES workspace_apps(id)
);

CREATE TABLE IF NOT EXISTS threads (
    id VARCHAR(255) PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    workspace_id VARCHAR(255),
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id)
);

CREATE TABLE IF NOT EXISTS messages (
    id VARCHAR(255) PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    thread_id VARCHAR(255),
    user_id VARCHAR(255),
    external_id VARCHAR(255),
    stream_id VARCHAR(255),
    type VARCHAR(50),
    text TEXT,
    pin_id VARCHAR(255),
    quoted_message_id VARCHAR(255),
    reaction_counts JSONB,
    silent BOOLEAN DEFAULT FALSE,
    attachment_ids TEXT[],
    reply_thread_ids TEXT[],
    FOREIGN KEY (thread_id) REFERENCES threads(id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS thread_participants (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    thread_id VARCHAR(255),
    user_id VARCHAR(255),
    external_id VARCHAR(255),
    last_read_id VARCHAR(255),
    last_seen_id VARCHAR(255),
    last_mention_message_id VARCHAR(255),
    subscription VARCHAR(50),
    archived_at TIMESTAMP,
    remind_at TIMESTAMP,
    remind_at_entry_id VARCHAR(255),
    starred_at TIMESTAMP,
    unread_at TIMESTAMP,
    stream_joined_at TIMESTAMP,
    pending_notify_at TIMESTAMP,
    mailboxes TEXT[],
    FOREIGN KEY (thread_id) REFERENCES threads(id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    UNIQUE (thread_id, user_id)
);

CREATE TABLE IF NOT EXISTS groups (
    id VARCHAR(255) PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    workspace_id VARCHAR(255),
    name VARCHAR(255),
    messageable_by VARCHAR(50),
    archived_at TIMESTAMP,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id)
);

CREATE TABLE IF NOT EXISTS message_metadata (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    message_id VARCHAR(255),
    FOREIGN KEY (message_id) REFERENCES messages(id)
);

-- Create indexes for common queries
CREATE INDEX idx_users_deleted_at ON users(deleted_at);
CREATE INDEX idx_workspaces_deleted_at ON workspaces(deleted_at);
CREATE INDEX idx_workspace_apps_deleted_at ON workspace_apps(deleted_at);
CREATE INDEX idx_threads_deleted_at ON threads(deleted_at);
CREATE INDEX idx_messages_deleted_at ON messages(deleted_at);
CREATE INDEX idx_messages_user_external ON messages(user_id, external_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_thread_participants_user_external ON thread_participants(user_id, external_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_workspace_app_webhooks_lookup ON workspace_app_webhooks(id, auth_token) WHERE deleted_at IS NULL;

-- Insert some sample data
INSERT INTO workspaces (id, name, messageable_by) VALUES 
    ('ws_001', 'Test Workspace', 'members');

INSERT INTO users (id, name, email, workspace_app_id) VALUES 
    ('user_001', 'Test User', 'test@example.com', 'app_001'),
    ('user_002', 'Another User', 'another@example.com', NULL);

INSERT INTO workspace_apps (id, user_id, workspace_id, app_id, app_user_id) VALUES 
    ('app_001', 'user_001', 'ws_001', 'slack_001', 'slack_user_001');

INSERT INTO threads (id, workspace_id) VALUES 
    ('thread_001', 'ws_001'),
    ('thread_002', 'ws_001');

INSERT INTO messages (id, thread_id, user_id, text, type) VALUES 
    ('msg_001', 'thread_001', 'user_001', 'Hello World', 'text'),
    ('msg_002', 'thread_001', 'user_002', 'Reply', 'text');

INSERT INTO groups (id, workspace_id, name, messageable_by) VALUES 
    ('group_001', 'ws_001', 'Test Group', 'members');
