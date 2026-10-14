-- =====================================================================
-- DEWACELL DATABASE MIGRATION 05: FULL DATABASE CRUD (NO HARDCODE)
-- Migration: 05_wallet_database_crud.sql
-- Menghilangkan seluruh dummy/hardcode data dan menghubungkan sistem 100% ke PostgreSQL Supabase
-- Jalankan query ini di: Supabase Dashboard -> SQL Editor -> New Query
-- =====================================================================

-- 1. Buka Akses RLS untuk app_wallets, app_wallet_transactions, dan app_point_transactions
ALTER TABLE public.app_wallets ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "app_wallets_policy" ON public.app_wallets;
CREATE POLICY "app_wallets_policy" ON public.app_wallets FOR ALL TO anon, authenticated USING (true) WITH CHECK (true);

ALTER TABLE public.app_wallet_transactions ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "app_wallet_transactions_policy" ON public.app_wallet_transactions;
CREATE POLICY "app_wallet_transactions_policy" ON public.app_wallet_transactions FOR ALL TO anon, authenticated USING (true) WITH CHECK (true);

ALTER TABLE public.app_point_transactions ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "app_point_transactions_policy" ON public.app_point_transactions;
CREATE POLICY "app_point_transactions_policy" ON public.app_point_transactions FOR ALL TO anon, authenticated USING (true) WITH CHECK (true);

-- 2. Pastikan tabel app_wallets otomatis memiliki baris untuk setiap user di auth.users (Saldo awal murni 0)
INSERT INTO public.app_wallets (user_id, balance, points)
SELECT id, 0, 0
FROM auth.users
ON CONFLICT (user_id) DO NOTHING;

-- 3. FUNCTION ATOMIK: Ambil atau Inisialisasi Dompet Pengguna Real
CREATE OR REPLACE FUNCTION public.get_user_wallet_info(p_user_id UUID)
RETURNS JSONB AS $$
DECLARE
    v_wallet RECORD;
BEGIN
    SELECT * INTO v_wallet FROM public.app_wallets WHERE user_id = p_user_id;
    IF NOT FOUND THEN
        INSERT INTO public.app_wallets (user_id, balance, points, is_locked)
        VALUES (p_user_id, 0, 0, false)
        RETURNING * INTO v_wallet;
    END IF;

    RETURN to_jsonb(v_wallet);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 4. FUNCTION ATOMIK: Ambil Seluruh Pengguna Terdaftar Beserta Saldo Real & Role Real (Untuk Super Admin)
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
            'updated_at', COALESCE(w.updated_at, u.created_at)
        ) ORDER BY u.created_at DESC
    ) INTO v_list
    FROM auth.users u
    LEFT JOIN public.app_wallets w ON w.user_id = u.id;

    RETURN COALESCE(v_list, '[]'::jsonb);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 5. FUNCTION ATOMIK: Ambil Riwayat Transaksi Real Pengguna
CREATE OR REPLACE FUNCTION public.get_user_wallet_transactions(p_user_id UUID, p_limit INT DEFAULT 50)
RETURNS JSONB AS $$
DECLARE
    v_list JSONB;
BEGIN
    SELECT jsonb_agg(to_jsonb(t)) INTO v_list
    FROM (
        SELECT * FROM public.app_wallet_transactions
        WHERE user_id = p_user_id
        ORDER BY created_at DESC
        LIMIT p_limit
    ) t;

    RETURN COALESCE(v_list, '[]'::jsonb);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 6. FUNCTION ATOMIK: Ambil Seluruh Buku Besar Mutasi Platform Real (Untuk Super Admin)
CREATE OR REPLACE FUNCTION public.get_all_app_transactions(p_limit INT DEFAULT 200)
RETURNS JSONB AS $$
DECLARE
    v_list JSONB;
BEGIN
    SELECT jsonb_agg(to_jsonb(t)) INTO v_list
    FROM (
        SELECT * FROM public.app_wallet_transactions
        ORDER BY created_at DESC
        LIMIT p_limit
    ) t;

    RETURN COALESCE(v_list, '[]'::jsonb);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 7. FUNCTION ATOMIK: Ambil Seluruh Mutasi DewaPoin Real (Untuk Super Admin)
CREATE OR REPLACE FUNCTION public.get_all_point_transactions(p_limit INT DEFAULT 200)
RETURNS JSONB AS $$
DECLARE
    v_list JSONB;
BEGIN
    SELECT jsonb_agg(to_jsonb(t)) INTO v_list
    FROM (
        SELECT * FROM public.app_point_transactions
        ORDER BY created_at DESC
        LIMIT p_limit
    ) t;

    RETURN COALESCE(v_list, '[]'::jsonb);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 8. FUNCTION ATOMIK: Lock / Unlock Dompet Pengguna oleh Admin
CREATE OR REPLACE FUNCTION public.admin_toggle_wallet_lock(p_target_user_id UUID, p_locked BOOLEAN)
RETURNS JSONB AS $$
DECLARE
    v_wallet RECORD;
BEGIN
    UPDATE public.app_wallets
    SET is_locked = p_locked, updated_at = NOW()
    WHERE user_id = p_target_user_id
    RETURNING * INTO v_wallet;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Dompet pengguna tidak ditemukan';
    END IF;

    RETURN to_jsonb(v_wallet);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 9. Reload cache schema PostgREST Supabase
NOTIFY pgrst, 'reload schema';
NOTIFY pgrst, 'reload config';
