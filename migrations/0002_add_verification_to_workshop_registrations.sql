ALTER TABLE workshop_registrations DROP CONSTRAINT IF EXISTS workshop_registrations_email_key;

ALTER TABLE workshop_registrations
    ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS regis_code VARCHAR(32),
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;

-- Backfill pre-existing rows (created before this migration) as already
-- confirmed, since they predate the pending/verification concept.
UPDATE workshop_registrations SET status = 'success' WHERE expires_at IS NULL;
UPDATE workshop_registrations SET expires_at = created_at WHERE expires_at IS NULL;
UPDATE workshop_registrations SET regis_code = md5(random()::text || id::text) WHERE regis_code IS NULL;

ALTER TABLE workshop_registrations ALTER COLUMN expires_at SET NOT NULL;
ALTER TABLE workshop_registrations ALTER COLUMN regis_code SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_workshop_registrations_regis_code ON workshop_registrations (regis_code);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'workshop_registrations_status_check'
    ) THEN
        ALTER TABLE workshop_registrations
            ADD CONSTRAINT workshop_registrations_status_check
            CHECK (status IN ('pending', 'success', 'expired'));
    END IF;
END $$;
