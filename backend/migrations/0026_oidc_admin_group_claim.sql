ALTER TABLE oidc_providers ADD COLUMN IF NOT EXISTS admin_group_claim TEXT;
ALTER TABLE oidc_providers ADD COLUMN IF NOT EXISTS admin_group_value TEXT;
