-- Per-message computed category (primary / newsletter / promotion / automated / social).
-- NULL means uncategorized (treated as 'primary' in queries).
ALTER TABLE messages ADD COLUMN IF NOT EXISTS category VARCHAR(50);

-- Index for category tab filtering. Partial on is_deleted=false to stay small.
CREATE INDEX IF NOT EXISTS idx_messages_category
  ON messages (account_id, folder, category, date DESC)
  WHERE is_deleted = false;

-- Per-account opt-in toggle. Off by default as specified.
ALTER TABLE email_accounts
  ADD COLUMN IF NOT EXISTS categorization_enabled BOOLEAN NOT NULL DEFAULT false;

-- User-configured social category list sources.
-- source_type='manual'  → value is a domain or email address the user typed in
-- source_type='builtin' → value is a named set key bundled with the app
-- source_type='url'     → value is a URL; resolved_domains is populated by a fetch job
CREATE TABLE IF NOT EXISTS category_list_sources (
  id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id          UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source_type      VARCHAR(20) NOT NULL CHECK (source_type IN ('manual', 'builtin', 'url')),
  value            TEXT        NOT NULL,
  label            VARCHAR(200),
  resolved_domains TEXT[],
  last_fetched_at  TIMESTAMPTZ,
  fetch_ok         BOOLEAN,
  fetch_error      TEXT,
  enabled          BOOLEAN     NOT NULL DEFAULT true,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, source_type, value)
);
