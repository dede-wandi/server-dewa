-- ==============================================================================
-- DEWACELL DATABASE MIGRATION 13: DYNAMIC PAYMENT CHANNELS & MULTI-VENDOR GATEWAYS
-- File: 13_create_payment_channels.sql
-- Description: Pengelolaan channel pembayaran dinamis (Fund Credit, QRIS, VA Bank)
--              dengan dukungan multi-vendor (Manual Admin, Duitku, DANA, Midtrans, dll)
-- ==============================================================================

CREATE TABLE IF NOT EXISTS public.payment_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) NOT NULL UNIQUE,          -- contoh: 'fund_credit', 'qris_duitku', 'qris_dana', 'bca_va_duitku'
    name VARCHAR(100) NOT NULL,                -- contoh: 'Fund Credit (Transfer Manual Admin)', 'QRIS Duitku', 'BCA Virtual Account'
    channel_type VARCHAR(50) NOT NULL,         -- 'MANUAL', 'QRIS', 'VIRTUAL_ACCOUNT', 'EWALLET', 'RETAIL'
    vendor VARCHAR(50) NOT NULL,               -- 'manual', 'duitku', 'dana', 'midtrans', 'xendit', 'tripay'
    account_number VARCHAR(100) DEFAULT '',    -- contoh: '8801 2345 6789' untuk transfer manual
    account_name VARCHAR(100) DEFAULT '',      -- contoh: 'PT DEWA CELLULER INDONESIA'
    icon_name VARCHAR(50) DEFAULT 'wallet-outline',
    icon_url TEXT DEFAULT '',
    admin_fee NUMERIC(15, 2) NOT NULL DEFAULT 0,
    admin_fee_percent NUMERIC(5, 2) NOT NULL DEFAULT 0,
    min_amount NUMERIC(15, 2) NOT NULL DEFAULT 10000,
    max_amount NUMERIC(15, 2) NOT NULL DEFAULT 10000000,
    status VARCHAR(20) NOT NULL DEFAULT 'COMING_SOON', -- 'ACTIVE', 'COMING_SOON', 'INACTIVE'
    is_pg BOOLEAN NOT NULL DEFAULT false,      -- true jika Payment Gateway otomatis, false jika manual/fund_credit
    badge VARCHAR(50) DEFAULT 'Coming Soon',   -- 'AKTIF', 'Coming Soon', 'Promo'
    instructions TEXT DEFAULT '',              -- Petunjuk transfer / instruksi bayar
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payment_channels_status ON public.payment_channels(status);
CREATE INDEX IF NOT EXISTS idx_payment_channels_vendor ON public.payment_channels(vendor);
CREATE INDEX IF NOT EXISTS idx_payment_channels_type ON public.payment_channels(channel_type);
CREATE INDEX IF NOT EXISTS idx_payment_channels_sort ON public.payment_channels(sort_order ASC);

-- Enable RLS
ALTER TABLE public.payment_channels ENABLE ROW LEVEL SECURITY;

-- 1. Policy Public Read: Semua user (termasuk anon & authenticated) dapat melihat channel yang aktif & coming soon
DROP POLICY IF EXISTS "Public read payment channels" ON public.payment_channels;
CREATE POLICY "Public read payment channels" ON public.payment_channels
    FOR SELECT
    TO anon, authenticated
    USING (status IN ('ACTIVE', 'COMING_SOON'));

-- 2. Policy Service Role: Backend service_role memiliki akses penuh
DROP POLICY IF EXISTS "Service role full access payment channels" ON public.payment_channels;
CREATE POLICY "Service role full access payment channels" ON public.payment_channels
    FOR ALL
    TO service_role
    USING (true)
    WITH CHECK (true);

-- Seed Data Awal: Channel Fund Credit (Aktif) & Aneka Vendor PG (Coming Soon)
INSERT INTO public.payment_channels (code, name, channel_type, vendor, account_number, account_name, icon_name, admin_fee, min_amount, max_amount, status, is_pg, badge, instructions, sort_order)
VALUES
    ('fund_credit', 'Fund Credit (Transfer Manual Admin)', 'MANUAL', 'manual', '8801 2345 6789', 'PT DEWA CELLULER INDONESIA', 'wallet-outline', 0, 10000, 10000000, 'ACTIVE', false, 'AKTIF', 'Lakukan transfer sesuai nominal ke rekening resmi BCA PT DEWA CELL, lalu klik tombol konfirmasi. Admin akan langsung memproses saldo ke akun Anda.', 1),
    ('qris_duitku', 'QRIS Duitku', 'QRIS', 'duitku', '', '', 'qr-code-outline', 0, 10000, 5000000, 'COMING_SOON', true, 'Coming Soon', 'Scan QRIS Duitku otomatis menggunakan GoPay, OVO, Dana, LinkAja, ShopeePay atau Mobile Banking.', 2),
    ('qris_dana', 'QRIS DANA Bisnis', 'QRIS', 'dana', '', '', 'qr-code-outline', 0, 10000, 5000000, 'COMING_SOON', true, 'Coming Soon', 'Scan QRIS langsung terhubung dengan merchant DANA Bisnis.', 3),
    ('bca_va_duitku', 'BCA Virtual Account (Duitku)', 'VIRTUAL_ACCOUNT', 'duitku', '', '', 'card-outline', 0, 10000, 10000000, 'COMING_SOON', true, 'Coming Soon', 'Transfer melalui BCA Virtual Account otomatis tanpa perlu konfirmasi manual.', 4),
    ('bni_va_duitku', 'BNI Virtual Account (Duitku)', 'VIRTUAL_ACCOUNT', 'duitku', '', '', 'card-outline', 0, 10000, 10000000, 'COMING_SOON', true, 'Coming Soon', 'Transfer melalui BNI Virtual Account otomatis.', 5),
    ('mandiri_va_duitku', 'Mandiri Livin VA (Duitku)', 'VIRTUAL_ACCOUNT', 'duitku', '', '', 'card-outline', 0, 10000, 10000000, 'COMING_SOON', true, 'Coming Soon', 'Transfer melalui Mandiri Virtual Account otomatis.', 6),
    ('bri_va_duitku', 'BRI BRIVA (Duitku)', 'VIRTUAL_ACCOUNT', 'duitku', '', '', 'card-outline', 0, 10000, 10000000, 'COMING_SOON', true, 'Coming Soon', 'Transfer melalui BRI Virtual Account (BRIVA) otomatis.', 7)
ON CONFLICT (code) DO NOTHING;
