-- =====================================================================
-- 21_fix_refund_status_constraint.sql
-- Memperbaiki check constraint app_wallet_transactions agar mengizinkan 'REFUNDED'
-- dan memastikan fungsi refund_app_wallet_transaction berjalan mulus
-- =====================================================================

-- 1. Lepas constraint lama dan pasang constraint baru yang mendukung 'REFUNDED'
ALTER TABLE public.app_wallet_transactions 
DROP CONSTRAINT IF EXISTS app_wallet_transactions_status_check;

ALTER TABLE public.app_wallet_transactions 
ADD CONSTRAINT app_wallet_transactions_status_check 
CHECK (status IN ('SUCCESS', 'PENDING', 'FAILED', 'REVERSED', 'REFUNDED'));

-- 2. Pastikan fungsi refund_app_wallet_transaction menangani update status dengan aman
CREATE OR REPLACE FUNCTION public.refund_app_wallet_transaction(
    p_transaction_code TEXT,
    p_reason TEXT DEFAULT 'Transaksi gagal diproses oleh sistem / provider'
)
RETURNS JSONB AS $$
DECLARE
    v_orig RECORD;
    v_refund_result JSONB;
    v_clean_title TEXT;
    v_title TEXT;
    v_clean_desc TEXT;
BEGIN
    SELECT * INTO v_orig
    FROM public.app_wallet_transactions
    WHERE transaction_code = p_transaction_code;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Transaksi dengan kode % tidak ditemukan', p_transaction_code;
    END IF;

    IF v_orig.type != 'DEBIT' THEN
        RAISE EXCEPTION 'Hanya transaksi DEBIT (pembayaran/pengeluaran) yang dapat di-refund';
    END IF;

    IF EXISTS (
        SELECT 1 FROM public.app_wallet_transactions 
        WHERE reference_id = p_transaction_code AND category = 'REFUND'
    ) THEN
        RAISE EXCEPTION 'Transaksi % sudah pernah di-refund sebelumnya', p_transaction_code;
    END IF;

    -- Bersihkan prefix: hapus "Pembelian ", "Refund Dana: ", atau "Refund "
    v_clean_title := REGEXP_REPLACE(v_orig.title, '^(Pembelian|Refund Dana:|Refund)\s*', '', 'i');
    v_title := 'Refund ' || v_clean_title;

    -- Bersihkan deskripsi agar hanya nomor tujuan murni
    v_clean_desc := REGEXP_REPLACE(REGEXP_REPLACE(v_orig.description, '^Tujuan:\s*', '', 'i'), '\s*\([^)]*\)$', '');
    IF v_clean_desc = '' THEN
        v_clean_desc := v_orig.description;
    END IF;

    -- Mutasi kredit refund ke dompet user
    v_refund_result := public.process_app_wallet_mutation(
        p_user_id      := v_orig.user_id,
        p_type         := 'CREDIT',
        p_category     := 'REFUND',
        p_amount       := v_orig.amount,
        p_title        := v_title,
        p_description  := v_clean_desc,
        p_reference_id := v_orig.transaction_code,
        p_admin_notes  := p_reason
    );

    -- Tandai status transaksi aslinya menjadi REVERSED / REFUNDED
    UPDATE public.app_wallet_transactions
    SET status = 'REVERSED'
    WHERE transaction_code = p_transaction_code;

    -- Update status di tabel orders jika ada
    UPDATE public.orders
    SET status = 'REFUNDED',
        error_message = p_reason
    WHERE order_code = p_transaction_code OR wallet_trx_code = p_transaction_code;

    RETURN v_refund_result;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Berikan izin eksekusi ke anon, authenticated, dan service_role
GRANT EXECUTE ON FUNCTION public.refund_app_wallet_transaction(TEXT, TEXT) TO anon, authenticated, service_role;
