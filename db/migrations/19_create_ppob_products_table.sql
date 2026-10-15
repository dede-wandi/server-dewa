-- ==============================================================================
-- DEWACELL DATABASE MIGRATION 19: DEDICATED PPOB & DIGITAL PRODUCTS TABLE
-- File: 19_create_ppob_products_table.sql
-- Description: Membuat tabel terpisah 'public.ppob_products' khusus produk digital
--              (Pulsa, Paket Data, PLN, E-Money, Game) agar TIDAK BENTROK dengan
--              tabel 'public.products' yang digunakan oleh sistem POSDEWA (Kasir/Fisik).
-- ==============================================================================

-- 1. Buat Tabel Produk Digital PPOB
CREATE TABLE IF NOT EXISTS public.ppob_products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID NOT NULL REFERENCES public.app_categories(id) ON DELETE RESTRICT,
    vendor_code VARCHAR(50) NOT NULL DEFAULT 'MANUAL',
    buyer_sku_code VARCHAR(100) NOT NULL,
    code VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(100) NOT NULL,              -- 'pulsa', 'data', 'pln', 'emoney', 'game', dll
    brand VARCHAR(100) DEFAULT '',
    cost_price NUMERIC(15, 2) NOT NULL DEFAULT 0,
    price NUMERIC(15, 2) NOT NULL DEFAULT 0,
    points_reward BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    stock INT NOT NULL DEFAULT 9999,
    description TEXT,
    image_url TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 2. Indexing untuk Kecepatan Query Katalog & Validasi SKU Vendor
CREATE INDEX IF NOT EXISTS idx_ppob_products_category_id ON public.ppob_products(category_id);
CREATE INDEX IF NOT EXISTS idx_ppob_products_code ON public.ppob_products(code);
CREATE INDEX IF NOT EXISTS idx_ppob_products_vendor_code ON public.ppob_products(vendor_code);
CREATE INDEX IF NOT EXISTS idx_ppob_products_buyer_sku_code ON public.ppob_products(buyer_sku_code);
CREATE INDEX IF NOT EXISTS idx_ppob_products_category ON public.ppob_products(category);
CREATE INDEX IF NOT EXISTS idx_ppob_products_status ON public.ppob_products(status);
CREATE INDEX IF NOT EXISTS idx_ppob_products_is_active ON public.ppob_products(is_active);

-- 3. Row Level Security (RLS)
ALTER TABLE public.ppob_products ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS "ppob_products_full_policy" ON public.ppob_products;

CREATE POLICY "ppob_products_full_policy" ON public.ppob_products 
FOR ALL TO anon, authenticated, service_role 
USING (true) 
WITH CHECK (true);
