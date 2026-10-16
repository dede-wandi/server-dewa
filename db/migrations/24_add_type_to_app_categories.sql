-- ==============================================================================
-- DEWACELL DATABASE MIGRATION 24: ADD TYPE TO APP CATEGORIES
-- File: 24_add_type_to_app_categories.sql
-- Description: Menambahkan kolom 'type' ('pulsa', 'data', 'voucher', 'tlp', 'sms', dll)
--              pada tabel app_categories untuk pengelompokan jenis layanan di aplikasi mobile.
-- ==============================================================================

-- 1. Tambahkan kolom 'type' dengan default 'pulsa'
ALTER TABLE public.app_categories 
ADD COLUMN IF NOT EXISTS type VARCHAR(50) NOT NULL DEFAULT 'pulsa';

-- 2. Tambahkan indexing untuk kolom type
CREATE INDEX IF NOT EXISTS idx_app_categories_type ON public.app_categories(type);

-- 3. Update kategori default berdasarkan code/slug jika relevan
UPDATE public.app_categories 
SET type = 'data' 
WHERE slug ILIKE '%data%' OR code ILIKE '%data%' OR name ILIKE '%data%' OR name ILIKE '%internet%';

UPDATE public.app_categories 
SET type = 'voucher' 
WHERE slug ILIKE '%voucher%' OR code ILIKE '%voucher%' OR name ILIKE '%voucher%';

UPDATE public.app_categories 
SET type = 'tlp' 
WHERE slug ILIKE '%tlp%' OR slug ILIKE '%telepon%' OR code ILIKE '%tlp%' OR name ILIKE '%telepon%';

UPDATE public.app_categories 
SET type = 'pln' 
WHERE slug ILIKE '%pln%' OR slug ILIKE '%listrik%' OR code ILIKE '%pln%';

UPDATE public.app_categories 
SET type = 'emoney' 
WHERE slug ILIKE '%emoney%' OR slug ILIKE '%wallet%' OR slug ILIKE '%dana%' OR slug ILIKE '%ovo%';

UPDATE public.app_categories 
SET type = 'game' 
WHERE slug ILIKE '%game%' OR code ILIKE '%game%';

UPDATE public.app_categories 
SET type = 'pdam' 
WHERE slug ILIKE '%pdam%' OR slug ILIKE '%air%' OR code ILIKE '%pdam%';
