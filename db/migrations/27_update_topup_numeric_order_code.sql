-- =====================================================================
-- MIGRATION 27: SERAGAMKAN NO PESANAN TOPUP KE ANGKA SAJA (16 DIGIT)
-- Jalankan query ini di: Supabase Dashboard -> SQL Editor -> New Query
-- =====================================================================

-- 1. Perbarui Function request_app_wallet_topup agar generate kode angka murni 16 digit
CREATE OR REPLACE FUNCTION public.request_app_wallet_topup(
    p_user_id UUID,
    p_amount NUMERIC,
    p_channel TEXT,
    p_notes TEXT DEFAULT NULL
)
RETURNS JSONB AS $$
DECLARE
    v_wallet RECORD;
    v_trx_code TEXT;
    v_ref_id TEXT;
    v_trx_id UUID;
    v_channel TEXT;
BEGIN
    IF p_amount <= 0 THEN
        RAISE EXCEPTION 'Nominal top up harus lebih besar dari 0';
    END IF;

    v_channel := COALESCE(NULLIF(TRIM(p_channel), ''), 'fund_credit');

    -- Ambil atau buat wallet user
    SELECT * INTO v_wallet 
    FROM public.app_wallets 
    WHERE user_id = p_user_id;

    IF NOT FOUND THEN
        INSERT INTO public.app_wallets (user_id, balance, points)
        VALUES (p_user_id, 0, 0)
        RETURNING * INTO v_wallet;
    END IF;

    -- Generate kode transaksi unik & referensi invoice seragam angka murni 16 digit:
    -- Format: YYMMDDHH24MISS + 4 digit random (contoh: 2610171630128472)
    v_trx_code := TO_CHAR(NOW(), 'YYMMDDHH24MISS') || LPAD(FLOOR(RANDOM() * 9000 + 1000)::TEXT, 4, '0');
    v_ref_id   := TO_CHAR(NOW(), 'YYMMDDHH24MISS') || LPAD(FLOOR(RANDOM() * 9000 + 1000)::TEXT, 4, '0');

    -- Masukkan ke buku besar dengan status PENDING (Saldo belum bertambah)
    INSERT INTO public.app_wallet_transactions (
        transaction_code,
        user_id,
        type,
        category,
        amount,
        balance_before,
        balance_after,
        title,
        description,
        reference_id,
        admin_notes,
        status,
        created_at
    ) VALUES (
        v_trx_code,
        p_user_id,
        'CREDIT',
        'TOPUP',
        p_amount,
        v_wallet.balance,
        v_wallet.balance,
        'Permintaan Top Up Saldo',
        'Menunggu pembayaran dan verifikasi admin / transfer manual',
        v_ref_id,
        p_notes,
        'PENDING',
        NOW()
    ) RETURNING id INTO v_trx_id;

    RETURN jsonb_build_object(
        'success', true,
        'transaction_id', v_trx_id,
        'transaction_code', v_trx_code,
        'user_id', p_user_id,
        'amount', p_amount,
        'channel', v_channel,
        'payment_code', v_ref_id,
        'balance_current', v_wallet.balance,
        'status', 'PENDING',
        'message', 'Permintaan top up berhasil dibuat dengan status PENDING. Silakan selesaikan pembayaran atau tunggu persetujuan Admin.'
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

GRANT EXECUTE ON FUNCTION public.request_app_wallet_topup(UUID, NUMERIC, TEXT, TEXT) TO anon, authenticated, service_role;

-- 2. Bersihkan transaksi lama yang memiliki prefix VA- atau TOP- ke angka murni
UPDATE public.app_wallet_transactions
SET reference_id = REGEXP_REPLACE(reference_id, '[^0-9]', '', 'g')
WHERE reference_id ILIKE '%VA-%' 
   OR reference_id ILIKE '%FUNDCREDIT%';

UPDATE public.app_wallet_transactions
SET transaction_code = REGEXP_REPLACE(transaction_code, '[^0-9]', '', 'g')
WHERE transaction_code ILIKE 'TOP-%';
