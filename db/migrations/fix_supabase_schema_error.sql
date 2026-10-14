-- =====================================================================
-- PERBAIKAN SUPABASE: "Database error querying schema" (Error 500)
-- Penyebab: Supabase GoTrue Auth crash jika ada kolom di auth.users bernilai NULL
-- akibat INSERT manual ke tabel auth.users
-- Jalankan query ini di: Supabase Dashboard -> SQL Editor -> New Query
-- =====================================================================

-- 1. Normalisasi nilai NULL menjadi string kosong ('') pada auth.users
UPDATE auth.users 
SET 
    confirmation_token          = COALESCE(confirmation_token, ''),
    recovery_token              = COALESCE(recovery_token, ''),
    email_change                = COALESCE(email_change, ''),
    email_change_token_new      = COALESCE(email_change_token_new, ''),
    email_change_token_current  = COALESCE(email_change_token_current, ''),
    phone_change                = COALESCE(phone_change, ''),
    phone_change_token          = COALESCE(phone_change_token, ''),
    reauthentication_token      = COALESCE(reauthentication_token, ''),
    raw_app_meta_data           = COALESCE(raw_app_meta_data, '{"provider":"email","providers":["email"]}'::jsonb),
    raw_user_meta_data          = COALESCE(raw_user_meta_data, '{}'::jsonb);

-- 2. Pastikan permission schema public dan auth sudah benar
GRANT USAGE ON SCHEMA public TO postgres, anon, authenticated, service_role;
GRANT ALL ON ALL TABLES IN SCHEMA public TO postgres, anon, authenticated, service_role;
GRANT ALL ON ALL FUNCTIONS IN SCHEMA public TO postgres, anon, authenticated, service_role;
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO postgres, anon, authenticated, service_role;

GRANT USAGE ON SCHEMA auth TO postgres, supabase_auth_admin, service_role;
GRANT ALL ON ALL TABLES IN SCHEMA auth TO postgres, supabase_auth_admin, service_role;
GRANT ALL ON ALL SEQUENCES IN SCHEMA auth TO postgres, supabase_auth_admin, service_role;
GRANT ALL ON ALL ROUTINES IN SCHEMA auth TO postgres, supabase_auth_admin, service_role;

-- 3. Reload Schema Cache PostgREST
NOTIFY pgrst, 'reload schema';
NOTIFY pgrst, 'reload config';

-- 4. Verifikasi status user
SELECT id, email, raw_app_meta_data->'roles' as roles, confirmation_token, recovery_token 
FROM auth.users;
