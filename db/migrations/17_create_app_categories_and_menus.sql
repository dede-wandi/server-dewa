-- ==============================================================================
-- DEWACELL DATABASE MIGRATION 17: DYNAMIC APP MENUS & MULTI-TIER CATEGORIES (TREE)
-- File: 17_create_app_categories_and_menus.sql
-- Description: Pengelolaan kategori layanan berhierarki (Kategori, Sub-Kategori,
--              Sub-Sub-Kategori) dan menu layanan dinamis untuk aplikasi pengguna.
--              Tabel dibuat KOSONGAN agar Admin bebas menginput dan menguji via Postman.
--              Nama tabel menggunakan prefix 'app_' sehingga aman dari tabel POS 'categories'.
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

-- Reload Schema Cache PostgREST
NOTIFY pgrst, 'reload schema';
NOTIFY pgrst, 'reload config';
