-- =====================================================================
-- DEWACELL DATABASE MIGRATION 06: SUPERADMIN USER CONTROL (FULL AUTHORITY)
-- Migration: 06_superadmin_user_control.sql
-- Memberikan wewenang penuh kepada Super Admin untuk:
-- 1. Reset password pengguna tanpa perlu password lama
-- 2. Ubah alamat email pengguna
-- 3. Update profil / role pengguna langsung di PostgreSQL auth.users
-- 4. Hapus pengguna dan membersihkan data terkait
-- Jalankan query ini di: Supabase Dashboard -> SQL Editor -> New Query
-- =====================================================================

-- Pastikan ekstensi pgcrypto aktif untuk enkripsi password bcrypt
CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA extensions;

-- 1. FUNCTION: Super Admin Reset Password User
CREATE OR REPLACE FUNCTION public.admin_reset_user_password(
    p_user_id UUID,
    p_new_password TEXT
)
RETURNS JSONB AS $$
DECLARE
    v_user RECORD;
    v_hash TEXT;
BEGIN
    IF LENGTH(TRIM(p_new_password)) < 6 THEN
        RAISE EXCEPTION 'Password baru minimal harus 6 karakter';
    END IF;

    -- Enkripsi password baru menggunakan bcrypt via pgcrypto
    v_hash := extensions.crypt(p_new_password, extensions.gen_salt('bf'));

    UPDATE auth.users
    SET 
        encrypted_password = v_hash,
        updated_at = NOW()
    WHERE id = p_user_id
    RETURNING id, email INTO v_user;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Pengguna dengan ID % tidak ditemukan', p_user_id;
    END IF;

    RETURN jsonb_build_object(
        'success', true,
        'user_id', v_user.id,
        'user_email', v_user.email,
        'message', 'Password pengguna berhasil direset oleh Super Admin'
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 2. FUNCTION: Super Admin Ubah Email User
CREATE OR REPLACE FUNCTION public.admin_update_user_email(
    p_user_id UUID,
    p_new_email TEXT
)
RETURNS JSONB AS $$
DECLARE
    v_clean_email TEXT;
    v_user RECORD;
BEGIN
    v_clean_email := LOWER(TRIM(p_new_email));
    IF v_clean_email = '' OR POSITION('@' IN v_clean_email) = 0 THEN
        RAISE EXCEPTION 'Format email tidak valid';
    END IF;

    -- Pastikan email belum dipakai user lain
    IF EXISTS (SELECT 1 FROM auth.users WHERE LOWER(email) = v_clean_email AND id <> p_user_id) THEN
        RAISE EXCEPTION 'Email % sudah terdaftar oleh pengguna lain', v_clean_email;
    END IF;

    UPDATE auth.users
    SET 
        email = v_clean_email,
        email_change = '',
        email_confirmed_at = COALESCE(email_confirmed_at, NOW()),
        updated_at = NOW()
    WHERE id = p_user_id
    RETURNING id, email INTO v_user;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Pengguna dengan ID % tidak ditemukan', p_user_id;
    END IF;

    -- Update update_at di dompet jika ada
    UPDATE public.app_wallets
    SET updated_at = NOW()
    WHERE user_id = p_user_id;

    RETURN jsonb_build_object(
        'success', true,
        'user_id', v_user.id,
        'new_email', v_user.email,
        'message', 'Email pengguna berhasil diperbarui oleh Super Admin'
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 3. FUNCTION: Super Admin Update Lengkap User (Email, Password, Role)
CREATE OR REPLACE FUNCTION public.admin_update_user(
    p_user_id UUID,
    p_new_email TEXT DEFAULT NULL,
    p_new_password TEXT DEFAULT NULL,
    p_new_role TEXT DEFAULT NULL
)
RETURNS JSONB AS $$
DECLARE
    v_user RECORD;
    v_clean_email TEXT;
BEGIN
    -- Validasi keberadaan user
    SELECT * INTO v_user FROM auth.users WHERE id = p_user_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Pengguna dengan ID % tidak ditemukan', p_user_id;
    END IF;

    -- Update Email jika diisi
    IF p_new_email IS NOT NULL AND TRIM(p_new_email) <> '' THEN
        v_clean_email := LOWER(TRIM(p_new_email));
        IF EXISTS (SELECT 1 FROM auth.users WHERE LOWER(email) = v_clean_email AND id <> p_user_id) THEN
            RAISE EXCEPTION 'Email % sudah terdaftar oleh pengguna lain', v_clean_email;
        END IF;

        UPDATE auth.users
        SET 
            email = v_clean_email,
            email_change = '',
            email_confirmed_at = COALESCE(email_confirmed_at, NOW()),
            updated_at = NOW()
        WHERE id = p_user_id;
    END IF;

    -- Update Password jika diisi
    IF p_new_password IS NOT NULL AND TRIM(p_new_password) <> '' THEN
        IF LENGTH(TRIM(p_new_password)) < 6 THEN
            RAISE EXCEPTION 'Password minimal 6 karakter';
        END IF;

        UPDATE auth.users
        SET 
            encrypted_password = extensions.crypt(p_new_password, extensions.gen_salt('bf')),
            updated_at = NOW()
        WHERE id = p_user_id;
    END IF;

    -- Update Role jika diisi
    IF p_new_role IS NOT NULL AND TRIM(p_new_role) <> '' THEN
        UPDATE auth.users
        SET 
            raw_app_meta_data = COALESCE(raw_app_meta_data, '{}'::jsonb) || jsonb_build_object('role', LOWER(TRIM(p_new_role))),
            updated_at = NOW()
        WHERE id = p_user_id;
    END IF;

    SELECT id, email, raw_app_meta_data->>'role' as role INTO v_user FROM auth.users WHERE id = p_user_id;

    RETURN jsonb_build_object(
        'success', true,
        'user_id', v_user.id,
        'email', v_user.email,
        'role', v_user.role,
        'message', 'Data pengguna berhasil diperbarui oleh Super Admin'
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 4. FUNCTION: Super Admin Hapus User
CREATE OR REPLACE FUNCTION public.admin_delete_user(
    p_user_id UUID
)
RETURNS JSONB AS $$
BEGIN
    -- Hapus dompet dan transaksi terkait
    DELETE FROM public.app_point_transactions WHERE user_id = p_user_id;
    DELETE FROM public.app_wallet_transactions WHERE user_id = p_user_id;
    DELETE FROM public.app_wallets WHERE user_id = p_user_id;

    -- Hapus dari auth.users
    DELETE FROM auth.users WHERE id = p_user_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Pengguna dengan ID % tidak ditemukan', p_user_id;
    END IF;

    RETURN jsonb_build_object(
        'success', true,
        'user_id', p_user_id,
        'message', 'Pengguna dan seluruh data terkait berhasil dihapus oleh Super Admin'
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 5. Berikan hak akses eksekusi ke anon, authenticated, dan service_role
GRANT EXECUTE ON FUNCTION public.admin_reset_user_password(UUID, TEXT) TO anon, authenticated, service_role;
GRANT EXECUTE ON FUNCTION public.admin_update_user_email(UUID, TEXT) TO anon, authenticated, service_role;
GRANT EXECUTE ON FUNCTION public.admin_update_user(UUID, TEXT, TEXT, TEXT) TO anon, authenticated, service_role;
GRANT EXECUTE ON FUNCTION public.admin_delete_user(UUID) TO anon, authenticated, service_role;

-- 6. Reload cache schema PostgREST
NOTIFY pgrst, 'reload schema';
