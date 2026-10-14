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
    p_new_role TEXT DEFAULT NULL,
    p_user_metadata JSONB DEFAULT NULL
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
            raw_user_meta_data = COALESCE(raw_user_meta_data, '{}'::jsonb) || jsonb_build_object('role', LOWER(TRIM(p_new_role))),
            updated_at = NOW()
        WHERE id = p_user_id;
    END IF;

    -- Update Profil & User Metadata jika diisi (nama, telepon, alamat, dsb)
    IF p_user_metadata IS NOT NULL THEN
        UPDATE auth.users
        SET 
            raw_user_meta_data = COALESCE(raw_user_meta_data, '{}'::jsonb) || p_user_metadata,
            updated_at = NOW()
        WHERE id = p_user_id;
    END IF;

    SELECT id, email, raw_app_meta_data->>'role' as role, raw_user_meta_data INTO v_user FROM auth.users WHERE id = p_user_id;

    RETURN jsonb_build_object(
        'success', true,
        'user_id', v_user.id,
        'email', v_user.email,
        'role', v_user.role,
        'user_metadata', v_user.raw_user_meta_data,
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

-- 5. FUNCTION: Super Admin Ambil Detail Lengkap Profil & Metadata User
CREATE OR REPLACE FUNCTION public.admin_get_user_detail(p_user_id UUID)
RETURNS JSONB AS $$
DECLARE
    v_user RECORD;
    v_wallet RECORD;
BEGIN
    SELECT * INTO v_user FROM auth.users WHERE id = p_user_id;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    SELECT * INTO v_wallet FROM public.app_wallets WHERE user_id = p_user_id;

    RETURN jsonb_build_object(
        'user_id', v_user.id,
        'user_email', v_user.email,
        'role', COALESCE(v_user.raw_app_meta_data->>'role', v_user.raw_app_meta_data->'roles'->>0, 'user'),
        'balance', COALESCE(v_wallet.balance, 0),
        'points', COALESCE(v_wallet.points, 0),
        'is_locked', COALESCE(v_wallet.is_locked, false),
        'created_at', v_user.created_at,
        'updated_at', COALESCE(v_wallet.updated_at, v_user.created_at),
        'user_metadata', COALESCE(v_user.raw_user_meta_data, '{}'::jsonb),
        'app_metadata', COALESCE(v_user.raw_app_meta_data, '{}'::jsonb)
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 6. Update get_all_registered_users_wallets agar menyertakan user_metadata & app_metadata
CREATE OR REPLACE FUNCTION public.get_all_registered_users_wallets()
RETURNS JSONB AS $$
DECLARE
    v_list JSONB;
BEGIN
    SELECT jsonb_agg(
        jsonb_build_object(
            'user_id', u.id,
            'user_email', u.email,
            'role', COALESCE(u.raw_app_meta_data->>'role', u.raw_app_meta_data->'roles'->>0, 'user'),
            'balance', COALESCE(w.balance, 0),
            'points', COALESCE(w.points, 0),
            'is_locked', COALESCE(w.is_locked, false),
            'created_at', u.created_at,
            'updated_at', COALESCE(w.updated_at, u.created_at),
            'user_metadata', COALESCE(u.raw_user_meta_data, '{}'::jsonb),
            'app_metadata', COALESCE(u.raw_app_meta_data, '{}'::jsonb)
        ) ORDER BY u.created_at DESC
    ) INTO v_list
    FROM auth.users u
    LEFT JOIN public.app_wallets w ON w.user_id = u.id;

    RETURN COALESCE(v_list, '[]'::jsonb);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 7. FUNCTION: Super Admin Penyesuaian Saldo (Koreksi)
CREATE OR REPLACE FUNCTION public.admin_adjust_app_wallet(
    p_target_user_id UUID,
    p_type TEXT,            -- 'CREDIT' atau 'DEBIT'
    p_amount NUMERIC,
    p_reason TEXT,
    p_admin_id UUID DEFAULT NULL,
    p_admin_notes TEXT DEFAULT NULL
)
RETURNS JSONB AS $$
DECLARE
    v_title TEXT;
    v_desc TEXT;
    v_admin_email TEXT;
    v_result JSONB;
BEGIN
    IF p_admin_id IS NOT NULL THEN
        SELECT email INTO v_admin_email FROM auth.users WHERE id = p_admin_id;
    END IF;
    IF v_admin_email IS NULL THEN
        v_admin_email := 'Admin Sistem';
    END IF;

    IF p_reason IS NULL OR TRIM(p_reason) = '' THEN
        RAISE EXCEPTION 'Alasan penyesuaian saldo wajib diisi untuk kepatuhan audit pembukuan';
    END IF;

    IF p_type = 'CREDIT' THEN
        v_title := 'Penyesuaian Saldo oleh Super Admin (Kredit/Penambahan)';
        v_desc  := 'Penambahan saldo sebesar Rp ' || TO_CHAR(p_amount, 'FM999,999,999') || 
                   ' oleh Admin (' || v_admin_email || '). Alasan: ' || p_reason;
    ELSE
        v_title := 'Penyesuaian Saldo oleh Super Admin (Debet/Koreksi Salah Input)';
        v_desc  := 'Pemotongan saldo sebesar Rp ' || TO_CHAR(p_amount, 'FM999,999,999') || 
                   ' karena koreksi pembukuan oleh Admin (' || v_admin_email || '). Alasan: ' || p_reason;
    END IF;

    v_result := public.process_app_wallet_mutation(
        p_user_id      := p_target_user_id,
        p_type         := p_type,
        p_category     := 'ADJUSTMENT',
        p_amount       := p_amount,
        p_title        := v_title,
        p_description  := v_desc,
        p_reference_id := NULL,
        p_admin_id     := p_admin_id,
        p_admin_notes  := COALESCE(p_admin_notes, p_reason)
    );

    RETURN v_result;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 8. Berikan hak akses eksekusi ke anon, authenticated, dan service_role
GRANT EXECUTE ON FUNCTION public.admin_reset_user_password(UUID, TEXT) TO anon, authenticated, service_role;
GRANT EXECUTE ON FUNCTION public.admin_update_user_email(UUID, TEXT) TO anon, authenticated, service_role;
GRANT EXECUTE ON FUNCTION public.admin_update_user(UUID, TEXT, TEXT, TEXT, JSONB) TO anon, authenticated, service_role;
GRANT EXECUTE ON FUNCTION public.admin_delete_user(UUID) TO anon, authenticated, service_role;
GRANT EXECUTE ON FUNCTION public.admin_get_user_detail(UUID) TO anon, authenticated, service_role;
GRANT EXECUTE ON FUNCTION public.get_all_registered_users_wallets() TO anon, authenticated, service_role;
GRANT EXECUTE ON FUNCTION public.admin_adjust_app_wallet(UUID, TEXT, NUMERIC, TEXT, UUID, TEXT) TO anon, authenticated, service_role;

-- 9. Reload cache schema PostgREST
NOTIFY pgrst, 'reload schema';
