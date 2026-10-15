-- ==============================================================================
-- DEWACELL DATABASE MIGRATION 15: NOTIFICATIONS & LOGIN LOGS INSERT POLICIES
-- File: 15_allow_notifications_and_logs_insert.sql
-- Description: Mengizinkan authenticated & backend service/anon mencatat log login
--              dan notifikasi keamanan pengguna ke tabel database Supabase
-- ==============================================================================

-- 1. Policy Insert Notifications: Mengizinkan authenticated dan backend anon memasukkan notifikasi
DROP POLICY IF EXISTS "Users can insert own notifications" ON public.notifications;
CREATE POLICY "Users can insert own notifications" ON public.notifications
    FOR INSERT
    TO authenticated
    WITH CHECK (auth.uid() = user_id);

DROP POLICY IF EXISTS "Allow anon insert notifications" ON public.notifications;
CREATE POLICY "Allow anon insert notifications" ON public.notifications
    FOR INSERT
    TO anon
    WITH CHECK (true);

-- 2. Policy Insert User Login Logs: Mengizinkan pencatatan audit log login
DROP POLICY IF EXISTS "Users can insert own login logs" ON public.user_login_logs;
CREATE POLICY "Users can insert own login logs" ON public.user_login_logs
    FOR INSERT
    TO authenticated
    WITH CHECK (auth.uid() = user_id);

DROP POLICY IF EXISTS "Allow anon insert login logs" ON public.user_login_logs;
CREATE POLICY "Allow anon insert login logs" ON public.user_login_logs
    FOR INSERT
    TO anon
    WITH CHECK (true);

-- 3. Reload Schema Cache PostgREST
NOTIFY pgrst, 'reload schema';
NOTIFY pgrst, 'reload config';
