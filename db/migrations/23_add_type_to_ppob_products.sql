-- ==============================================================================
-- DEWACELL DATABASE MIGRATION 23: ADD TYPE TO PPOB PRODUCTS
-- File: 23_add_type_to_ppob_products.sql
-- Description: Menambahkan kolom 'type' ('PREPAID' vs 'POSTPAID') pada produk PPOB
--              agar mendukung alur Prabayar (langsung beli) & Pascabayar (cek tagihan).
-- ==============================================================================

-- 1. Tambahkan kolom 'type' dengan default 'PREPAID' (Prabayar)
ALTER TABLE public.ppob_products 
ADD COLUMN IF NOT EXISTS type VARCHAR(50) NOT NULL DEFAULT 'PREPAID';

-- 2. Indexing kolom type untuk kecepatan filtering
CREATE INDEX IF NOT EXISTS idx_ppob_products_type ON public.ppob_products(type);

-- 3. Update produk yang sudah ada agar dipastikan bertipe 'PREPAID'
UPDATE public.ppob_products 
SET type = 'PREPAID' 
WHERE type IS NULL OR type = '';
