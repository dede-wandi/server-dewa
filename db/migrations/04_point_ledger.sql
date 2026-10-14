-- =====================================================================
-- DEWACELL DEWAPOIN REWARD & BUKU BESAR MUTASI POIN
-- Migration: 04_point_ledger.sql
-- Mencatat seluruh riwayat mutasi perolehan poin (cashback/reward) & penukaran belanja pakai poin
-- Jalankan query ini di: Supabase Dashboard -> SQL Editor -> New Query
-- =====================================================================

-- 1. Buat Tabel Buku Besar Mutasi Poin (DewaPoin Ledger)
CREATE TABLE IF NOT EXISTS public.app_point_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_code TEXT NOT NULL UNIQUE,                                       -- Contoh: PTS-20261014-001
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('CREDIT', 'DEBIT')),                      -- CREDIT = Poin Masuk, DEBIT = Poin Keluar
    category TEXT NOT NULL CHECK (category IN (
        'CASHBACK',     -- Cashback reward belanja pulsa / PPOB / FnB
        'REWARD',       -- Bonus reward misi / promo
        'REDEEM',       -- Penukaran poin untuk diskon belanja
        'ADJUSTMENT',   -- Penyesuaian poin oleh Super Admin
        'EXPIRED'       -- Poin hangus
    )),
    amount BIGINT NOT NULL CHECK (amount > 0),                                   -- Jumlah poin
    points_before BIGINT NOT NULL,                                               -- Saldo poin sebelum
    points_after BIGINT NOT NULL,                                                -- Saldo poin sesudah
    title TEXT NOT NULL,                                                         -- Nama mutasi (misal: "Reward Cashback Pulsa", "Diskon Belanja Pakai Poin")
    description TEXT NOT NULL,                                                   -- Keterangan rinci
    reference_id TEXT,                                                           -- Kode transaksi terkait
    status TEXT NOT NULL DEFAULT 'SUCCESS' CHECK (status IN ('SUCCESS', 'CANCELLED')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexing
CREATE INDEX IF NOT EXISTS idx_app_point_transactions_user_id ON public.app_point_transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_app_point_transactions_created_at ON public.app_point_transactions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_app_point_transactions_type ON public.app_point_transactions(type);

-- 2. Function Tukar Poin untuk Belanja (Atomik)
CREATE OR REPLACE FUNCTION public.redeem_app_points(
    p_user_id UUID,
    p_points BIGINT,
    p_title TEXT DEFAULT 'Penukaran DewaPoin untuk Diskon Belanja',
    p_description TEXT DEFAULT 'Potongan harga belanja menggunakan DewaPoin'
)
RETURNS JSONB AS $$
DECLARE
    v_wallet RECORD;
    v_pts_before BIGINT;
    v_pts_after BIGINT;
    v_trx_code TEXT;
    v_trx_id UUID;
BEGIN
    IF p_points <= 0 THEN
        RAISE EXCEPTION 'Jumlah poin harus lebih besar dari 0';
    END IF;

    SELECT * INTO v_wallet 
    FROM public.app_wallets 
    WHERE user_id = p_user_id 
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Dompet pengguna tidak ditemukan';
    END IF;

    IF v_wallet.points < p_points THEN
        RAISE EXCEPTION 'Poin tidak mencukupi. Poin Anda: %, Dibutuhkan: %', v_wallet.points, p_points;
    END IF;

    v_pts_before := v_wallet.points;
    v_pts_after := v_pts_before - p_points;

    -- Update poin
    UPDATE public.app_wallets
    SET points = v_pts_after,
        updated_at = NOW()
    WHERE user_id = p_user_id;

    v_trx_code := 'PTS-' || TO_CHAR(NOW(), 'YYYYMMDD') || '-' || LPAD(FLOOR(RANDOM() * 900000 + 100000)::TEXT, 6, '0');

    -- Catat riwayat
    INSERT INTO public.app_point_transactions (
        transaction_code,
        user_id,
        type,
        category,
        amount,
        points_before,
        points_after,
        title,
        description,
        status,
        created_at
    ) VALUES (
        v_trx_code,
        p_user_id,
        'DEBIT',
        'REDEEM',
        p_points,
        v_pts_before,
        v_pts_after,
        p_title,
        p_description,
        'SUCCESS',
        NOW()
    ) RETURNING id INTO v_trx_id;

    RETURN jsonb_build_object(
        'success', true,
        'transaction_id', v_trx_id,
        'transaction_code', v_trx_code,
        'points_redeemed', p_points,
        'points_balance', v_pts_after,
        'created_at', NOW()
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
