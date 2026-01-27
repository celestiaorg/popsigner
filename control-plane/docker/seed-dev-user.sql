-- Seed dev user for local development
-- This creates a test user with a known session token for easy testing

-- Insert dev organization (idempotent with ON CONFLICT)
INSERT INTO organizations (id, name, slug, plan, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000001'::uuid,
    'Dev Organization',
    'dev-org',
    'free',
    NOW(),
    NOW()
)
ON CONFLICT (id) DO NOTHING;

-- Insert dev user (idempotent with ON CONFLICT)
INSERT INTO users (id, name, email, oauth_provider, oauth_provider_id, avatar_url, email_verified, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000002'::uuid,
    'Dev User',
    'dev@popsigner.local',
    'dev',
    'dev-user-1',
    '',
    true,
    NOW(),
    NOW()
)
ON CONFLICT (email) DO NOTHING;

-- Add dev user to dev organization (idempotent with ON CONFLICT)
INSERT INTO org_members (org_id, user_id, role, joined_at)
VALUES (
    '00000000-0000-0000-0000-000000000001'::uuid,
    '00000000-0000-0000-0000-000000000002'::uuid,
    'owner',
    NOW()
)
ON CONFLICT (org_id, user_id) DO NOTHING;

-- Insert dev session with a known token (idempotent with ON CONFLICT)
-- This session token will be: dev-session-token-12345
-- To use: Set cookie "banhbao_session=dev-session-token-12345"
INSERT INTO sessions (id, user_id, expires_at, created_at)
VALUES (
    'dev-session-token-12345',
    '00000000-0000-0000-0000-000000000002'::uuid,
    NOW() + INTERVAL '365 days',  -- Expires in 1 year
    NOW()
)
ON CONFLICT (id) DO UPDATE
SET expires_at = NOW() + INTERVAL '365 days';

-- Success message
DO $$
BEGIN
    RAISE NOTICE '✅ Dev user created successfully!';
    RAISE NOTICE '   Email: dev@popsigner.local';
    RAISE NOTICE '   Organization: Dev Organization';
    RAISE NOTICE '   Session Token: dev-session-token-12345';
    RAISE NOTICE '';
    RAISE NOTICE '📝 To login, set this cookie in your browser:';
    RAISE NOTICE '   Name: banhbao_session';
    RAISE NOTICE '   Value: dev-session-token-12345';
    RAISE NOTICE '   Domain: .localhost (note the dot - required for subdomain sharing)';
    RAISE NOTICE '';
    RAISE NOTICE '🔧 Or use this JavaScript in browser console:';
    RAISE NOTICE '   document.cookie = "banhbao_session=dev-session-token-12345; domain=.localhost; path=/; max-age=31536000"';
END $$;
