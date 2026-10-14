-- =====================================================================
-- DEWACELL RBAC (Role-Based Access Control) Multi-Role Migration
-- Jalankan query ini di: Supabase Dashboard -> SQL Editor -> New Query
-- =====================================================================

-- 1. Buat Tabel Master Roles
CREATE TABLE IF NOT EXISTS public.roles (
    id TEXT PRIMARY KEY,                   -- Contoh: 'admin', 'cashier', 'finance', 'user'
    name TEXT NOT NULL,                    -- Nama display: 'Administrator', 'Kasir'
    description TEXT,                      -- Keterangan role
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 2. Buat Tabel Relasi Multi-Role (Many-to-Many antara User & Role)
CREATE TABLE IF NOT EXISTS public.user_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    role_id TEXT NOT NULL REFERENCES public.roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    assigned_by UUID REFERENCES auth.users(id),
    UNIQUE (user_id, role_id)
);

-- Indexing untuk query cepat
CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON public.user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON public.user_roles(role_id);

-- 3. Masukkan Daftar Role Default Sistem DewaCell
INSERT INTO public.roles (id, name, description) VALUES
    ('superadmin', 'Super Administrator', 'Akses penuh ke semua konfigurasi sistem dan server'),
    ('admin',      'Administrator',       'Mengelola produk PPOB, harga, dan laporan transaksi'),
    ('cashier',    'Kasir POS',           'Melayani penjualan pulsa, token PLN, dan top up wallet'),
    ('finance',    'Keuangan',            'Rekonsiliasi kas, mutasi saldo, dan laporan laba rugi'),
    ('user',       'Pelanggan / Member',  'Akses standar aplikasi client / member')
ON CONFLICT (id) DO UPDATE 
SET name = EXCLUDED.name, description = EXCLUDED.description;

-- 4. Function & Trigger: Otomatis Sinkronkan Roles ke JWT Token Supabase
-- Setiap kali tabel user_roles berubah (INSERT, UPDATE, DELETE),
-- trigger ini otomatis memperbarui auth.users.raw_app_meta_data.roles
-- sehingga token JWT user otomatis memiliki claim roles: ["admin", "cashier"]
CREATE OR REPLACE FUNCTION public.sync_user_roles_to_auth()
RETURNS TRIGGER AS $$
DECLARE
    target_user_id UUID;
    roles_array JSONB;
BEGIN
    IF (TG_OP = 'DELETE') THEN
        target_user_id := OLD.user_id;
    ELSE
        target_user_id := NEW.user_id;
    END IF;

    -- Ambil semua role milik user dalam bentuk JSON Array
    SELECT COALESCE(jsonb_agg(role_id), '["user"]'::jsonb)
    INTO roles_array
    FROM public.user_roles
    WHERE user_id = target_user_id;

    -- Update metadata di tabel auth.users
    UPDATE auth.users
    SET raw_app_meta_data = COALESCE(raw_app_meta_data, '{}'::jsonb) || jsonb_build_object('roles', roles_array)
    WHERE id = target_user_id;

    RETURN NULL;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Pasang Trigger ke tabel user_roles
DROP TRIGGER IF EXISTS trigger_sync_user_roles ON public.user_roles;
CREATE TRIGGER trigger_sync_user_roles
AFTER INSERT OR UPDATE OR DELETE ON public.user_roles
FOR EACH ROW EXECUTE FUNCTION public.sync_user_roles_to_auth();

-- 5. Helper Function: Memberikan Role ke User berdasarkan Email
CREATE OR REPLACE FUNCTION public.assign_user_role(p_email TEXT, p_role TEXT)
RETURNS TEXT AS $$
DECLARE
    v_user_id UUID;
BEGIN
    -- Cari user id dari tabel auth.users
    SELECT id INTO v_user_id FROM auth.users WHERE email = p_email LIMIT 1;
    
    IF v_user_id IS NULL THEN
        RETURN 'Error: User dengan email ' || p_email || ' tidak ditemukan';
    END IF;

    -- Pastikan role terdaftar
    IF NOT EXISTS (SELECT 1 FROM public.roles WHERE id = p_role) THEN
        RETURN 'Error: Role ' || p_role || ' tidak terdaftar di tabel roles';
    END IF;

    -- Insert ke user_roles
    INSERT INTO public.user_roles (user_id, role_id)
    VALUES (v_user_id, p_role)
    ON CONFLICT (user_id, role_id) DO NOTHING;

    RETURN 'Success: Role ' || p_role || ' berhasil diberikan ke ' || p_email;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 6. Helper Function: Menghapus Role dari User berdasarkan Email
CREATE OR REPLACE FUNCTION public.remove_user_role(p_email TEXT, p_role TEXT)
RETURNS TEXT AS $$
DECLARE
    v_user_id UUID;
BEGIN
    SELECT id INTO v_user_id FROM auth.users WHERE email = p_email LIMIT 1;
    
    IF v_user_id IS NULL THEN
        RETURN 'Error: User dengan email ' || p_email || ' tidak ditemukan';
    END IF;

    DELETE FROM public.user_roles 
    WHERE user_id = v_user_id AND role_id = p_role;

    RETURN 'Success: Role ' || p_role || ' berhasil dicabut dari ' || p_email;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- =====================================================================
-- CONTOH CARA PAKAI DI SUPABASE SQL EDITOR:
-- =====================================================================
-- A. Berikan multi-role ('admin' DAN 'cashier') ke user@gmail.com:
-- SELECT public.assign_user_role('user@gmail.com', 'admin');
-- SELECT public.assign_user_role('user@gmail.com', 'cashier');
--
-- B. Cek daftar role user:
-- SELECT u.email, r.id AS role_id, r.name AS role_name
-- FROM public.user_roles ur
-- JOIN auth.users u ON u.id = ur.user_id
-- JOIN public.roles r ON r.id = ur.role_id
-- WHERE u.email = 'user@gmail.com';
-- =====================================================================
