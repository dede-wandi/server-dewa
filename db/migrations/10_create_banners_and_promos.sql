-- ==============================================================================
-- DEWACELL DATABASE MIGRATION 10: DYNAMIC HOME BANNERS & PROMO SLIDERS
-- File: 10_create_banners_and_promos.sql
-- Description: Tabel dan perizinan untuk banner promo dinamis di Home App & Admin CRUD
-- ==============================================================================

CREATE TABLE IF NOT EXISTS public.banners (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    subtitle TEXT,
    tag VARCHAR(100) DEFAULT 'PROMO SPESIAL',
    image_url TEXT,
    cta_text VARCHAR(100) DEFAULT 'Beli Sekarang 🔥',
    target_url VARCHAR(255) DEFAULT '/service/pulsa',
    action_type VARCHAR(50) DEFAULT 'navigate', -- 'navigate', 'link', 'none'
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

-- Indexing untuk query cepat
CREATE INDEX IF NOT EXISTS idx_banners_active_sort ON public.banners(is_active, sort_order, created_at DESC);

-- Enable RLS (Row Level Security)
ALTER TABLE public.banners ENABLE ROW LEVEL SECURITY;

-- 1. Policy Read Public: Pengguna aplikasi (anon & authenticated) dapat melihat banner yang aktif
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

-- Grant privileges
GRANT ALL ON TABLE public.banners TO anon, authenticated, service_role;

-- ==============================================================================
-- INITIAL SEED DATA (Banner Bawaan Aplikasi DewaCell)
-- ==============================================================================
INSERT INTO public.banners (
    title, subtitle, tag, cta_text, target_url, action_type,
    bg_color, accent_color, icon, emoji, sort_order, is_active
)
VALUES
(
    'Pulsa & Data Murah Banget!',
    'Hemat hingga 40% semua operator',
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
(
    'Token PLN Bonus 5%!',
    'Transaksi instan 24 jam, tanpa antre',
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
(
    'Sayur & Buah Antar ke Pintu!',
    'Order sekarang, tiba dalam 2 jam',
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
(
    'Bayar Semua Tagihan di Sini!',
    'PDAM, BPJS, Internet, TV Kabel & lebih',
    'TAGIHAN PPOB',
    'Bayar Tagihan 💳',
    '/service/bills',
    'navigate',
    '#4C1D95',
    '#7C3AED',
    'receipt',
    '🧾',
    4,
    true
)
ON CONFLICT DO NOTHING;
