-- =====================================================================
-- DEWACELL SALDO REAL PLATFORM & BUKU BESAR MUTASI (DEWAPAY)
-- Migration: 03_wallet_and_ledger.sql
-- TABEL TERPISAH (OPSI 1): Menjaga tabel 'wallets' & 'wallet_mutations' milik POSDEWA tetap aman 100%
-- Jalankan query ini di: Supabase Dashboard -> SQL Editor -> New Query
-- =====================================================================

-- 1. Buat Tabel Master Saldo Real Aplikasi (DewaPay Wallet)
CREATE TABLE IF NOT EXISTS public.app_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE REFERENCES auth.users(id) ON DELETE CASCADE,
    balance NUMERIC(15, 2) NOT NULL DEFAULT 0 CHECK (balance >= 0),
    points BIGINT NOT NULL DEFAULT 0 CHECK (points >= 0),
    is_locked BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexing untuk query cepat saldo user
CREATE INDEX IF NOT EXISTS idx_app_wallets_user_id ON public.app_wallets(user_id);

-- 2. Buat Tabel Buku Besar Mutasi Saldo Real Aplikasi (App Ledger)
-- Mencatat seluruh riwayat arus kas debit & kredit dengan nama transaksi yang transparan dan jelas
CREATE TABLE IF NOT EXISTS public.app_wallet_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_code TEXT NOT NULL UNIQUE,                                       -- Contoh: TRX-20261014-001, REF-..., ADJ-...
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('CREDIT', 'DEBIT')),                      -- CREDIT = Uang Masuk, DEBIT = Uang Keluar
    category TEXT NOT NULL CHECK (category IN (
        'TOPUP',        -- Isi saldo
        'PAYMENT',      -- Pembayaran produk PPOB / FnB
        'REFUND',       -- Pengembalian dana saldo (transaksi gagal/batal)
        'ADJUSTMENT',   -- Penyesuaian saldo manual oleh Super Admin (koreksi salah input/selisih kas)
        'TRANSFER_IN',  -- Terima transfer saldo dari user lain
        'TRANSFER_OUT'  -- Kirim transfer saldo ke user lain
    )),
    amount NUMERIC(15, 2) NOT NULL CHECK (amount > 0),                           -- Nominal uang transaksi
    balance_before NUMERIC(15, 2) NOT NULL,                                      -- Saldo sebelum transaksi
    balance_after NUMERIC(15, 2) NOT NULL,                                       -- Saldo sesudah transaksi
    title TEXT NOT NULL,                                                         -- Nama transaksi jelas (misal: "Refund Pembelian Pulsa Gagal", "Koreksi Saldo oleh Admin")
    description TEXT NOT NULL,                                                   -- Keterangan rinci & alasan transaksi
    reference_id TEXT,                                                           -- Nomor referensi transaksi sumber / ID order PPOB
    admin_id UUID REFERENCES auth.users(id),                                     -- ID Admin jika transaksi berupa penyesuaian manual
    admin_notes TEXT,                                                            -- Catatan internal admin
    status TEXT NOT NULL DEFAULT 'SUCCESS' CHECK (status IN ('SUCCESS', 'PENDING', 'FAILED', 'REVERSED')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexing tabel transaksi
CREATE INDEX IF NOT EXISTS idx_app_wallet_transactions_user_id ON public.app_wallet_transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_app_wallet_transactions_code ON public.app_wallet_transactions(transaction_code);
CREATE INDEX IF NOT EXISTS idx_app_wallet_transactions_created_at ON public.app_wallet_transactions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_app_wallet_transactions_category ON public.app_wallet_transactions(category);

-- =====================================================================
-- 3. FUNCTION ATOMIK: PROSES MUTASI SALDO DENGAN ROW LOCK (ANTI RACE-CONDITION)
-- Menjamin saldo tidak pernah negatif dan transaksi selalu tercatat di buku besar
-- =====================================================================
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
    v_prefix TEXT;
    v_trx_id UUID;
BEGIN
    -- Validasi input nominal
    IF p_amount <= 0 THEN
        RAISE EXCEPTION 'Nominal transaksi harus lebih besar dari 0';
    END IF;

    -- Validasi tipe transaksi
    IF p_type NOT IN ('CREDIT', 'DEBIT') THEN
        RAISE EXCEPTION 'Tipe mutasi tidak valid. Harus CREDIT atau DEBIT';
    END IF;

    -- Kunci baris dompet dengan FOR UPDATE untuk mencegah race condition (ACID Safe)
    SELECT * INTO v_wallet 
    FROM public.app_wallets 
    WHERE user_id = p_user_id 
    FOR UPDATE;

    -- Jika user belum memiliki wallet saldo aplikasi, buatkan otomatis
    IF NOT FOUND THEN
        INSERT INTO public.app_wallets (user_id, balance, points)
        VALUES (p_user_id, 0, 0)
        RETURNING * INTO v_wallet;
    END IF;

    -- Periksa status lock wallet
    IF v_wallet.is_locked THEN
        RAISE EXCEPTION 'Dompet saldo aplikasi sedang dibekukan / dikunci. Hubungi CS DewaCell.';
    END IF;

    v_balance_before := v_wallet.balance;

    -- Hitung saldo akhir & validasi kecukupan saldo untuk transaksi DEBIT
    IF p_type = 'DEBIT' THEN
        IF v_balance_before < p_amount THEN
            RAISE EXCEPTION 'Saldo tidak mencukupi. Saldo saat ini: Rp %, dibutuhkan: Rp %', v_balance_before, p_amount;
        END IF;
        v_balance_after := v_balance_before - p_amount;
    ELSE
        -- CREDIT (Uang masuk / Refund / Penyesuaian tambah)
        v_balance_after := v_balance_before + p_amount;
    END IF;

    -- Tentukan prefix kode transaksi sesuai kategori
    v_prefix := CASE p_category
        WHEN 'TOPUP' THEN 'TOP'
        WHEN 'PAYMENT' THEN 'TRX'
        WHEN 'REFUND' THEN 'REF'
        WHEN 'ADJUSTMENT' THEN 'ADJ'
        WHEN 'TRANSFER_IN' THEN 'TFI'
        WHEN 'TRANSFER_OUT' THEN 'TFO'
        ELSE 'MUT'
    END;

    v_trx_code := v_prefix || '-' || TO_CHAR(NOW(), 'YYYYMMDD') || '-' || LPAD(FLOOR(RANDOM() * 900000 + 100000)::TEXT, 6, '0');

    -- Update saldo di tabel app_wallets
    UPDATE public.app_wallets
    SET balance = v_balance_after,
        updated_at = NOW()
    WHERE user_id = p_user_id;

    -- Catat riwayat lengkap ke tabel mutasi app_wallet_transactions
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

    -- Kembalikan respons JSON lengkap
    RETURN jsonb_build_object(
        'success', true,
        'transaction_id', v_trx_id,
        'transaction_code', v_trx_code,
        'type', p_type,
        'category', p_category,
        'amount', p_amount,
        'balance_before', v_balance_before,
        'balance_after', v_balance_after,
        'title', p_title,
        'description', p_description,
        'created_at', NOW()
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- =====================================================================
-- 4. FUNCTION REFUND TRANSAKSI (PENGEMBALIAN DANA SALDO)
-- Menghubungkan transaksi debit lama dan mengembalikan uang secara utuh ke saldo
-- =====================================================================
CREATE OR REPLACE FUNCTION public.refund_app_wallet_transaction(
    p_transaction_code TEXT,
    p_reason TEXT DEFAULT 'Transaksi gagal diproses oleh sistem / provider'
)
RETURNS JSONB AS $$
DECLARE
    v_orig RECORD;
    v_refund_result JSONB;
    v_title TEXT;
    v_desc TEXT;
BEGIN
    -- Cari transaksi asli yang hendak di-refund
    SELECT * INTO v_orig
    FROM public.app_wallet_transactions
    WHERE transaction_code = p_transaction_code;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Transaksi dengan kode % tidak ditemukan', p_transaction_code;
    END IF;

    IF v_orig.type != 'DEBIT' THEN
        RAISE EXCEPTION 'Hanya transaksi DEBIT (pembayaran/pengeluaran) yang dapat di-refund';
    END IF;

    -- Pastikan transaksi ini belum pernah di-refund sebelumnya
    IF EXISTS (
        SELECT 1 FROM public.app_wallet_transactions 
        WHERE reference_id = p_transaction_code AND category = 'REFUND'
    ) THEN
        RAISE EXCEPTION 'Transaksi % sudah pernah di-refund sebelumnya', p_transaction_code;
    END IF;

    v_title := 'Refund Dana: ' || v_orig.title;
    v_desc := 'Pengembalian saldo atas ' || v_orig.transaction_code || '. Alasan: ' || p_reason;

    -- Jalankan mutasi CREDIT pengembalian saldo
    v_refund_result := public.process_app_wallet_mutation(
        p_user_id      := v_orig.user_id,
        p_type         := 'CREDIT',
        p_category     := 'REFUND',
        p_amount       := v_orig.amount,
        p_title        := v_title,
        p_description  := v_desc,
        p_reference_id := v_orig.transaction_code
    );

    RETURN v_refund_result;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- =====================================================================
-- 5. FUNCTION SUPER ADMIN: PENYESUAIAN SALDO (KOREKSI KEKELIRUAN / SALAH INPUT)
-- Super Admin dapat menambah (CREDIT) atau memotong (DEBIT) saldo dengan pencatatan audit jelas
-- =====================================================================
CREATE OR REPLACE FUNCTION public.admin_adjust_app_wallet(
    p_target_user_id UUID,
    p_type TEXT,            -- 'CREDIT' (tambah saldo) atau 'DEBIT' (kurang saldo)
    p_amount NUMERIC,
    p_reason TEXT,          -- Wajib diisi alasan jelas kenapa disesuaikan
    p_admin_id UUID,        -- ID admin pelaksana
    p_admin_notes TEXT DEFAULT NULL
)
RETURNS JSONB AS $$
DECLARE
    v_title TEXT;
    v_desc TEXT;
    v_admin_email TEXT;
    v_result JSONB;
BEGIN
    -- Validasi keberadaan admin
    SELECT email INTO v_admin_email FROM auth.users WHERE id = p_admin_id;
    IF v_admin_email IS NULL THEN
        v_admin_email := 'Admin Sistem';
    END IF;

    IF p_reason IS NULL OR TRIM(p_reason) = '' THEN
        RAISE EXCEPTION 'Alasan penyesuaian saldo wajib diisi untuk kepatuhan audit pembukuan';
    END IF;

    -- Susun nama dan deskripsi transaksi yang sangat jelas & transparan
    IF p_type = 'CREDIT' THEN
        v_title := 'Penyesuaian Saldo oleh Super Admin (Kredit/Penambahan)';
        v_desc  := 'Penambahan saldo sebesar Rp ' || TO_CHAR(p_amount, 'FM999,999,999') || 
                   ' oleh Admin (' || v_admin_email || '). Alasan: ' || p_reason;
    ELSE
        v_title := 'Penyesuaian Saldo oleh Super Admin (Debet/Koreksi Salah Input)';
        v_desc  := 'Pemotongan saldo sebesar Rp ' || TO_CHAR(p_amount, 'FM999,999,999') || 
                   ' karena koreksi pembukuan oleh Admin (' || v_admin_email || '). Alasan: ' || p_reason;
    END IF;

    -- Jalankan proses mutasi
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

-- =====================================================================
-- 6. INISIALISASI / BACKFILL USER EKSIS KE TABEL APP_WALLETS
-- Pastikan user yang sudah terdaftar langsung memiliki akun dompet aplikasi
-- =====================================================================
INSERT INTO public.app_wallets (user_id, balance, points)
SELECT id, 500000, 10000 -- Berikan saldo awal Rp 500.000 untuk pengujian
FROM auth.users
ON CONFLICT (user_id) DO NOTHING;

-- Catat mutasi awal pembukaan dompet untuk user eksis
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
    status
)
SELECT 
    'TOP-' || TO_CHAR(NOW(), 'YYYYMMDD') || '-' || SUBSTRING(u.id::text, 1, 6),
    u.id,
    'CREDIT',
    'TOPUP',
    500000,
    0,
    500000,
    'Bonus Saldo Awal Akun DewaCell',
    'Saldo deposit pembukaan akun resmi DewaCell PPOB',
    'SUCCESS'
FROM auth.users u
WHERE NOT EXISTS (
    SELECT 1 FROM public.app_wallet_transactions wt WHERE wt.user_id = u.id
);
