-- =====================================================================
-- MIGRATION 20: DEDICATED ORDERS TABLE & 16-DIGIT NUMERIC CODES
-- Standard Gojek: Title bersih nama produk, Deskripsi nomor tujuan murni,
-- Title refund: "Refund [Nama Produk]", Format kode: 16 Digit Angka Bersih
-- =====================================================================

-- 1. TABEL ORDERS RESMI & PERMANEN
CREATE TABLE IF NOT EXISTS public.orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_code VARCHAR(32) UNIQUE NOT NULL,      -- 16-digit numerik murni (misal: 2610152206151234)
    user_id UUID REFERENCES auth.users(id) ON DELETE CASCADE,
    user_email TEXT,
    product_code TEXT NOT NULL,
    product_name TEXT NOT NULL,
    product_category TEXT NOT NULL DEFAULT 'pulsa',
    target_account TEXT NOT NULL,                -- No HP / No Meter PLN / ID Game murni
    price NUMERIC(15, 2) NOT NULL,
    points_earned BIGINT DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING', -- PENDING, PROCESSING, SUCCESS, FAILED, REFUNDED
    serial_number TEXT,                          -- SN / Stroom Token PLN
    wallet_trx_code VARCHAR(32),                 -- Kode transaksi dompet terkait (16 digit)
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_orders_user_id ON public.orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_order_code ON public.orders(order_code);
CREATE INDEX IF NOT EXISTS idx_orders_status ON public.orders(status);

ALTER TABLE public.orders ENABLE ROW LEVEL SECURITY;

DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies WHERE tablename = 'orders' AND policyname = 'Users can read own orders'
    ) THEN
        CREATE POLICY "Users can read own orders"
        ON public.orders FOR SELECT
        TO authenticated
        USING (auth.uid() = user_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_policies WHERE tablename = 'orders' AND policyname = 'Admins can manage all orders'
    ) THEN
        CREATE POLICY "Admins can manage all orders"
        ON public.orders FOR ALL
        TO authenticated
        USING (
            EXISTS (
                SELECT 1 FROM public.user_roles ur
                JOIN public.roles r ON ur.role_id = r.id
                WHERE ur.user_id = auth.uid() AND r.name IN ('admin', 'superadmin')
            )
        );
    END IF;
END $$;

-- 2. FUNGSI MUTASI SALDO (16-DIGIT NUMERIK MURNI TANPA PREFIX)
CREATE OR REPLACE FUNCTION public.process_app_wallet_mutation(
    p_user_id UUID,
    p_type TEXT,            -- 'CREDIT' atau 'DEBIT'
    p_category TEXT,        -- 'TOPUP', 'PAYMENT', 'REFUND', 'ADJUSTMENT', dll
    p_amount NUMERIC,
    p_title TEXT,
    p_description TEXT,
    p_reference_id TEXT DEFAULT NULL,
    p_admin_id UUID DEFAULT NULL,
    p_admin_notes TEXT DEFAULT NULL
)
RETURNS JSONB AS $$
DECLARE
    v_wallet RECORD;
    v_balance_before NUMERIC(15, 2);
    v_balance_after NUMERIC(15, 2);
    v_trx_code TEXT;
    v_trx_id UUID;
BEGIN
    IF p_amount <= 0 THEN
        RAISE EXCEPTION 'Nominal transaksi harus lebih besar dari 0';
    END IF;

    IF p_type NOT IN ('CREDIT', 'DEBIT') THEN
        RAISE EXCEPTION 'Tipe mutasi tidak valid. Harus CREDIT atau DEBIT';
    END IF;

    SELECT * INTO v_wallet 
    FROM public.app_wallets 
    WHERE user_id = p_user_id 
    FOR UPDATE;

    IF NOT FOUND THEN
        INSERT INTO public.app_wallets (user_id, balance, points)
        VALUES (p_user_id, 0, 0)
        RETURNING * INTO v_wallet;
    END IF;

    IF v_wallet.is_locked THEN
        RAISE EXCEPTION 'Dompet saldo aplikasi sedang dibekukan / dikunci. Hubungi CS DewaCell.';
    END IF;

    v_balance_before := v_wallet.balance;

    IF p_type = 'DEBIT' THEN
        IF v_balance_before < p_amount THEN
            RAISE EXCEPTION 'Saldo tidak mencukupi. Saldo saat ini: Rp %, dibutuhkan: Rp %', v_balance_before, p_amount;
        END IF;
        v_balance_after := v_balance_before - p_amount;
    ELSE
        v_balance_after := v_balance_before + p_amount;
    END IF;

    -- Standar Gojek: 16 Digit Numerik Murni (YYMMDDHHMMSS + 4 Random Digits)
    v_trx_code := TO_CHAR(NOW(), 'YYMMDDHH24MISS') || LPAD(FLOOR(RANDOM() * 9000 + 1000)::TEXT, 4, '0');

    UPDATE public.app_wallets
    SET balance = v_balance_after,
        updated_at = NOW()
    WHERE user_id = p_user_id;

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
        admin_id,
        admin_notes,
        status,
        created_at
    ) VALUES (
        v_trx_code,
        p_user_id,
        p_type,
        p_category,
        p_amount,
        v_balance_before,
        v_balance_after,
        p_title,
        p_description,
        p_reference_id,
        p_admin_id,
        p_admin_notes,
        'SUCCESS',
        NOW()
    ) RETURNING id INTO v_trx_id;

    RETURN jsonb_build_object(
        'success', true,
        'transaction_id', v_trx_id,
        'transaction_code', v_trx_code,
        'balance_before', v_balance_before,
        'balance_after', v_balance_after,
        'amount', p_amount,
        'type', p_type,
        'category', p_category
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 3. FUNGSI REFUND (TITTLE: Refund [Produk], DESKRIPSI: Nomor HP Murni, STATUS: REFUNDED)
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

    -- Bersihkan prefix lama jika ada: hapus "Pembelian ", "Refund Dana: ", atau "Refund "
    v_clean_title := REGEXP_REPLACE(v_orig.title, '^(Pembelian|Refund Dana:|Refund)\s*', '', 'i');
    v_title := 'Refund ' || v_clean_title;

    -- Bersihkan deskripsi agar hanya nomor tujuan murni (hapus "Tujuan: " dan nama brand dalam kurung)
    v_clean_desc := REGEXP_REPLACE(REGEXP_REPLACE(v_orig.description, '^Tujuan:\s*', '', 'i'), '\s*\([^)]*\)$', '');
    IF v_clean_desc = '' THEN
        v_clean_desc := v_orig.description;
    END IF;

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

    -- Tandai status transaksi aslinya menjadi REFUNDED
    UPDATE public.app_wallet_transactions
    SET status = 'REFUNDED'
    WHERE transaction_code = p_transaction_code;

    -- Tandai juga di tabel orders jika ada
    UPDATE public.orders
    SET status = 'REFUNDED',
        error_message = p_reason
    WHERE order_code = p_transaction_code OR wallet_trx_code = p_transaction_code;

    RETURN v_refund_result;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 4. BERSIHKAN DATA LAMA DI DATABASE AGAR SEMUA PREFIX HILANG (JADI NOMOR SAJA)
UPDATE public.app_wallet_transactions
SET transaction_code = REGEXP_REPLACE(transaction_code, '^(ORD|TRX|TOP|REF|ADJ|TFO|TFI)-', '')
WHERE transaction_code ~ '^(ORD|TRX|TOP|REF|ADJ|TFO|TFI)-';

UPDATE public.app_wallet_transactions
SET reference_id = REGEXP_REPLACE(reference_id, '^(ORD|TRX|TOP|REF|ADJ|TFO|TFI)-', '')
WHERE reference_id ~ '^(ORD|TRX|TOP|REF|ADJ|TFO|TFI)-';

-- Bersihkan title lama di app_wallet_transactions
UPDATE public.app_wallet_transactions
SET title = REGEXP_REPLACE(title, '^Pembelian\s+', '')
WHERE title ~* '^Pembelian\s+';

UPDATE public.app_wallet_transactions
SET title = REGEXP_REPLACE(title, '^Refund Dana:\s*(Pembelian\s*)?', 'Refund ')
WHERE title ~* '^Refund Dana:';

-- Bersihkan deskripsi lama agar nomor murni
UPDATE public.app_wallet_transactions
SET description = REGEXP_REPLACE(REGEXP_REPLACE(description, '^Tujuan:\s*', ''), '\s*\([^)]*\)$', '')
WHERE description ~* '^Tujuan:\s*';

-- Bersihkan deskripsi refund lama yang berisi kalimat panjang "Pengembalian saldo atas..."
UPDATE public.app_wallet_transactions
SET description = 'Pesanan ditolak server provider'
WHERE description ~* '^Pengembalian (saldo|dana) atas';
