-- ==============================================================================
-- DEWACELL DATABASE MIGRATION 12: USER & ADMIN NOTIFICATIONS
-- File: 12_create_notifications.sql
-- Description: Tabel notifikasi sistem, transaksi, pembayaran, promo & pengumuman
-- ==============================================================================

CREATE TABLE IF NOT EXISTS public.notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    type VARCHAR(50) DEFAULT 'ANNOUNCEMENT', -- 'TRANSACTION', 'PAYMENT', 'PROMO', 'ANNOUNCEMENT', 'SECURITY'
    status_baca VARCHAR(20) DEFAULT 'Belum Dibaca', -- 'Belum Dibaca', 'Sudah Dibaca'
    is_read BOOLEAN DEFAULT false,
    reference_id VARCHAR(100), -- ID transaksi / kode pesanan / reference
    action_url TEXT,           -- Deep link URL (contoh: '/(tabs)/orders', '/(tabs)/wallet')
    created_at TIMESTAMPTZ DEFAULT now(),
    read_at TIMESTAMPTZ
);

-- Indexing for fast fetch (latest 20 notifications per user)
CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON public.notifications(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON public.notifications(user_id, is_read);
CREATE INDEX IF NOT EXISTS idx_notifications_type ON public.notifications(type);

-- Enable RLS
ALTER TABLE public.notifications ENABLE ROW LEVEL SECURITY;

-- 1. Policy User Read: Pengguna hanya dapat membaca notifikasi miliknya sendiri
DROP POLICY IF EXISTS "Users can read own notifications" ON public.notifications;
CREATE POLICY "Users can read own notifications" ON public.notifications
    FOR SELECT
    TO authenticated
    USING (auth.uid() = user_id);

-- 2. Policy User Update: Pengguna dapat menandai notifikasi miliknya sebagai dibaca/belum dibaca
DROP POLICY IF EXISTS "Users can update own notifications" ON public.notifications;
CREATE POLICY "Users can update own notifications" ON public.notifications
    FOR UPDATE
    TO authenticated
    USING (auth.uid() = user_id)
    WITH CHECK (auth.uid() = user_id);

-- 3. Policy User Delete: Pengguna dapat menghapus notifikasi miliknya sendiri
DROP POLICY IF EXISTS "Users can delete own notifications" ON public.notifications;
CREATE POLICY "Users can delete own notifications" ON public.notifications
    FOR DELETE
    TO authenticated
    USING (auth.uid() = user_id);

-- 4. Policy Service Role: Backend service_role memiliki akses penuh (CRUD)
DROP POLICY IF EXISTS "Service role full access notifications" ON public.notifications;
CREATE POLICY "Service role full access notifications" ON public.notifications
    FOR ALL
    TO service_role
    USING (true)
    WITH CHECK (true);
