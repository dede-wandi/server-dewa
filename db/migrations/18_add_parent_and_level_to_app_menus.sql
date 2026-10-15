-- ==============================================================================
-- DEWACELL DATABASE MIGRATION 18: ADD PARENT & LEVEL TO APP MENUS (SUB-MENUS)
-- File: 18_add_parent_and_level_to_app_menus.sql
-- Description: Menambahkan dukungan sub-menu bertingkat / bercabang (Tree Mode)
--              pada tabel app_menus.
-- ==============================================================================

-- 1. Tambah kolom parent_id dan level pada app_menus
ALTER TABLE public.app_menus ADD COLUMN IF NOT EXISTS parent_id UUID NULL REFERENCES public.app_menus(id) ON DELETE CASCADE;
ALTER TABLE public.app_menus ADD COLUMN IF NOT EXISTS level INT NOT NULL DEFAULT 1;

-- 2. Index untuk pencarian parent dan level
CREATE INDEX IF NOT EXISTS idx_app_menus_parent ON public.app_menus (parent_id);
CREATE INDEX IF NOT EXISTS idx_app_menus_level ON public.app_menus (level);

-- 3. Reload Schema Cache Supabase PostgREST
NOTIFY pgrst, 'reload schema';
NOTIFY pgrst, 'reload config';
