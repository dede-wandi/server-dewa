-- ==============================================================================
-- DEWACELL DATABASE MIGRATION 17: DYNAMIC APP MENUS & MULTI-TIER CATEGORIES (TREE)
-- File: 17_create_app_categories_and_menus.sql
-- Description: Pengelolaan kategori layanan berhierarki (Kategori, Sub-Kategori,
--              Sub-Sub-Kategori) dan menu layanan dinamis untuk aplikasi pengguna.
--              Nama tabel menggunakan prefix 'app_' agar tabel POSDEWA 'categories'
--              tetap aman 100% tanpa bentrok.
-- ==============================================================================

-- 1. Tabel Kategori Layanan Bertingkat / Bercabang (Hierarchical Tree)
CREATE TABLE IF NOT EXISTS public.app_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id UUID NULL REFERENCES public.app_categories(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    code VARCHAR(50) NOT NULL UNIQUE,
    slug VARCHAR(120) NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    icon VARCHAR(100) DEFAULT '',
    icon_url TEXT DEFAULT '',
    banner_url TEXT DEFAULT '',
    badge VARCHAR(50) DEFAULT '',
    level INT NOT NULL DEFAULT 1,          -- 1: Kategori Utama, 2: Sub-Kategori, 3: Sub-Sub Kategori, dst
    sort_order INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    meta JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_app_categories_parent ON public.app_categories(parent_id);
CREATE INDEX IF NOT EXISTS idx_app_categories_level ON public.app_categories(level);
CREATE INDEX IF NOT EXISTS idx_app_categories_active ON public.app_categories(is_active);
CREATE INDEX IF NOT EXISTS idx_app_categories_sort ON public.app_categories(sort_order ASC);
CREATE INDEX IF NOT EXISTS idx_app_categories_slug ON public.app_categories(slug);

-- Enable RLS
ALTER TABLE public.app_categories ENABLE ROW LEVEL SECURITY;

-- Policy Public Read untuk app_categories
DROP POLICY IF EXISTS "Public read app categories" ON public.app_categories;
CREATE POLICY "Public read app categories" ON public.app_categories
    FOR SELECT
    TO anon, authenticated
    USING (is_active = true);

-- Policy Full Access untuk service_role
DROP POLICY IF EXISTS "Service role full access app categories" ON public.app_categories;
CREATE POLICY "Service role full access app categories" ON public.app_categories
    FOR ALL
    TO service_role
    USING (true)
    WITH CHECK (true);

-- Policy Admin Full Access
DROP POLICY IF EXISTS "Admin write app categories" ON public.app_categories;
CREATE POLICY "Admin write app categories" ON public.app_categories
    FOR ALL
    TO authenticated
    USING (true)
    WITH CHECK (true);


-- 2. Tabel Menu Layanan Pengguna (App Service Menus)
CREATE TABLE IF NOT EXISTS public.app_menus (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    subtitle VARCHAR(150) DEFAULT '',
    category_id UUID NULL REFERENCES public.app_categories(id) ON DELETE SET NULL,
    action_type VARCHAR(50) NOT NULL DEFAULT 'CATEGORY', -- 'CATEGORY', 'ROUTE', 'WEBVIEW', 'DIALOG'
    target_url TEXT DEFAULT '',
    icon_type VARCHAR(30) DEFAULT 'IONICONS',            -- 'IONICONS', 'URL', 'SVG', 'EMOJI'
    icon_name VARCHAR(100) DEFAULT 'grid-outline',
    icon_url TEXT DEFAULT '',
    bg_color VARCHAR(30) DEFAULT '#EFF6FF',
    icon_color VARCHAR(30) DEFAULT '#2563EB',
    badge VARCHAR(50) DEFAULT '',                       -- 'PROMO', 'NEW', 'HOT', 'CASHBACK', 'HEMAT'
    badge_color VARCHAR(30) DEFAULT '#EF4444',
    placement VARCHAR(50) DEFAULT 'HOME_GRID',          -- 'HOME_GRID', 'QUICK_ACTION', 'DRAWER', 'FOOTER'
    sort_order INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_coming_soon BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_app_menus_category ON public.app_menus(category_id);
CREATE INDEX IF NOT EXISTS idx_app_menus_placement ON public.app_menus(placement);
CREATE INDEX IF NOT EXISTS idx_app_menus_active ON public.app_menus(is_active);
CREATE INDEX IF NOT EXISTS idx_app_menus_sort ON public.app_menus(sort_order ASC);

-- Enable RLS
ALTER TABLE public.app_menus ENABLE ROW LEVEL SECURITY;

-- Policy Public Read untuk app_menus
DROP POLICY IF EXISTS "Public read app menus" ON public.app_menus;
CREATE POLICY "Public read app menus" ON public.app_menus
    FOR SELECT
    TO anon, authenticated
    USING (is_active = true);

-- Policy Full Access untuk service_role
DROP POLICY IF EXISTS "Service role full access app menus" ON public.app_menus;
CREATE POLICY "Service role full access app menus" ON public.app_menus
    FOR ALL
    TO service_role
    USING (true)
    WITH CHECK (true);

-- Policy Admin Full Access
DROP POLICY IF EXISTS "Admin write app menus" ON public.app_menus;
CREATE POLICY "Admin write app menus" ON public.app_menus
    FOR ALL
    TO authenticated
    USING (true)
    WITH CHECK (true);


-- ==============================================================================
-- 3. SEED INITIAL DATA: HIERARCHICAL CATEGORIES (BERCABANG) & SERVICE MENUS
-- ==============================================================================

-- 3.1. Kategori Utama (Level 1 - Root)
INSERT INTO public.app_categories (id, parent_id, name, code, slug, description, icon, badge, level, sort_order, is_active)
VALUES
    ('c1000000-0000-0000-0000-000000000001', NULL, 'Pulsa & Paket Data', 'cat_pulsa_data', 'pulsa-dan-paket-data', 'Layanan pulsa reguler, transfer pulsa, paket kuota internet, telepon & SMS semua operator', 'phone-portrait-outline', 'PROMO', 1, 1, true),
    ('c1000000-0000-0000-0000-000000000002', NULL, 'Listrik & PLN', 'cat_pln', 'listrik-pln', 'Token listrik prabayar, tagihan pascabayar, dan pembayaran non-taglis PLN', 'flash-outline', 'HEMAT', 1, 2, true),
    ('c1000000-0000-0000-0000-000000000003', NULL, 'Uang Elektronik', 'cat_emoney', 'uang-elektronik', 'Top up saldo e-wallet DANA, GoPay, OVO, ShopeePay, LinkAja', 'wallet-outline', 'INSTAN', 1, 3, true),
    ('c1000000-0000-0000-0000-000000000004', NULL, 'Voucher Game', 'cat_games', 'voucher-game', 'Top up diamond & kredit game Mobile Legends, Free Fire, PUBG, Genshin Impact, Valorant', 'game-controller-outline', 'HOT', 1, 4, true),
    ('c1000000-0000-0000-0000-000000000005', NULL, 'Tagihan PPOB & Air', 'cat_tagihan', 'tagihan-ppob', 'Pembayaran tagihan air PDAM, BPJS Kesehatan, Telkom Indihome, Multifinance', 'receipt-outline', '', 1, 5, true),
    ('c1000000-0000-0000-0000-000000000006', NULL, 'Voucher & Streaming', 'cat_streaming', 'voucher-dan-streaming', 'Langganan streaming Netflix, Spotify, Vidio, Viu, YouTube Premium', 'tv-outline', 'NEW', 1, 6, true)
ON CONFLICT (code) DO NOTHING;

-- 3.2. Sub-Kategori (Level 2 - Induk di Level 1)
INSERT INTO public.app_categories (id, parent_id, name, code, slug, description, icon, badge, level, sort_order, is_active)
VALUES
    -- Di bawah 'Pulsa & Paket Data'
    ('c2000000-0000-0000-0000-000000000001', 'c1000000-0000-0000-0000-000000000001', 'Telkomsel', 'sub_tsel', 'telkomsel', 'Produk pulsa, paket data, dan masa aktif Telkomsel By.U', 'phone-portrait', 'TERLARIS', 2, 1, true),
    ('c2000000-0000-0000-0000-000000000002', 'c1000000-0000-0000-0000-000000000001', 'Indosat Ooredoo', 'sub_isat', 'indosat-ooredoo', 'Produk pulsa IM3, data Freedom, dan nelpon Indosat', 'phone-portrait', '', 2, 2, true),
    ('c2000000-0000-0000-0000-000000000003', 'c1000000-0000-0000-0000-000000000001', 'XL Axiata', 'sub_xl', 'xl-axiata', 'Produk pulsa XL, paket Akrab, dan Xtra Combo', 'phone-portrait', '', 2, 3, true),
    ('c2000000-0000-0000-0000-000000000004', 'c1000000-0000-0000-0000-000000000001', 'Tri (3)', 'sub_tri', 'tri-indonesia', 'Produk pulsa Tri, kuota AlwaysOn, dan Happy', 'phone-portrait', '', 2, 4, true),
    ('c2000000-0000-0000-0000-000000000005', 'c1000000-0000-0000-0000-000000000001', 'Smartfren', 'sub_smart', 'smartfren', 'Produk pulsa Smartfren dan paket Unlimited Nonstop', 'phone-portrait', '', 2, 5, true),

    -- Di bawah 'Listrik & PLN'
    ('c2000000-0000-0000-0000-000000000010', 'c1000000-0000-0000-0000-000000000002', 'Token Listrik Prabayar', 'sub_pln_token', 'token-listrik-prabayar', 'Beli 20 digit nomor stroom token PLN instan 24 jam', 'flash', 'INSTAN', 2, 1, true),
    ('c2000000-0000-0000-0000-000000000011', 'c1000000-0000-0000-0000-000000000002', 'Tagihan Listrik Pascabayar', 'sub_pln_pasca', 'tagihan-listrik-pascabayar', 'Cek dan bayar tagihan pemakaian listrik bulanan', 'receipt', '', 2, 2, true),

    -- Di bawah 'Uang Elektronik'
    ('c2000000-0000-0000-0000-000000000020', 'c1000000-0000-0000-0000-000000000003', 'DANA', 'sub_dana', 'dana', 'Top up saldo akun DANA instan', 'wallet', 'HEMAT', 2, 1, true),
    ('c2000000-0000-0000-0000-000000000021', 'c1000000-0000-0000-0000-000000000003', 'GoPay', 'sub_gopay', 'gopay', 'Top up saldo GoPay Customer & Driver', 'wallet', '', 2, 2, true),
    ('c2000000-0000-0000-0000-000000000022', 'c1000000-0000-0000-0000-000000000003', 'OVO', 'sub_ovo', 'ovo', 'Top up saldo OVO Cash', 'wallet', '', 2, 3, true),
    ('c2000000-0000-0000-0000-000000000023', 'c1000000-0000-0000-0000-000000000003', 'ShopeePay', 'sub_shopeepay', 'shopeepay', 'Top up saldo ShopeePay', 'wallet', '', 2, 4, true),

    -- Di bawah 'Voucher Game'
    ('c2000000-0000-0000-0000-000000000030', 'c1000000-0000-0000-0000-000000000004', 'Mobile Legends', 'sub_mlbb', 'mobile-legends', 'Top up Diamond Mobile Legends Bang Bang', 'game-controller', 'HOT', 2, 1, true),
    ('c2000000-0000-0000-0000-000000000031', 'c1000000-0000-0000-0000-000000000004', 'Free Fire', 'sub_ff', 'free-fire', 'Top up Diamond Free Fire Garena', 'game-controller', '', 2, 2, true)
ON CONFLICT (code) DO NOTHING;

-- 3.3. Sub-Sub Kategori (Level 3 - Bercabang di Bawah Sub-Kategori Telkomsel)
INSERT INTO public.app_categories (id, parent_id, name, code, slug, description, icon, badge, level, sort_order, is_active)
VALUES
    ('c3000000-0000-0000-0000-000000000001', 'c2000000-0000-0000-0000-000000000001', 'Pulsa Reguler Telkomsel', 'sub3_tsel_pulsa', 'pulsa-reguler-telkomsel', 'Pulsa reguler menambah masa aktif', 'phone-portrait', '', 3, 1, true),
    ('c3000000-0000-0000-0000-000000000002', 'c2000000-0000-0000-0000-000000000001', 'Kuota Internet Telkomsel', 'sub3_tsel_data', 'kuota-internet-telkomsel', 'Paket internet MAXstream, OMG, Combo Sakti', 'globe-outline', 'PROMO', 3, 2, true),
    ('c3000000-0000-0000-0000-000000000003', 'c2000000-0000-0000-0000-000000000001', 'Paket Nelpon & SMS', 'sub3_tsel_telp', 'paket-nelpon-sms-telkomsel', 'Paket bicara hemat ke sesama dan operator lain', 'call-outline', '', 3, 3, true),
    ('c3000000-0000-0000-0000-000000000004', 'c2000000-0000-0000-0000-000000000001', 'Perpanjang Masa Aktif', 'sub3_tsel_masa_aktif', 'perpanjang-masa-aktif-telkomsel', 'Beli masa aktif tanpa perlu isi pulsa besar', 'calendar-outline', '', 3, 4, true)
ON CONFLICT (code) DO NOTHING;

-- 3.4. Menu Layanan Aplikasi Pengguna (Home Screen Grid)
INSERT INTO public.app_menus (id, code, name, subtitle, category_id, action_type, target_url, icon_type, icon_name, bg_color, icon_color, badge, badge_color, placement, sort_order, is_active)
VALUES
    ('m1000000-0000-0000-0000-000000000001', 'menu_pulsa', 'Pulsa', 'Isi ulang pulsa murah', 'c1000000-0000-0000-0000-000000000001', 'CATEGORY', '/service/pulsa', 'IONICONS', 'phone-portrait-outline', '#EFF6FF', '#2563EB', '', '#EF4444', 'HOME_GRID', 1, true),
    ('m1000000-0000-0000-0000-000000000002', 'menu_paket_data', 'Paket Data', 'Kuota internet cepat', 'c1000000-0000-0000-0000-000000000001', 'CATEGORY', '/service/data', 'IONICONS', 'globe-outline', '#F0FDF4', '#16A34A', 'PROMO', '#EF4444', 'HOME_GRID', 2, true),
    ('m1000000-0000-0000-0000-000000000003', 'menu_pln_token', 'Listrik PLN', 'Token & tagihan listrik', 'c1000000-0000-0000-0000-000000000002', 'CATEGORY', '/service/pln', 'IONICONS', 'flash-outline', '#FEF3C7', '#D97706', '', '#EF4444', 'HOME_GRID', 3, true),
    ('m1000000-0000-0000-0000-000000000004', 'menu_emoney', 'E-Money', 'DANA, GoPay, OVO, Shopee', 'c1000000-0000-0000-0000-000000000003', 'CATEGORY', '/service/emoney', 'IONICONS', 'wallet-outline', '#F3E8FF', '#9333EA', 'INSTAN', '#16A34A', 'HOME_GRID', 4, true),
    ('m1000000-0000-0000-0000-000000000005', 'menu_game_voucher', 'Voucher Game', 'Diamond ML, FF, PUBG', 'c1000000-0000-0000-0000-000000000004', 'CATEGORY', '/service/games', 'IONICONS', 'game-controller-outline', '#FFE4E6', '#E11D48', 'HOT', '#E11D48', 'HOME_GRID', 5, true),
    ('m1000000-0000-0000-0000-000000000006', 'menu_bpjs', 'BPJS Kesehatan', 'Bayar premi BPJS bulanan', 'c1000000-0000-0000-0000-000000000005', 'CATEGORY', '/service/bpjs', 'IONICONS', 'shield-checkmark-outline', '#ECFDF5', '#059669', '', '#EF4444', 'HOME_GRID', 6, true),
    ('m1000000-0000-0000-0000-000000000007', 'menu_pdam', 'Air PDAM', 'Tagihan PDAM seluruh daerah', 'c1000000-0000-0000-0000-000000000005', 'CATEGORY', '/service/pdam', 'IONICONS', 'water-outline', '#E0F2FE', '#0284C7', '', '#EF4444', 'HOME_GRID', 7, true),
    ('m1000000-0000-0000-0000-000000000008', 'menu_semua', 'Lainnya', 'Semua produk & tagihan', NULL, 'ROUTE', '/service/more', 'IONICONS', 'grid-outline', '#F1F5F9', '#475569', '', '#EF4444', 'HOME_GRID', 8, true)
ON CONFLICT (code) DO NOTHING;

-- Reload Schema Cache
NOTIFY pgrst, 'reload schema';
NOTIFY pgrst, 'reload config';
