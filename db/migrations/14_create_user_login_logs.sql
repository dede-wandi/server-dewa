-- ==============================================================================
-- DEWACELL DATABASE MIGRATION 14: USER LOGIN & DEVICE AUDIT LOGS
-- File: 14_create_user_login_logs.sql
-- Description: Pencatatan riwayat login user dengan deteksi device_id, device_name, OS, dan IP
--              Mendukung perlindungan keamanan akun dan audit perangkat
-- ==============================================================================

CREATE TABLE IF NOT EXISTS public.user_login_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    email VARCHAR(255) NOT NULL,
    device_id VARCHAR(100) DEFAULT '',
    device_name VARCHAR(100) DEFAULT '',
    device_os VARCHAR(100) DEFAULT '',
    location VARCHAR(150) DEFAULT '',
    ip_address VARCHAR(50) DEFAULT '',
    user_agent TEXT DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Ensure column exists if table was previously created
ALTER TABLE public.user_login_logs ADD COLUMN IF NOT EXISTS location VARCHAR(150) DEFAULT '';

-- Indexing for fast fetch
CREATE INDEX IF NOT EXISTS idx_user_login_logs_user ON public.user_login_logs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_login_logs_device ON public.user_login_logs(device_id);
CREATE INDEX IF NOT EXISTS idx_user_login_logs_created ON public.user_login_logs(created_at DESC);

-- Enable RLS
ALTER TABLE public.user_login_logs ENABLE ROW LEVEL SECURITY;

-- 1. Policy User Read: Pengguna hanya dapat membaca log login miliknya sendiri
DROP POLICY IF EXISTS "Users can read own login logs" ON public.user_login_logs;
CREATE POLICY "Users can read own login logs" ON public.user_login_logs
    FOR SELECT
    TO authenticated
    USING (auth.uid() = user_id);

-- 2. Policy Service Role: Backend service_role memiliki akses penuh
DROP POLICY IF EXISTS "Service role full access login logs" ON public.user_login_logs;
CREATE POLICY "Service role full access login logs" ON public.user_login_logs
    FOR ALL
    TO service_role
    USING (true)
    WITH CHECK (true);
