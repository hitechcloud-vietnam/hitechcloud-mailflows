ALTER TABLE email_accounts
  ADD COLUMN IF NOT EXISTS sender_name VARCHAR(255);
