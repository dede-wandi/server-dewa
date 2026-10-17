-- =====================================================================
-- MIGRATION 26: REGISTRASI ANGGOTA DOWNLINE LANGSUNG KE POSTGRESQL DB
-- Jalankan query ini di: Supabase Dashboard -> SQL Editor -> New Query
-- =====================================================================

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA extensions;

-- 1. Bersihkan trigger lama/rusak pada auth.users (jika ada) yang menyebabkan error 500
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN 
        SELECT trigger_name 
        FROM information_schema.triggers 
        WHERE event_object_schema = 'auth' 
          AND event_object_table = 'users'
          AND trigger_name IN ('on_auth_user_created', 'handle_new_user', 'trg_on_auth_user_created')
    LOOP
        EXECUTE 'DROP TRIGGER IF EXISTS ' || quote_ident(r.trigger_name) || ' ON auth.users CASCADE;';
    END LOOP;
END $$;

-- 2. Function RPC: create_agency_member_user
-- Menyimpan 100% data akun downline langsung ke auth.users, agency_members, app_wallets, dan user_roles
CREATE OR REPLACE FUNCTION public.create_agency_member_user(
    p_agent_id UUID,
    p_name TEXT,
    p_phone TEXT,
    p_password TEXT,
    p_business_name TEXT DEFAULT NULL
)
RETURNS JSONB AS $$
DECLARE
    v_clean_phone TEXT;
    v_email TEXT;
    v_user_id UUID;
    v_hash TEXT;
    v_member_record RECORD;
    v_business TEXT;
BEGIN
    -- Validasi Input
    IF p_agent_id IS NULL THEN
        RAISE EXCEPTION 'ID Agen Upline wajib diisi';
    END IF;
    IF TRIM(p_name) = '' THEN
        RAISE EXCEPTION 'Nama anggota wajib diisi';
    END IF;
    IF LENGTH(TRIM(p_password)) < 6 THEN
        RAISE EXCEPTION 'Password minimal 6 karakter';
    END IF;

    -- Normalisasi nomor HP ke format 08xxxx
    v_clean_phone := REPLACE(REPLACE(REPLACE(TRIM(p_phone), ' ', ''), '-', ''), '+', '');
    IF v_clean_phone LIKE '62%' THEN
        v_clean_phone := '0' || SUBSTRING(v_clean_phone FROM 3);
    END IF;

    IF LENGTH(v_clean_phone) < 8 THEN
        RAISE EXCEPTION 'Nomor HP tidak valid (minimal 8 angka)';
    END IF;

    v_email := LOWER(v_clean_phone || '@member.dewacell.com');
    v_business := COALESCE(NULLIF(TRIM(p_business_name), ''), TRIM(p_name));

    -- Cek jika nomor HP / email internal sudah terdaftar di auth.users
    SELECT id INTO v_user_id FROM auth.users WHERE LOWER(email) = v_email;
    IF v_user_id IS NOT NULL THEN
        RAISE EXCEPTION 'Nomor HP % sudah terdaftar di sistem', v_clean_phone;
    END IF;

    -- Generate UUID baru untuk akun anggota
    v_user_id := gen_random_uuid();
    v_hash := extensions.crypt(p_password, extensions.gen_salt('bf'));

    -- A. INSERT AKUN KE auth.users (Langsung auto-confirm tanpa verifikasi email)
    INSERT INTO auth.users (
        instance_id,
        id,
        aud,
        role,
        email,
        encrypted_password,
        email_confirmed_at,
        recovery_token,
        confirmation_token,
        email_change_token_new,
        email_change,
        phone,
        phone_confirmed_at,
        phone_change,
        phone_change_token,
        reauthentication_token,
        raw_app_meta_data,
        raw_user_meta_data,
        is_super_admin,
        created_at,
        updated_at
    ) VALUES (
        '00000000-0000-0000-0000-000000000000'::uuid,
        v_user_id,
        'authenticated',
        'authenticated',
        v_email,
        v_hash,
        NOW(),
        '',
        '',
        '',
        '',
        v_clean_phone,
        NOW(),
        '',
        '',
        '',
        '{"provider": "email", "providers": ["email"]}'::jsonb,
        jsonb_build_object(
            'full_name', TRIM(p_name),
            'display_name', TRIM(p_name),
            'phone', v_clean_phone,
            'business_name', v_business,
            'upline_id', p_agent_id,
            'role', 'user',
            'email_verified', true
        ),
        false,
        NOW(),
        NOW()
    );

    -- B. INSERT RELASI KE public.agency_members
    INSERT INTO public.agency_members (
        id,
        agent_id,
        member_id,
        member_name,
        member_phone,
        status,
        joined_at
    ) VALUES (
        gen_random_uuid(),
        p_agent_id,
        v_user_id,
        TRIM(p_name),
        v_clean_phone,
        'ACTIVE',
        NOW()
    ) RETURNING * INTO v_member_record;

    -- C. INSERT DOMPET SALDO KE public.app_wallets (Saldo 0)
    INSERT INTO public.app_wallets (
        user_id,
        balance,
        points,
        is_locked,
        created_at,
        updated_at
    ) VALUES (
        v_user_id,
        0,
        0,
        false,
        NOW(),
        NOW()
    ) ON CONFLICT (user_id) DO NOTHING;

    -- D. INSERT ROLE KE public.user_roles (Role user)
    INSERT INTO public.user_roles (
        user_id,
        role_id,
        assigned_at
    ) VALUES (
        v_user_id,
        'user',
        NOW()
    ) ON CONFLICT (user_id, role_id) DO NOTHING;

    RETURN jsonb_build_object(
        'success', true,
        'id', v_member_record.id,
        'agent_id', v_member_record.agent_id,
        'member_id', v_member_record.member_id,
        'member_name', v_member_record.member_name,
        'member_phone', v_member_record.member_phone,
        'email', v_email,
        'status', v_member_record.status,
        'joined_at', v_member_record.joined_at
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 3. Berikan hak akses eksekusi ke anon, authenticated, dan service_role
GRANT EXECUTE ON FUNCTION public.create_agency_member_user(UUID, TEXT, TEXT, TEXT, TEXT) TO anon, authenticated, service_role;

-- 4. FUNCTION: Ambil Daftar Anggota Downline per Agen (Bypass RLS via Security Definer)
CREATE OR REPLACE FUNCTION public.get_agency_members_by_agent(p_agent_id UUID)
RETURNS JSONB AS $$
DECLARE
    v_list JSONB;
BEGIN
    SELECT COALESCE(jsonb_agg(
        jsonb_build_object(
            'id', m.id,
            'agent_id', m.agent_id,
            'member_id', m.member_id,
            'member_name', m.member_name,
            'member_phone', m.member_phone,
            'status', m.status,
            'joined_at', m.joined_at
        ) ORDER BY m.joined_at DESC
    ), '[]'::jsonb) INTO v_list
    FROM public.agency_members m
    WHERE m.agent_id = p_agent_id;

    RETURN v_list;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

GRANT EXECUTE ON FUNCTION public.get_agency_members_by_agent(UUID) TO anon, authenticated, service_role;

-- 5. Kebijakan RLS & Permission Akses Tabel
ALTER TABLE public.agency_members ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "Full Access agency_members" ON public.agency_members;
CREATE POLICY "Full Access agency_members" ON public.agency_members
FOR ALL TO anon, authenticated, service_role USING (true) WITH CHECK (true);
GRANT ALL ON TABLE public.agency_members TO anon, authenticated, service_role;

ALTER TABLE public.agency_profiles ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "Full Access agency_profiles" ON public.agency_profiles;
CREATE POLICY "Full Access agency_profiles" ON public.agency_profiles
FOR ALL TO anon, authenticated, service_role USING (true) WITH CHECK (true);
GRANT ALL ON TABLE public.agency_profiles TO anon, authenticated, service_role;

ALTER TABLE public.agent_category_markups ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "Full Access agent_category_markups" ON public.agent_category_markups;
CREATE POLICY "Full Access agent_category_markups" ON public.agent_category_markups
FOR ALL TO anon, authenticated, service_role USING (true) WITH CHECK (true);
GRANT ALL ON TABLE public.agent_category_markups TO anon, authenticated, service_role;

-- 6. Reload PostgREST Cache
NOTIFY pgrst, 'reload schema';

