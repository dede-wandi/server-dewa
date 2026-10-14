-- ==============================================================================
-- DEWACELL DATABASE MIGRATION 11: NEWS & MITRA EDUCATION ARTICLES
-- File: 11_create_articles_and_news.sql
-- Description: Tabel artikel berita, tips jualan konter, dan panduan edukasi mitra
-- ==============================================================================

CREATE TABLE IF NOT EXISTS public.articles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    category VARCHAR(100) DEFAULT 'TIPS_BISNIS', -- 'TIPS_BISNIS', 'UPDATE_SISTEM', 'PANDUAN_KASIR', 'PROMO'
    excerpt TEXT,                                 -- Ringkasan singkat untuk list kartu
    content TEXT NOT NULL,                        -- Isi artikel berita/panduan lengkap
    thumbnail_url TEXT,                           -- Gambar cover artikel
    author VARCHAR(100) DEFAULT 'Tim DewaCell',
    reading_time INT DEFAULT 3,                   -- Estimasi waktu baca (menit)
    is_published BOOLEAN DEFAULT true,
    view_count INT DEFAULT 0,
    tags TEXT[] DEFAULT ARRAY['dewacell', 'edukasi']::TEXT[],
    published_at TIMESTAMPTZ DEFAULT now(),
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Indexing
CREATE INDEX IF NOT EXISTS idx_articles_published_cat ON public.articles(is_published, category, published_at DESC);
CREATE INDEX IF NOT EXISTS idx_articles_slug ON public.articles(slug);

-- Enable RLS
ALTER TABLE public.articles ENABLE ROW LEVEL SECURITY;

-- 1. Policy Public: Semua pengguna dapat membaca artikel yang sudah terbit
DROP POLICY IF EXISTS "Public read published articles" ON public.articles;
CREATE POLICY "Public read published articles" ON public.articles
    FOR SELECT
    TO anon, authenticated
    USING (is_published = true AND published_at <= now());

-- 2. Policy Service Role: Backend service_role memiliki akses penuh
DROP POLICY IF EXISTS "Service role full access articles" ON public.articles;
CREATE POLICY "Service role full access articles" ON public.articles
    FOR ALL
    TO service_role
    USING (true)
    WITH CHECK (true);

-- 3. Policy Admin Management: Admin & Superadmin CRUD artikel
DROP POLICY IF EXISTS "Admin full control articles" ON public.articles;
CREATE POLICY "Admin full control articles" ON public.articles
    FOR ALL
    TO authenticated
    USING (
        EXISTS (
            SELECT 1 FROM public.user_roles ur
            JOIN public.roles r ON ur.role_id = r.id
            WHERE ur.user_id = auth.uid() AND r.name IN ('admin', 'superadmin')
        )
    )
    WITH CHECK (
        EXISTS (
            SELECT 1 FROM public.user_roles ur
            JOIN public.roles r ON ur.role_id = r.id
            WHERE ur.user_id = auth.uid() AND r.name IN ('admin', 'superadmin')
        )
    );

-- Hak akses table
GRANT ALL ON TABLE public.articles TO anon, authenticated, service_role;

-- ==============================================================================
-- INITIAL SEED DATA ARTIKEL EDUKASI MITRA
-- ==============================================================================
INSERT INTO public.articles (
    title, slug, category, excerpt, content, thumbnail_url, author, reading_time, is_published, tags
)
VALUES
(
    'Cara Cetak Struk Kasir POS Pakai Printer Bluetooth Thermal',
    'cara-cetak-struk-kasir-pos-bluetooth',
    'PANDUAN_KASIR',
    'Panduan praktis menghubungkan printer thermal bluetooth 58mm/80mm ke aplikasi DewaCell untuk cetak bukti transaksi pelanggan.',
    'Mencetak struk fisik bagi pelanggan konter dan toko kelontong memberikan rasa percaya yang jauh lebih tinggi. Berikut adalah langkah mudah menghubungkan printer thermal bluetooth Anda:\n\n1. **Nyalakan Printer Bluetooth**: Pastikan printer dalam mode ON dan lampu indikator bluetooth berkedip.\n2. **Buka Menu Pengaturan DewaCell**: Masuk ke Tab Akun/Pengaturan -> pilih "Hubungkan Printer Thermal".\n3. **Lakukan Scanning Perangkat**: Klik tombol "Cari Printer", pilih nama bluetooth printer Anda (biasanya bernama RPP02N, MTP-2, atau Bluetooth Printer).\n4. **Tes Cetak Struk**: Setelah terhubung, klik "Tes Print". Jika kertas keluar dengan tulisan DewaCell POS, printer siap digunakan untuk setiap transaksi pulsa, token, dan kasir kelontong.',
    'https://images.unsplash.com/photo-1556742049-0a67c5574f73?w=800&auto=format&fit=crop&q=80',
    'DewaCell Support',
    3,
    true,
    ARRAY['printer', 'bluetooth', 'struk', 'kasir']::TEXT[]
),
(
    '5 Strategi Menaikkan Omzet Jualan Pulsa & PPOB di Konter',
    '5-strategi-menaikkan-omzet-jualan-pulsa-ppob',
    'TIPS_BISNIS',
    'Tips terbukti dari mitra sukses DewaCell dalam melipatgandakan keuntungan harian toko pulsa dan pembayaran tagihan.',
    'Bisnis pulsa dan PPOB bukan hanya sekadar menunggu pembeli datang. Dengan strategi yang tepat, Anda bisa meningkatkan transaksi hingga 3 kali lipat:\n\n1. **Sediakan Layanan Lengkap (One Stop Service)**: Jangan hanya jual pulsa seluler. Aktifkan pembayaran token PLN, BPJS, PDAM, dan Top Up E-Wallet (DANA, GoPay, OVO).\n2. **Tampilkan Banner Promo di Depan Toko**: Pasang spanduk harga transparan agar warga sekitar tahu konter Anda melayani semua jenis pembayaran online 24 jam.\n3. **Manfaatkan Pembayaran QRIS**: Banyak pelanggan muda saat ini tidak membawa uang tunai. Aktifkan fitur QRIS DewaCell agar pelanggan bisa scan langsung dari aplikasi bank apa saja.\n4. **Jaga Ketersediaan Saldo**: Jangan sampai menolak pelanggan karena saldo dompet kosong. Manfaatkan fitur Top Up instan via QRIS gratis admin di aplikasi DewaCell.\n5. **Berikan Pelayanan Ramah & Cepat**: Kecepatan transaksi adalah kunci pelanggan kembali lagi.',
    'https://images.unsplash.com/photo-1460925895917-afdab827c52f?w=800&auto=format&fit=crop&q=80',
    'DewaCell Business Advisor',
    4,
    true,
    ARRAY['tips', 'omzet', 'bisnis', 'mitra']::TEXT[]
),
(
    'Update Keamanan: Proteksi Saldo dengan PIN 6 Digit',
    'update-keamanan-proteksi-saldo-pin-6-digit',
    'UPDATE_SISTEM',
    'Fitur keamanan dompet digital DewaCell kini diperketat dengan enkripsi PIN 6 digit untuk transfer dan pencairan saldo.',
    'Keamanan dana mitra adalah prioritas nomor satu di DewaCell. Mulai pembaruan versi ini, setiap transaksi pemindahan dana dompet wajib diverifikasi dengan PIN Keamanan:\n\n* **Kapan PIN Digunakan?**: Saat melakukan transfer saldo antar mitra, pembayaran merchant, dan penarikan saldo.\n* **Cara Pasang PIN**: Buka Tab Dompet -> pilih menu "Pengaturan PIN" -> masukkan 6 digit angka rahasia Anda.\n* **Tips Aman**: Hindari menggunakan kombinasi tanggal lahir atau angka berurutan (seperti 123456). Jangan pernah membagikan PIN kepada siapa pun termasuk pihak yang mengaku sebagai staf DewaCell.',
    'https://images.unsplash.com/photo-1563986768609-322da13575f3?w=800&auto=format&fit=crop&q=80',
    'Security Officer DewaCell',
    2,
    true,
    ARRAY['keamanan', 'pin', 'saldo', 'update']::TEXT[]
)
ON CONFLICT (slug) DO UPDATE SET
    title = EXCLUDED.title,
    excerpt = EXCLUDED.excerpt,
    content = EXCLUDED.content,
    thumbnail_url = EXCLUDED.thumbnail_url,
    category = EXCLUDED.category,
    reading_time = EXCLUDED.reading_time,
    is_published = EXCLUDED.is_published,
    tags = EXCLUDED.tags;
