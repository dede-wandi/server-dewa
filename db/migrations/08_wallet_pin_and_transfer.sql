-- =====================================================================
-- DEWACELL WALLET PIN SECURITY & PEER-TO-PEER BALANCE TRANSFER
-- Migration: 08_wallet_pin_and_transfer.sql
-- =====================================================================

-- 1. Tambah kolom PIN hash dan status PIN pada tabel app_wallets
ALTER TABLE public.app_wallets
ADD COLUMN IF NOT EXISTS pin_hash TEXT NULL,
ADD COLUMN IF NOT EXISTS is_pin_set BOOLEAN NOT NULL DEFAULT false,
ADD COLUMN IF NOT EXISTS pin_updated_at TIMESTAMP WITH TIME ZONE NULL;

-- 2. Function Setup PIN Baru (Hanya jika belum dibuat)
CREATE OR REPLACE FUNCTION public.setup_wallet_pin(
    p_user_id UUID,
    p_pin_hash TEXT
)
RETURNS JSONB AS $$
DECLARE
    v_wallet RECORD;
BEGIN
    SELECT * INTO v_wallet FROM public.app_wallets WHERE user_id = p_user_id FOR UPDATE;
    
    IF NOT FOUND THEN
        INSERT INTO public.app_wallets (user_id, balance, points, is_locked, is_pin_set, pin_hash, pin_updated_at)
        VALUES (p_user_id, 0, 0, false, true, p_pin_hash, NOW())
        RETURNING * INTO v_wallet;
    ELSE
        IF v_wallet.is_pin_set AND v_wallet.pin_hash IS NOT NULL AND v_wallet.pin_hash != '' THEN
            RAISE EXCEPTION 'PIN transaksi sudah pernah dibuat. Gunakan fitur ubah PIN jika ingin mengganti.';
        END IF;

        UPDATE public.app_wallets
        SET pin_hash = p_pin_hash,
            is_pin_set = true,
            pin_updated_at = NOW(),
            updated_at = NOW()
        WHERE user_id = p_user_id
        RETURNING * INTO v_wallet;
    END IF;

    RETURN jsonb_build_object(
        'success', true,
        'user_id', p_user_id,
        'is_pin_set', true,
        'message', 'PIN transaksi 6 digit berhasil dibuat'
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 3. Function Ubah PIN
CREATE OR REPLACE FUNCTION public.change_wallet_pin(
    p_user_id UUID,
    p_new_pin_hash TEXT
)
RETURNS JSONB AS $$
DECLARE
    v_wallet RECORD;
BEGIN
    SELECT * INTO v_wallet FROM public.app_wallets WHERE user_id = p_user_id FOR UPDATE;
    
    IF NOT FOUND OR NOT v_wallet.is_pin_set OR v_wallet.pin_hash IS NULL THEN
        RAISE EXCEPTION 'PIN transaksi belum dibuat. Silakan buat PIN terlebih dahulu.';
    END IF;

    UPDATE public.app_wallets
    SET pin_hash = p_new_pin_hash,
        is_pin_set = true,
        pin_updated_at = NOW(),
        updated_at = NOW()
    WHERE user_id = p_user_id
    RETURNING * INTO v_wallet;

    RETURN jsonb_build_object(
        'success', true,
        'user_id', p_user_id,
        'is_pin_set', true,
        'message', 'PIN transaksi berhasil diperbarui'
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 4. Function Transfer Saldo Antar Pengguna Real (Atomik & ACID Safe)
CREATE OR REPLACE FUNCTION public.transfer_app_wallet_balance(
    p_sender_id UUID,
    p_receiver_id UUID,
    p_amount NUMERIC,
    p_notes TEXT DEFAULT 'Transfer Saldo DewaPay'
)
RETURNS JSONB AS $$
DECLARE
    v_sender_wallet RECORD;
    v_receiver_wallet RECORD;
    v_sender_before NUMERIC;
    v_sender_after NUMERIC;
    v_receiver_before NUMERIC;
    v_receiver_after NUMERIC;
    v_trx_code_out TEXT;
    v_trx_code_in TEXT;
    v_sender_trx_id UUID;
    v_receiver_trx_id UUID;
    v_sender_email TEXT;
    v_receiver_email TEXT;
BEGIN
    IF p_amount <= 0 THEN
        RAISE EXCEPTION 'Nominal transfer harus lebih besar dari 0';
    END IF;

    IF p_sender_id = p_receiver_id THEN
        RAISE EXCEPTION 'Tidak dapat mentransfer saldo ke akun sendiri';
    END IF;

    -- Ambil email pengirim dan penerima
    SELECT email INTO v_sender_email FROM auth.users WHERE id = p_sender_id;
    SELECT email INTO v_receiver_email FROM auth.users WHERE id = p_receiver_id;

    IF v_receiver_email IS NULL THEN
        RAISE EXCEPTION 'Pengguna penerima transfer tidak ditemukan';
    END IF;

    -- Kunci dompet pengirim
    SELECT * INTO v_sender_wallet FROM public.app_wallets WHERE user_id = p_sender_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Dompet pengirim tidak ditemukan atau saldo kosong';
    END IF;

    IF v_sender_wallet.is_locked THEN
        RAISE EXCEPTION 'Dompet Anda sedang dikunci / dibekukan. Tidak dapat melakukan transfer.';
    END IF;

    v_sender_before := v_sender_wallet.balance;
    IF v_sender_before < p_amount THEN
        RAISE EXCEPTION 'Saldo tidak mencukupi untuk transfer. Saldo Anda: Rp %, Dibutuhkan: Rp %', v_sender_before, p_amount;
    END IF;

    -- Kunci dompet penerima (atau buatkan jika belum ada)
    SELECT * INTO v_receiver_wallet FROM public.app_wallets WHERE user_id = p_receiver_id FOR UPDATE;
    IF NOT FOUND THEN
        INSERT INTO public.app_wallets (user_id, balance, points)
        VALUES (p_receiver_id, 0, 0)
        RETURNING * INTO v_receiver_wallet;
    END IF;

    IF v_receiver_wallet.is_locked THEN
        RAISE EXCEPTION 'Dompet penerima sedang dibekukan / dikunci.';
    END IF;

    v_receiver_before := v_receiver_wallet.balance;
    v_sender_after := v_sender_before - p_amount;
    v_receiver_after := v_receiver_before + p_amount;

    -- Generate kode transaksi unik
    v_trx_code_out := 'TFO-' || TO_CHAR(NOW(), 'YYYYMMDD') || '-' || LPAD(FLOOR(RANDOM() * 900000 + 100000)::TEXT, 6, '0');
    v_trx_code_in := 'TFI-' || TO_CHAR(NOW(), 'YYYYMMDD') || '-' || LPAD(FLOOR(RANDOM() * 900000 + 100000)::TEXT, 6, '0');

    -- Update saldo pengirim
    UPDATE public.app_wallets
    SET balance = v_sender_after, updated_at = NOW()
    WHERE user_id = p_sender_id;

    -- Catat mutasi DEBIT pengirim
    INSERT INTO public.app_wallet_transactions (
        transaction_code, user_id, type, category, amount, balance_before, balance_after,
        title, description, reference_id, status, created_at
    ) VALUES (
        v_trx_code_out, p_sender_id, 'DEBIT', 'TRANSFER_OUT', p_amount, v_sender_before, v_sender_after,
        'Transfer Saldo ke ' || v_receiver_email, COALESCE(p_notes, 'Transfer Saldo DewaPay'), v_trx_code_in, 'SUCCESS', NOW()
    ) RETURNING id INTO v_sender_trx_id;

    -- Update saldo penerima
    UPDATE public.app_wallets
    SET balance = v_receiver_after, updated_at = NOW()
    WHERE user_id = p_receiver_id;

    -- Catat mutasi CREDIT penerima
    INSERT INTO public.app_wallet_transactions (
        transaction_code, user_id, type, category, amount, balance_before, balance_after,
        title, description, reference_id, status, created_at
    ) VALUES (
        v_trx_code_in, p_receiver_id, 'CREDIT', 'TRANSFER_IN', p_amount, v_receiver_before, v_receiver_after,
        'Terima Saldo dari ' || v_sender_email, COALESCE(p_notes, 'Transfer Saldo DewaPay'), v_trx_code_out, 'SUCCESS', NOW()
    ) RETURNING id INTO v_receiver_trx_id;

    RETURN jsonb_build_object(
        'success', true,
        'sender_transaction_code', v_trx_code_out,
        'receiver_transaction_code', v_trx_code_in,
        'amount', p_amount,
        'sender_balance_after', v_sender_after,
        'receiver_email', v_receiver_email,
        'notes', p_notes
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
