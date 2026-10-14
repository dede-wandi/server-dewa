-- ==============================================================================
-- DEWACELL DATABASE MIGRATION 10: DYNAMIC BANNERS & MULTI-TYPE PROMOS
-- File: 10_create_banners_and_promos.sql
-- Description: Tabel banner promo multi-kondisi (PROMO, INFO, EDUKASI), multi-placement
--              (HOME_TOP, HOME_POPUP, WALLET_PAGE, SERVICE_PAGE), dan dukungan gambar poster
-- ==============================================================================

CREATE TABLE IF NOT EXISTS public.banners (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    subtitle TEXT,
    content TEXT,                                 -- Rincian lengkap T&C / isi pengumuman jika diklik
    type VARCHAR(50) DEFAULT 'PROMO',             -- 'PROMO', 'INFO', 'EDUKASI', 'MAINTENANCE'
    placement VARCHAR(50) DEFAULT 'HOME_TOP',     -- 'HOME_TOP', 'HOME_POPUP', 'WALLET_PAGE', 'SERVICE_PAGE'
    tag VARCHAR(100) DEFAULT 'PROMO SPESIAL',
    image_url TEXT,                               -- URL gambar banner (Full Poster jika diisi)
    cta_text VARCHAR(100) DEFAULT 'Lihat Promo',
    target_url VARCHAR(255) DEFAULT '/service/pulsa',
    action_type VARCHAR(50) DEFAULT 'navigate',   -- 'navigate', 'link', 'popup', 'none'
    bg_color VARCHAR(50) DEFAULT '#FFA000',
    accent_color VARCHAR(50) DEFAULT '#FF8F00',
    icon VARCHAR(100) DEFAULT 'phone-portrait',
    emoji VARCHAR(20) DEFAULT '📱',
    sort_order INT DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    start_date TIMESTAMPTZ,
    end_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Pastikan kolom baru terpasang jika tabel sudah ada sebelumnya
ALTER TABLE public.banners ADD COLUMN IF NOT EXISTS type VARCHAR(50) DEFAULT 'PROMO';
ALTER TABLE public.banners ADD COLUMN IF NOT EXISTS placement VARCHAR(50) DEFAULT 'HOME_TOP';
ALTER TABLE public.banners ADD COLUMN IF NOT EXISTS content TEXT;

-- Indexing untuk query cepat berdasarkan kondisi penempatan dan keaktifan
CREATE INDEX IF NOT EXISTS idx_banners_placement_active ON public.banners(is_active, placement, sort_order ASC, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_banners_type_active ON public.banners(is_active, type, sort_order ASC);

-- Enable RLS (Row Level Security)
ALTER TABLE public.banners ENABLE ROW LEVEL SECURITY;

-- 1. Policy Read Public: Pengguna aplikasi (anon & authenticated) dapat melihat banner yang aktif sesuai jadwal
DROP POLICY IF EXISTS "Public read active banners" ON public.banners;
CREATE POLICY "Public read active banners" ON public.banners
    FOR SELECT
    TO anon, authenticated
    USING (
        is_active = true AND 
        (start_date IS NULL OR start_date <= now()) AND 
        (end_date IS NULL OR end_date >= now())
    );

-- 2. Policy Service Role: Backend service_role memiliki akses penuh
DROP POLICY IF EXISTS "Service role full access banners" ON public.banners;
CREATE POLICY "Service role full access banners" ON public.banners
    FOR ALL
    TO service_role
    USING (true)
    WITH CHECK (true);

-- 3. Policy Admin Management: Admin & Superadmin memiliki akses CRUD
DROP POLICY IF EXISTS "Admin full control banners" ON public.banners;
CREATE POLICY "Admin full control banners" ON public.banners
    FOR ALL
    TO authenticated
    USING (
        EXISTS (
            SELECT 1 FROM public.user_roles ur
            JOIN public.roles r ON ur.role_id = r.id
            WHERE ur.user_id = auth.uid() AND r.name IN ('admin', 'superadmin')
        )
    )
    WITH CHECK (
        EXISTS (
            SELECT 1 FROM public.user_roles ur
            JOIN public.roles r ON ur.role_id = r.id
            WHERE ur.user_id = auth.uid() AND r.name IN ('admin', 'superadmin')
        )
    );

-- Hak akses table
GRANT ALL ON TABLE public.banners TO anon, authenticated, service_role;

-- ==============================================================================
-- INITIAL SEED DATA (Banner Berbagai Macam Tipe & Penempatan)
-- ==============================================================================
INSERT INTO public.banners (
    id, title, subtitle, content, type, placement, tag, cta_text, target_url, action_type,
    bg_color, accent_color, icon, emoji, sort_order, is_active
)
VALUES
-- 1. HOME CAROUSEL (PROMO)
(
    'a1111111-0000-0000-0000-000000000001',
    'Pulsa & Data Murah Banget!',
    'Hemat hingga 40% semua operator',
    'Nikmati potongan harga langsung untuk setiap pembelian pulsa reguler dan kuota data Telkomsel, Indosat, XL, Tri, dan Smartfren.',
    'PROMO',
    'HOME_TOP',
    'PROMO SPESIAL',
    'Beli Sekarang 🔥',
    '/service/pulsa',
    'navigate',
    '#FFA000',
    '#FF8F00',
    'phone-portrait',
    '📱',
    1,
    true
),
-- 2. HOME CAROUSEL (TOKEN PLN)
(
    'a1111111-0000-0000-0000-000000000002',
    'Token PLN Bonus 5%!',
    'Transaksi instan 24 jam, tanpa antre',
    'Beli token listrik PLN prabayar kapan saja tanpa gangguan antrean. Strum langsung masuk dalam hitungan detik.',
    'PROMO',
    'HOME_TOP',
    'TOKEN LISTRIK',
    'Beli Token ⚡',
    '/service/pln',
    'navigate',
    '#1E3A5F',
    '#2563EB',
    'flash',
    '⚡',
    2,
    true
),
-- 3. HOME CAROUSEL (GROCERIES)
(
    'a1111111-0000-0000-0000-000000000003',
    'Sayur & Buah Antar ke Pintu!',
    'Order sekarang, tiba dalam 2 jam',
    'Belanja kebutuhan dapur sayur dan buah segar pilihan langsung dari petani mitra DewaCell Mart.',
    'PROMO',
    'HOME_TOP',
    'GROCERIES SEGAR',
    'Order Sekarang 🛒',
    '/service/groceries',
    'navigate',
    '#065F46',
    '#10B981',
    'leaf',
    '🥦',
    3,
    true
),
-- 4. HOME POPUP PENGUMUMAN (POPUP SAAT BUKA APLIKASI)
(
    'a1111111-0000-0000-0000-000000000004',
    'Pemberitahuan Sistem DewaCell',
    'Dapatkan bonus poin reward untuk setiap transaksi PPOB hari ini!',
    'Halo Mitra DewaCell! Tingkatkan terus transaksi Anda. Setiap transaksi berhasil akan otomatis memberikan DewaPoin yang dapat ditukar dengan saldo dompet.',
    'INFO',
    'HOME_POPUP',
    'PENGUMUMAN RESMI',
    'Pelajari Selengkapnya ✨',
    '/wallet-history',
    'popup',
    '#0284C7',
    '#0369A1',
    'megaphone',
    '📢',
    1,
    true
),
-- 5. WALLET PAGE BANNER (PROMO SALDO)
(
    'a1111111-0000-0000-0000-000000000005',
    'Top Up Saldo Tanpa Biaya Admin!',
    'Gunakan metode QRIS atau Transfer Bank langsung',
    'Isi ulang saldo dompet DewaCell bebas biaya penanganan. Saldo langsung siap dipakai belanja dan jualan.',
    'PROMO',
    'WALLET_PAGE',
    'BEBAS BIAYA',
    'Top Up Sekarang 💳',
    '/service/topup',
    'navigate',
    '#059669',
    '#047857',
    'wallet',
    '💰',
    1,
    true
)
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    subtitle = EXCLUDED.subtitle,
    content = EXCLUDED.content,
    type = EXCLUDED.type,
    placement = EXCLUDED.placement,
    tag = EXCLUDED.tag,
    cta_text = EXCLUDED.cta_text,
    target_url = EXCLUDED.target_url,
    action_type = EXCLUDED.action_type,
    bg_color = EXCLUDED.bg_color,
    accent_color = EXCLUDED.accent_color,
    icon = EXCLUDED.icon,
    emoji = EXCLUDED.emoji,
    sort_order = EXCLUDED.sort_order,
    is_active = EXCLUDED.is_active;
