-- Operator enrollment for the Yotta online test environment only.
-- Run with psql -X -v ON_ERROR_STOP=1 against the deployed Identity database.
-- Repeat runs verify the same declaration; never adopt or overwrite another client.
BEGIN;
SELECT pg_advisory_xact_lock(hashtextextended('yotta-desktop-online-test', 0));
INSERT INTO oidc_clients (
    id, public, secret_hash, secret_ref, redirect_uris, post_logout_redirect_uris,
    audiences, grant_types, response_types, scopes, subject_type, subject_sector, managed_by
) VALUES (
    'yotta-desktop-online-test', TRUE, '', '',
    ARRAY['http://127.0.0.1:43822/callback'], ARRAY[]::text[],
    ARRAY['urn:yotta:registry', 'urn:yotta:hub', 'identity-api'],
    ARRAY['authorization_code', 'refresh_token'], ARRAY['code'],
    ARRAY['openid', 'profile', 'offline_access'], 'public', 'first-party', 'yotta-online-test'
) ON CONFLICT (id) DO NOTHING;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM oidc_clients
        WHERE id = 'yotta-desktop-online-test' AND public
          AND secret_hash = '' AND secret_ref = ''
          AND redirect_uris = ARRAY['http://127.0.0.1:43822/callback']
          AND post_logout_redirect_uris = ARRAY[]::text[]
          AND audiences = ARRAY['urn:yotta:registry', 'urn:yotta:hub', 'identity-api']
          AND grant_types = ARRAY['authorization_code', 'refresh_token']
          AND response_types = ARRAY['code']
          AND scopes = ARRAY['openid', 'profile', 'offline_access']
          AND subject_type = 'public' AND subject_sector = 'first-party'
          AND managed_by = 'yotta-online-test'
    ) THEN
        RAISE EXCEPTION 'Yotta online client differs; inspect the existing registration before changing it';
    END IF;
END $$;
COMMIT;
