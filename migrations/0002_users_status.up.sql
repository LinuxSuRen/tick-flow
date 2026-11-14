-- Invited accounts stay 'invited' until they set a password through the
-- one-time invitation link (issue #11). Login is only allowed for 'active'.
ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT 'active';
