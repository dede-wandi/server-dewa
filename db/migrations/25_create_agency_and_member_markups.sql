-- =====================================================================
-- MIGRATION 25: SISTEM KEAGENAN, DOWNLINE & MARKUP HARGA PER KATEGORI
-- =====================================================================

-- 1. DAFTARKAN ROLE 'agent' DI TABEL ROLES
INSERT INTO public.roles (id, name, description) VALUES
    ('agent', 'Agen / Upline', 'Mitra distributor dengan downline anggota dan kontrol markup harga kategori')
ON CONFLICT (id) DO UPDATE 
SET name = EXCLUDED.name, description = EXCLUDED.description;

-- 2. UPDATE KONSITENSI KATEGORI BUKU BESAR (Tambahkan 'COMMISSION')
ALTER TABLE public.app_wallet_transactions DROP CONSTRAINT IF EXISTS app_wallet_transactions_category_check;
ALTER TABLE public.app_wallet_transactions ADD CONSTRAINT app_wallet_transactions_category_check 
    CHECK (category IN ('TOPUP', 'PAYMENT', 'REFUND', 'ADJUSTMENT', 'TRANSFER_IN', 'TRANSFER_OUT', 'COMMISSION'));

-- 3. TABEL PROFIL AGEN & KODE REFERRAL
CREATE TABLE IF NOT EXISTS public.agency_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE REFERENCES auth.users(id) ON DELETE CASCADE,
    referral_code VARCHAR(32) NOT NULL UNIQUE,
    business_name VARCHAR(150),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agency_profiles_user_id ON public.agency_profiles(user_id);
CREATE INDEX IF NOT EXISTS idx_agency_profiles_code ON public.agency_profiles(referral_code);

-- 4. TABEL RELASI ANGGOTA (DOWNLINE)
CREATE TABLE IF NOT EXISTS public.agency_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    member_id UUID NOT NULL UNIQUE REFERENCES auth.users(id) ON DELETE CASCADE,
    member_name VARCHAR(150),
    member_phone VARCHAR(50),
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    joined_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agency_members_agent ON public.agency_members(agent_id);
CREATE INDEX IF NOT EXISTS idx_agency_members_member ON public.agency_members(member_id);

-- 5. TABEL MARKUP HARGA PER KATEGORI
-- member_id NULL = Default Template untuk semua anggota agen ini
-- member_id IS NOT NULL = Custom Markup khusus untuk anggota tertentu
CREATE TABLE IF NOT EXISTS public.agent_category_markups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    member_id UUID REFERENCES auth.users(id) ON DELETE CASCADE,
    category_id VARCHAR(100) NOT NULL,
    markup_amount NUMERIC(15, 2) NOT NULL DEFAULT 0 CHECK (markup_amount >= 0),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (agent_id, member_id, category_id)
);

CREATE INDEX IF NOT EXISTS idx_agent_markups_lookup ON public.agent_category_markups(agent_id, member_id, category_id);

-- 6. TAMBAH KOLOM PADA TABEL ORDERS UNTUK PENCATATAN KOMISI
ALTER TABLE public.orders ADD COLUMN IF NOT EXISTS upline_id UUID REFERENCES auth.users(id);
ALTER TABLE public.orders ADD COLUMN IF NOT EXISTS base_price NUMERIC(15, 2);
ALTER TABLE public.orders ADD COLUMN IF NOT EXISTS markup_amount NUMERIC(15, 2) DEFAULT 0;
ALTER TABLE public.orders ADD COLUMN IF NOT EXISTS commission_status VARCHAR(20) DEFAULT 'NONE';

CREATE INDEX IF NOT EXISTS idx_orders_upline_id ON public.orders(upline_id);
