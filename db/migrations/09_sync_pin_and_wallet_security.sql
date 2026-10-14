-- =====================================================================
-- DEWACELL PIN & WALLET SECURITY SYNC
-- Migration: 09_sync_pin_and_wallet_security.sql
-- =====================================================================

-- 1. Perbarui admin_get_user_detail agar menyertakan status PIN dan hash untuk verifikasi backend
CREATE OR REPLACE FUNCTION public.admin_get_user_detail(p_user_id UUID)
RETURNS JSONB AS $$
DECLARE
    v_user RECORD;
    v_wallet RECORD;
BEGIN
    SELECT * INTO v_user FROM auth.users WHERE id = p_user_id;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    SELECT * INTO v_wallet FROM public.app_wallets WHERE user_id = p_user_id;

    RETURN jsonb_build_object(
        'user_id', v_user.id,
        'user_email', v_user.email,
        'role', COALESCE(v_user.raw_app_meta_data->>'role', v_user.raw_app_meta_data->'roles'->>0, 'user'),
        'balance', COALESCE(v_wallet.balance, 0),
        'points', COALESCE(v_wallet.points, 0),
        'is_locked', COALESCE(v_wallet.is_locked, false),
        'is_pin_set', COALESCE(v_wallet.is_pin_set, false),
        'pin_hash', COALESCE(v_wallet.pin_hash, ''),
        'created_at', v_user.created_at,
        'updated_at', COALESCE(v_wallet.updated_at, v_user.created_at),
        'user_metadata', COALESCE(v_user.raw_user_meta_data, '{}'::jsonb),
        'app_metadata', COALESCE(v_user.raw_app_meta_data, '{}'::jsonb)
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- 2. Perbarui get_all_registered_users_wallets agar menyertakan is_pin_set dan pin_hash
CREATE OR REPLACE FUNCTION public.get_all_registered_users_wallets()
RETURNS JSONB AS $$
DECLARE
    v_list JSONB;
BEGIN
    SELECT jsonb_agg(
        jsonb_build_object(
            'user_id', u.id,
            'user_email', u.email,
            'role', COALESCE(u.raw_app_meta_data->>'role', u.raw_app_meta_data->'roles'->>0, 'user'),
            'balance', COALESCE(w.balance, 0),
            'points', COALESCE(w.points, 0),
            'is_locked', COALESCE(w.is_locked, false),
            'is_pin_set', COALESCE(w.is_pin_set, false),
            'pin_hash', COALESCE(w.pin_hash, ''),
            'created_at', u.created_at,
            'updated_at', COALESCE(w.updated_at, u.created_at),
            'user_metadata', COALESCE(u.raw_user_meta_data, '{}'::jsonb),
            'app_metadata', COALESCE(u.raw_app_meta_data, '{}'::jsonb)
        ) ORDER BY u.created_at DESC
    ) INTO v_list
    FROM auth.users u
    LEFT JOIN public.app_wallets w ON w.user_id = u.id;

    RETURN COALESCE(v_list, '[]'::jsonb);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

GRANT EXECUTE ON FUNCTION public.admin_get_user_detail(UUID) TO anon, authenticated, service_role;
GRANT EXECUTE ON FUNCTION public.get_all_registered_users_wallets() TO anon, authenticated, service_role;
