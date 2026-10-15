CREATE TABLE IF NOT EXISTS workshop_registrations (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT workshop_registrations_email_key UNIQUE (email)
);
