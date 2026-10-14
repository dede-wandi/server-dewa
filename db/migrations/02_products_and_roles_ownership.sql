-- =====================================================================
-- DEWACELL RBAC & MULTI-MERCHANT PRODUCT OWNERSHIP MIGRATION
-- Jalankan query ini di: Supabase Dashboard -> SQL Editor
-- =====================================================================

-- 1. Tambah Role FnB & Merchant Baru ke Tabel Master Roles
INSERT INTO public.roles (id, name, description) VALUES
    ('admin',     'Super Administrator', 'Akses penuh ke semua produk, merchant, transaksi, dan user'),
    ('fnb',       'Admin FnB / Merchant', 'Admin merchant makanan & minuman, hanya mengelola produk miliknya sendiri'),
    ('groceries', 'Admin Sembako / Pasar', 'Admin toko sembako/pasar, hanya mengelola produk miliknya sendiri'),
    ('user',      'Pelanggan / Pembeli', 'Dapat membeli produk admin dan produk semua merchant')
ON CONFLICT (id) DO UPDATE 
SET name = EXCLUDED.name, description = EXCLUDED.description;

-- 2. Buat Tabel Produk dengan Kepemilikan (Ownership & Role)
CREATE TABLE IF NOT EXISTS public.products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    role_owner TEXT NOT NULL REFERENCES public.roles(id) DEFAULT 'admin', -- 'admin', 'fnb', 'groceries'
    name TEXT NOT NULL,
    category TEXT NOT NULL,          -- 'pulsa', 'data', 'pln', 'fnb', 'makanan', 'minuman'
    description TEXT,
    price NUMERIC(15, 2) NOT NULL CHECK (price >= 0),
    cost_price NUMERIC(15, 2) DEFAULT 0,
    stock INT NOT NULL DEFAULT 0,
    unit TEXT DEFAULT 'pcs',         -- 'pcs', 'porsi', 'item'
    image_url TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexing untuk kecepatan pencarian produk
CREATE INDEX IF NOT EXISTS idx_products_owner_id ON public.products(owner_id);
CREATE INDEX IF NOT EXISTS idx_products_role_owner ON public.products(role_owner);
CREATE INDEX IF NOT EXISTS idx_products_category ON public.products(category);
CREATE INDEX IF NOT EXISTS idx_products_is_active ON public.products(is_active);

-- =====================================================================
-- 3. QUERY PRAKTIS UNTUK ROLE ADMIN & PENGELOLAAN PRODUK
-- =====================================================================

-- [A] QUERY MELIHAT SEMUA USER DENGAN ROLE ADMIN
-- Cara 1: Menggunakan tabel relasi user_roles
SELECT 
    u.id AS user_id,
    u.email,
    u.raw_user_meta_data->>'full_name' AS full_name,
    u.raw_user_meta_data->>'phone' AS phone,
    r.id AS role_id,
    r.name AS role_name,
    ur.assigned_at
FROM auth.users u
JOIN public.user_roles ur ON u.id = ur.user_id
JOIN public.roles r ON ur.role_id = r.id
WHERE ur.role_id = 'admin'
ORDER BY ur.assigned_at DESC;

-- Cara 2: Langsung dari metadata JWT auth.users
SELECT 
    id, 
    email, 
    raw_user_meta_data->>'full_name' AS full_name,
    raw_app_meta_data->'roles' AS roles
FROM auth.users
WHERE raw_app_meta_data->'roles' ? 'admin';

-- [B] QUERY MENJADIKAN USER TERTENTU SEBAGAI ADMIN (ASSIGN ADMIN)
-- Ganti 'user@gmail.com' dengan email yang ingin diangkat jadi admin
SELECT public.assign_user_role('user@gmail.com', 'admin');

-- [C] QUERY MENJADIKAN USER SEBAGAI ADMIN FNB
-- Ganti 'fnb_owner@gmail.com' dengan email merchant FnB
SELECT public.assign_user_role('fnb_owner@gmail.com', 'fnb');


-- =====================================================================
-- 4. QUERY BISNIS: ATURAN PRODUK ADMIN VS PRODUK MERCHANT (FNB)
-- =====================================================================

-- [D] QUERY UNTUK PEMBELI / CATALOG BELANJA (DAPAT MELIHAT SEMUA PRODUK AKTIF)
-- Produk Admin (Pulsa, PPOB) BISA DIBELI SEMUA ROLE.
-- Produk FnB / Merchant lain juga tampil untuk dibeli oleh pelanggan.
SELECT 
    p.id,
    p.name,
    p.category,
    p.price,
    p.stock,
    p.unit,
    p.role_owner,
    CASE 
        WHEN p.role_owner = 'admin' THEN 'Official DewaCell'
        ELSE COALESCE(u.raw_user_meta_data->>'business_name', u.raw_user_meta_data->>'full_name', 'Mitra Merchant')
    END AS merchant_name,
    p.is_active
FROM public.products p
JOIN auth.users u ON p.owner_id = u.id
WHERE p.is_active = true
ORDER BY (p.role_owner = 'admin') DESC, p.created_at DESC;

-- [E] QUERY KHUSUS HALAMAN ADMIN FNB / MERCHANT MASING-MASING
-- Admin FnB hanya boleh melihat dan mengelola produk milik toko mereka sendiri:
-- Ganti parameter :current_user_id dengan ID user yang sedang login
-- SELECT * FROM public.products WHERE owner_id = :current_user_id;

-- [F] QUERY KHUSUS SUPER ADMIN
-- Super Admin dapat melihat dan mengedit SELURUH PRODUK (baik milik admin maupun semua merchant FnB):
-- SELECT * FROM public.products ORDER BY created_at DESC;
