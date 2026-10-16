-- ==============================================================================
-- DEWACELL DATABASE MIGRATION 22: CRON SETTINGS & LIGHTWEIGHT SYNC LOGS
-- File: 22_create_cron_settings_and_sync_logs.sql
-- Description: Menyimpan interval waktu cronjob yang bisa diubah via Admin
--              serta riwayat perubahan (array JSONB ringkas agar hemat database).
-- ==============================================================================

-- 1. Tabel Pengaturan Cronjob Dinamis
CREATE TABLE IF NOT EXISTS public.cron_settings (
    key VARCHAR(100) PRIMARY KEY,
    interval_minutes INT NOT NULL DEFAULT 10,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    last_run_at TIMESTAMP WITH TIME ZONE,
    next_run_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB DEFAULT '{}'::jsonb,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Seed default setting untuk DFLASH sync jika belum ada
INSERT INTO public.cron_settings (key, interval_minutes, is_enabled)
VALUES ('dflash_sync', 10, true)
ON CONFLICT (key) DO NOTHING;

-- 2. Tabel Log Riwayat Sinkronisasi (Menggunakan array JSONB agar sangat hemat resource DB)
CREATE TABLE IF NOT EXISTS public.cron_sync_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_key VARCHAR(100) NOT NULL DEFAULT 'dflash_sync',
    status VARCHAR(50) NOT NULL DEFAULT 'SUCCESS', -- 'SUCCESS', 'ERROR', 'PARTIAL'
    total_checked INT NOT NULL DEFAULT 0,
    total_updated INT NOT NULL DEFAULT 0,
    gangguan_count INT NOT NULL DEFAULT 0,
    price_updated INT NOT NULL DEFAULT 0,
    status_updated INT NOT NULL DEFAULT 0,
    execution_ms INT NOT NULL DEFAULT 0,
    changes JSONB NOT NULL DEFAULT '[]'::jsonb,   -- Array objek perubahan SKU
    error_message TEXT DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cron_sync_logs_job_key ON public.cron_sync_logs(job_key);
CREATE INDEX IF NOT EXISTS idx_cron_sync_logs_created_at ON public.cron_sync_logs(created_at DESC);

-- 3. Row Level Security (RLS)
ALTER TABLE public.cron_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.cron_sync_logs ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS "cron_settings_full_policy" ON public.cron_settings;
CREATE POLICY "cron_settings_full_policy" ON public.cron_settings 
    FOR ALL 
    USING (true) 
    WITH CHECK (true);

DROP POLICY IF EXISTS "cron_sync_logs_full_policy" ON public.cron_sync_logs;
CREATE POLICY "cron_sync_logs_full_policy" ON public.cron_sync_logs 
    FOR ALL 
    USING (true) 
    WITH CHECK (true);
