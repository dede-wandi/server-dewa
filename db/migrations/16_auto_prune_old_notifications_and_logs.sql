-- ==============================================================================
-- DEWACELL DATABASE MIGRATION 16: AUTO PRUNE / CLEANUP OLD LOGS & NOTIFICATIONS
-- File: 16_auto_prune_old_notifications_and_logs.sql
-- Description: Trigger otomatis di PostgreSQL Supabase untuk membatasi jumlah
--              notifikasi dan log login maksimal 20 record per user.
--              Data lama otomatis dihapus (auto-delete) saat record baru masuk.
-- ==============================================================================

-- 1. Trigger Auto Prune: Tabel public.notifications (Maksimal 20 Notifikasi per User)
CREATE OR REPLACE FUNCTION public.prune_old_user_notifications()
RETURNS TRIGGER 
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
    DELETE FROM public.notifications
    WHERE user_id = NEW.user_id
      AND id NOT IN (
          SELECT id 
          FROM public.notifications 
          WHERE user_id = NEW.user_id 
          ORDER BY created_at DESC 
          LIMIT 20
      );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_prune_old_notifications ON public.notifications;
CREATE TRIGGER trg_prune_old_notifications
    AFTER INSERT ON public.notifications
    FOR EACH ROW
    EXECUTE FUNCTION public.prune_old_user_notifications();


-- 2. Trigger Auto Prune: Tabel public.user_login_logs (Maksimal 20 Log Login per User)
CREATE OR REPLACE FUNCTION public.prune_old_user_login_logs()
RETURNS TRIGGER 
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
    DELETE FROM public.user_login_logs
    WHERE user_id = NEW.user_id
      AND id NOT IN (
          SELECT id 
          FROM public.user_login_logs 
          WHERE user_id = NEW.user_id 
          ORDER BY created_at DESC 
          LIMIT 20
      );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_prune_old_login_logs ON public.user_login_logs;
CREATE TRIGGER trg_prune_old_login_logs
    AFTER INSERT ON public.user_login_logs
    FOR EACH ROW
    EXECUTE FUNCTION public.prune_old_user_login_logs();


-- 3. Reload Cache Schema PostgREST
NOTIFY pgrst, 'reload schema';
NOTIFY pgrst, 'reload config';
