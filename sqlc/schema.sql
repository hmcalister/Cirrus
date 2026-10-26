-- Application-owned user profiles, keyed by the Stratus identity.
--
-- Stratus owns authentication; the identity it reports is the normalised email
-- address the user signed in with. Cirrus stores only the profile data it needs
-- alongside that identity.
CREATE TABLE user_accounts (
    email                   TEXT PRIMARY KEY,
    active                  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
