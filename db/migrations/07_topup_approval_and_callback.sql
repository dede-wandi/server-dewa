-- =====================================================================
-- DEWACELL TOPUP ALUR APPROVAL & PAYMENT CALLBACK (MIGRATION 07)
-- Alur:
-- 1. User minta top up -> status PENDING, saldo belum bertambah.
-- 2. Super Admin approve / Payment gateway callback -> status SUCCESS, saldo bertambah secara atomik.
-- 3. Super Admin reject -> status FAILED, saldo tidak berubah.
-- =====================================================================

-- 1. FUNCTION: Request Top Up (User) -> Status PENDING
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

    v_channel := COALESCE(NULLIF(TRIM(p_channel), ''), 'BCA Virtual Account');

    -- Ambil atau buat wallet user
    SELECT * INTO v_wallet 
    FROM public.app_wallets 
    WHERE user_id = p_user_id;

    IF NOT FOUND THEN
        INSERT INTO public.app_wallets (user_id, balance, points)
        VALUES (p_user_id, 0, 0)
        RETURNING * INTO v_wallet;
    END IF;

    -- Generate kode transaksi unik & referensi invoice pembayaran
    v_trx_code := 'TOP-' || TO_CHAR(NOW(), 'YYYYMMDD') || '-' || LPAD(FLOOR(RANDOM() * 900000 + 100000)::TEXT, 6, '0');
    v_ref_id   := 'VA-' || UPPER(REGEXP_REPLACE(v_channel, '[^a-zA-Z0-9]', '', 'g')) || '-' || EXTRACT(EPOCH FROM NOW())::BIGINT;

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
        'Permintaan Top Up Saldo via ' || v_channel,
        'Menunggu pembayaran dan verifikasi admin / callback payment gateway',
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

-- 2. FUNCTION: Approve Top Up (Admin atau Payment Gateway Callback) -> Status SUCCESS & Saldo Bertambah
CREATE OR REPLACE FUNCTION public.approve_app_wallet_topup(
    p_transaction_code TEXT,
    p_admin_id UUID DEFAULT NULL,
    p_admin_notes TEXT DEFAULT NULL
)
RETURNS JSONB AS $$
DECLARE
    v_trx RECORD;
    v_wallet RECORD;
    v_bal_before NUMERIC(15, 2);
    v_bal_after NUMERIC(15, 2);
    v_admin_email TEXT;
BEGIN
    -- Kunci baris transaksi untuk mencegah double approve
    SELECT * INTO v_trx 
    FROM public.app_wallet_transactions 
    WHERE LOWER(transaction_code) = LOWER(TRIM(p_transaction_code))
       OR id::TEXT = TRIM(p_transaction_code)
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Transaksi top up dengan kode % tidak ditemukan', p_transaction_code;
    END IF;

    IF v_trx.status <> 'PENDING' THEN
        RAISE EXCEPTION 'Transaksi % tidak dapat disetujui karena status saat ini adalah %', p_transaction_code, v_trx.status;
    END IF;

    IF p_admin_id IS NOT NULL THEN
        SELECT email INTO v_admin_email FROM auth.users WHERE id = p_admin_id;
    END IF;
    IF v_admin_email IS NULL THEN
        v_admin_email := 'Admin Sistem / Payment Gateway';
    END IF;

    -- Kunci saldo user FOR UPDATE
    SELECT * INTO v_wallet 
    FROM public.app_wallets 
    WHERE user_id = v_trx.user_id 
    FOR UPDATE;

    IF NOT FOUND THEN
        INSERT INTO public.app_wallets (user_id, balance, points)
        VALUES (v_trx.user_id, 0, 0)
        RETURNING * INTO v_wallet;
    END IF;

    v_bal_before := v_wallet.balance;
    v_bal_after  := v_bal_before + v_trx.amount;

    -- Update saldo dompet
    UPDATE public.app_wallets 
    SET balance = v_bal_after, updated_at = NOW() 
    WHERE user_id = v_trx.user_id;

    -- Update status transaksi buku besar
    UPDATE public.app_wallet_transactions 
    SET 
        status = 'SUCCESS',
        balance_before = v_bal_before,
        balance_after = v_bal_after,
        admin_id = p_admin_id,
        admin_notes = COALESCE(p_admin_notes, 'Disetujui oleh ' || v_admin_email),
        title = REGEXP_REPLACE(v_trx.title, '^Permintaan\s+', '')
    WHERE id = v_trx.id;

    RETURN jsonb_build_object(
        'success', true,
        'transaction_id', v_trx.id,
        'transaction_code', v_trx.transaction_code,
        'user_id', v_trx.user_id,
        'amount', v_trx.amount,
        'balance_before', v_bal_before,
        'balance_after', v_bal_after,
        'status', 'SUCCESS',
        'approved_by', v_admin_email,
        'message', 'Top up saldo sebesar Rp ' || TO_CHAR(v_trx.amount, 'FM999,999,999') || ' berhasil disetujui dan saldo telah ditambahkan ke pengguna.'
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 3. FUNCTION: Reject Top Up (Admin) -> Status FAILED & Saldo Tetap
CREATE OR REPLACE FUNCTION public.reject_app_wallet_topup(
    p_transaction_code TEXT,
    p_admin_id UUID DEFAULT NULL,
    p_reason TEXT DEFAULT NULL
)
RETURNS JSONB AS $$
DECLARE
    v_trx RECORD;
    v_reason TEXT;
    v_admin_email TEXT;
BEGIN
    SELECT * INTO v_trx 
    FROM public.app_wallet_transactions 
    WHERE LOWER(transaction_code) = LOWER(TRIM(p_transaction_code))
       OR id::TEXT = TRIM(p_transaction_code)
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Transaksi top up dengan kode % tidak ditemukan', p_transaction_code;
    END IF;

    IF v_trx.status <> 'PENDING' THEN
        RAISE EXCEPTION 'Transaksi % tidak dapat ditolak karena status saat ini adalah %', p_transaction_code, v_trx.status;
    END IF;

    IF p_admin_id IS NOT NULL THEN
        SELECT email INTO v_admin_email FROM auth.users WHERE id = p_admin_id;
    END IF;
    IF v_admin_email IS NULL THEN
        v_admin_email := 'Admin Sistem';
    END IF;

    v_reason := COALESCE(NULLIF(TRIM(p_reason), ''), 'Ditolak oleh Admin (' || v_admin_email || ')');

    UPDATE public.app_wallet_transactions 
    SET 
        status = 'FAILED',
        admin_id = p_admin_id,
        admin_notes = v_reason
    WHERE id = v_trx.id;

    RETURN jsonb_build_object(
        'success', true,
        'transaction_id', v_trx.id,
        'transaction_code', v_trx.transaction_code,
        'user_id', v_trx.user_id,
        'amount', v_trx.amount,
        'status', 'FAILED',
        'reason', v_reason,
        'message', 'Permintaan top up ' || v_trx.transaction_code || ' berhasil ditolak.'
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 4. FUNCTION: Get Pending Top Ups
CREATE OR REPLACE FUNCTION public.get_pending_topups()
RETURNS JSONB AS $$
DECLARE
    v_res JSONB;
BEGIN
    SELECT jsonb_agg(
        jsonb_build_object(
            'id', t.id,
            'transaction_code', t.transaction_code,
            'user_id', t.user_id,
            'user_email', u.email,
            'amount', t.amount,
            'channel', t.reference_id,
            'title', t.title,
            'description', t.description,
            'status', t.status,
            'created_at', t.created_at
        ) ORDER BY t.created_at DESC
    ) INTO v_res
    FROM public.app_wallet_transactions t
    LEFT JOIN auth.users u ON u.id = t.user_id
    WHERE t.category = 'TOPUP' AND t.status = 'PENDING';

    RETURN COALESCE(v_res, '[]'::jsonb);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 5. Hak Akses
GRANT EXECUTE ON FUNCTION public.request_app_wallet_topup(UUID, NUMERIC, TEXT, TEXT) TO anon, authenticated, service_role;
GRANT EXECUTE ON FUNCTION public.approve_app_wallet_topup(TEXT, UUID, TEXT) TO anon, authenticated, service_role;
GRANT EXECUTE ON FUNCTION public.reject_app_wallet_topup(TEXT, UUID, TEXT) TO anon, authenticated, service_role;
GRANT EXECUTE ON FUNCTION public.get_pending_topups() TO anon, authenticated, service_role;

-- 6. Reload Cache
NOTIFY pgrst, 'reload schema';
