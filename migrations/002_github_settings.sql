CREATE TABLE IF NOT EXISTS github_settings (
    user_id      TEXT PRIMARY KEY,
    access_token TEXT NOT NULL,
    username     TEXT NOT NULL,
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
