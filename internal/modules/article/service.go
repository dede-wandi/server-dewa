package article

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dede-wandi/server-dewa/internal/supabase"
)

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func slugify(title string) string {
	lowered := strings.ToLower(title)
	reg := regexp.MustCompile("[^a-z0-9]+")
	cleaned := reg.ReplaceAllString(lowered, "-")
	return strings.Trim(cleaned, "-")
}

type Service interface {
	GetArticles(category, search string) ([]Article, error)
	GetArticleBySlugOrID(slugOrID string) (*Article, error)
	GetAllArticlesAdmin(category, search string) ([]Article, error)
	CreateArticle(req CreateArticleRequest) (*Article, error)
	UpdateArticle(id string, req UpdateArticleRequest) (*Article, error)
	DeleteArticle(id string) error
}

type articleService struct {
	sbClient *supabase.Client
	mu       sync.RWMutex
	fallback []Article
}

func NewService(sbClient *supabase.Client) Service {
	now := time.Now()
	defaults := []Article{
		{
			ID:           "b1111111-0000-0000-0000-000000000001",
			Title:        "Cara Cetak Struk Kasir POS Pakai Printer Bluetooth Thermal",
			Slug:         "cara-cetak-struk-kasir-pos-bluetooth",
			Category:     "PANDUAN_KASIR",
			Excerpt:      "Panduan praktis menghubungkan printer thermal bluetooth 58mm/80mm ke aplikasi DewaCell untuk cetak bukti transaksi pelanggan.",
			Content:      "Mencetak struk fisik bagi pelanggan konter dan toko kelontong memberikan rasa percaya yang jauh lebih tinggi. Berikut adalah langkah mudah menghubungkan printer thermal bluetooth Anda:\n\n1. **Nyalakan Printer Bluetooth**: Pastikan printer dalam mode ON dan lampu indikator bluetooth berkedip.\n2. **Buka Menu Pengaturan DewaCell**: Masuk ke Tab Akun/Pengaturan -> pilih 'Hubungkan Printer Thermal'.\n3. **Lakukan Scanning Perangkat**: Klik tombol 'Cari Printer', pilih nama bluetooth printer Anda (biasanya bernama RPP02N, MTP-2, atau Bluetooth Printer).\n4. **Tes Cetak Struk**: Setelah terhubung, klik 'Tes Print'. Jika kertas keluar dengan tulisan DewaCell POS, printer siap digunakan untuk setiap transaksi pulsa, token, dan kasir kelontong.",
			ThumbnailURL: "https://images.unsplash.com/photo-1556742049-0a67c5574f73?w=800&auto=format&fit=crop&q=80",
			Author:       "DewaCell Support",
			ReadingTime:  3,
			IsPublished:  true,
			ViewCount:    142,
			Tags:         []string{"printer", "bluetooth", "struk", "kasir"},
			PublishedAt:  now,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "b1111111-0000-0000-0000-000000000002",
			Title:        "5 Strategi Menaikkan Omzet Jualan Pulsa & PPOB di Konter",
			Slug:         "5-strategi-menaikkan-omzet-jualan-pulsa-ppob",
			Category:     "TIPS_BISNIS",
			Excerpt:      "Tips terbukti dari mitra sukses DewaCell dalam melipatgandakan keuntungan harian toko pulsa dan pembayaran tagihan.",
			Content:      "Bisnis pulsa dan PPOB bukan hanya sekadar menunggu pembeli datang. Dengan strategi yang tepat, Anda bisa meningkatkan transaksi hingga 3 kali lipat:\n\n1. **Sediakan Layanan Lengkap (One Stop Service)**: Jangan hanya jual pulsa seluler. Aktifkan pembayaran token PLN, BPJS, PDAM, dan Top Up E-Wallet (DANA, GoPay, OVO).\n2. **Tampilkan Banner Promo di Depan Toko**: Pasang spanduk harga transparan agar warga sekitar tahu konter Anda melayani semua jenis pembayaran online 24 jam.\n3. **Manfaatkan Pembayaran QRIS**: Banyak pelanggan muda saat ini tidak membawa uang tunai. Aktifkan fitur QRIS DewaCell agar pelanggan bisa scan langsung dari aplikasi bank apa saja.\n4. **Jaga Ketersediaan Saldo**: Jangan sampai menolak pelanggan karena saldo dompet kosong. Manfaatkan fitur Top Up instan via QRIS gratis admin di aplikasi DewaCell.\n5. **Berikan Pelayanan Ramah & Cepat**: Kecepatan transaksi adalah kunci pelanggan kembali lagi.",
			ThumbnailURL: "https://images.unsplash.com/photo-1460925895917-afdab827c52f?w=800&auto=format&fit=crop&q=80",
			Author:       "DewaCell Business Advisor",
			ReadingTime:  4,
			IsPublished:  true,
			ViewCount:    320,
			Tags:         []string{"tips", "omzet", "bisnis", "mitra"},
			PublishedAt:  now,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "b1111111-0000-0000-0000-000000000003",
			Title:        "Update Keamanan: Proteksi Saldo dengan PIN 6 Digit",
			Slug:         "update-keamanan-proteksi-saldo-pin-6-digit",
			Category:     "UPDATE_SISTEM",
			Excerpt:      "Fitur keamanan dompet digital DewaCell kini diperketat dengan enkripsi PIN 6 digit untuk transfer dan pencairan saldo.",
			Content:      "Keamanan dana mitra adalah prioritas nomor satu di DewaCell. Mulai pembaruan versi ini, setiap transaksi pemindahan dana dompet wajib diverifikasi dengan PIN Keamanan:\n\n* **Kapan PIN Digunakan?**: Saat melakukan transfer saldo antar mitra, pembayaran merchant, dan penarikan saldo.\n* **Cara Pasang PIN**: Buka Tab Dompet -> pilih menu 'Pengaturan PIN' -> masukkan 6 digit angka rahasia Anda.\n* **Tips Aman**: Hindari menggunakan kombinasi tanggal lahir atau angka berurutan (seperti 123456). Jangan pernah membagikan PIN kepada siapa pun termasuk pihak yang mengaku sebagai staf DewaCell.",
			ThumbnailURL: "https://images.unsplash.com/photo-1563986768609-322da13575f3?w=800&auto=format&fit=crop&q=80",
			Author:       "Security Officer DewaCell",
			ReadingTime:  2,
			IsPublished:  true,
			ViewCount:    215,
			Tags:         []string{"keamanan", "pin", "saldo", "update"},
			PublishedAt:  now,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}

	return &articleService{
		sbClient: sbClient,
		fallback: defaults,
	}
}

// GetArticles returns published articles with category and search filtering
func (s *articleService) GetArticles(category, search string) ([]Article, error) {
	var parts []string
	parts = append(parts, "is_published=eq.true")

	if category != "" && strings.ToUpper(category) != "ALL" {
		parts = append(parts, fmt.Sprintf("category=eq.%s", url.QueryEscape(strings.ToUpper(category))))
	}
	if search != "" {
		parts = append(parts, fmt.Sprintf("title=ilike.*%s*", url.QueryEscape(search)))
	}
	parts = append(parts, "order=published_at.desc")
	queryParams := strings.Join(parts, "&")

	respBytes, err := s.sbClient.GetDB("articles", queryParams, "")
	if err != nil {
		log.Printf("[ArticleService] DB query failed (%v), using fallback articles", err)
		return s.filterFallback(true, category, search), nil
	}

	var articles []Article
	if err := json.Unmarshal(respBytes, &articles); err != nil {
		log.Printf("[ArticleService] Failed to unmarshal articles: %v", err)
		return s.filterFallback(true, category, search), nil
	}

	if len(articles) == 0 {
		return s.filterFallback(true, category, search), nil
	}

	return articles, nil
}

// GetArticleBySlugOrID retrieves single article detail
func (s *articleService) GetArticleBySlugOrID(slugOrID string) (*Article, error) {
	// Query by slug or id
	queryParams := fmt.Sprintf("or=(slug.eq.%s,id.eq.%s)", url.QueryEscape(slugOrID), url.QueryEscape(slugOrID))
	respBytes, err := s.sbClient.GetDB("articles", queryParams, "")
	if err != nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		for _, a := range s.fallback {
			if a.Slug == slugOrID || a.ID == slugOrID {
				return &a, nil
			}
		}
		return nil, fmt.Errorf("artikel tidak ditemukan: %w", err)
	}

	var list []Article
	if err := json.Unmarshal(respBytes, &list); err != nil || len(list) == 0 {
		s.mu.RLock()
		defer s.mu.RUnlock()
		for _, a := range s.fallback {
			if a.Slug == slugOrID || a.ID == slugOrID {
				return &a, nil
			}
		}
		return nil, errors.New("artikel tidak ditemukan")
	}

	return &list[0], nil
}

// GetAllArticlesAdmin returns all articles for admin
func (s *articleService) GetAllArticlesAdmin(category, search string) ([]Article, error) {
	var parts []string
	if category != "" && strings.ToUpper(category) != "ALL" {
		parts = append(parts, fmt.Sprintf("category=eq.%s", url.QueryEscape(strings.ToUpper(category))))
	}
	if search != "" {
		parts = append(parts, fmt.Sprintf("title=ilike.*%s*", url.QueryEscape(search)))
	}
	parts = append(parts, "order=created_at.desc")
	queryParams := strings.Join(parts, "&")

	respBytes, err := s.sbClient.GetDB("articles", queryParams, "")
	if err != nil {
		return s.filterFallback(false, category, search), nil
	}

	var list []Article
	if err := json.Unmarshal(respBytes, &list); err != nil || len(list) == 0 {
		return s.filterFallback(false, category, search), nil
	}

	return list, nil
}

// CreateArticle inserts new article
func (s *articleService) CreateArticle(req CreateArticleRequest) (*Article, error) {
	if req.Title == "" {
		return nil, errors.New("judul artikel wajib diisi")
	}
	if req.Content == "" {
		return nil, errors.New("konten artikel wajib diisi")
	}

	slug := req.Slug
	if slug == "" {
		slug = slugify(req.Title)
	}

	category := req.Category
	if category == "" {
		category = "TIPS_BISNIS"
	}

	author := req.Author
	if author == "" {
		author = "Tim DewaCell"
	}

	readingTime := req.ReadingTime
	if readingTime <= 0 {
		readingTime = 3
	}

	isPublished := true
	if req.IsPublished != nil {
		isPublished = *req.IsPublished
	}

	newID := newUUID()
	now := time.Now()

	payload := map[string]interface{}{
		"id":            newID,
		"title":         req.Title,
		"slug":          slug,
		"category":      strings.ToUpper(category),
		"excerpt":       req.Excerpt,
		"content":       req.Content,
		"thumbnail_url": req.ThumbnailURL,
		"author":        author,
		"reading_time":  readingTime,
		"is_published":  isPublished,
		"view_count":    0,
		"tags":          req.Tags,
		"published_at":  now,
		"created_at":    now,
		"updated_at":    now,
	}

	respBytes, err := s.sbClient.PostDB("articles", payload, "return=representation", "")
	if err != nil {
		log.Printf("[ArticleService] DB insert failed: %v, writing to fallback memory", err)
		item := Article{
			ID:           newID,
			Title:        req.Title,
			Slug:         slug,
			Category:     strings.ToUpper(category),
			Excerpt:      req.Excerpt,
			Content:      req.Content,
			ThumbnailURL: req.ThumbnailURL,
			Author:       author,
			ReadingTime:  readingTime,
			IsPublished:  isPublished,
			Tags:         req.Tags,
			PublishedAt:  now,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		s.mu.Lock()
		s.fallback = append(s.fallback, item)
		s.mu.Unlock()
		return &item, nil
	}

	var inserted []Article
	if err := json.Unmarshal(respBytes, &inserted); err == nil && len(inserted) > 0 {
		return &inserted[0], nil
	}

	return s.GetArticleBySlugOrID(slug)
}

// UpdateArticle updates an existing article
func (s *articleService) UpdateArticle(id string, req UpdateArticleRequest) (*Article, error) {
	payload := make(map[string]interface{})
	now := time.Now()
	payload["updated_at"] = now

	if req.Title != nil {
		payload["title"] = *req.Title
	}
	if req.Slug != nil {
		payload["slug"] = *req.Slug
	}
	if req.Category != nil {
		payload["category"] = strings.ToUpper(*req.Category)
	}
	if req.Excerpt != nil {
		payload["excerpt"] = *req.Excerpt
	}
	if req.Content != nil {
		payload["content"] = *req.Content
	}
	if req.ThumbnailURL != nil {
		payload["thumbnail_url"] = *req.ThumbnailURL
	}
	if req.Author != nil {
		payload["author"] = *req.Author
	}
	if req.ReadingTime != nil {
		payload["reading_time"] = *req.ReadingTime
	}
	if req.IsPublished != nil {
		payload["is_published"] = *req.IsPublished
	}
	if req.Tags != nil {
		payload["tags"] = *req.Tags
	}

	queryParams := fmt.Sprintf("id=eq.%s", url.QueryEscape(id))
	respBytes, err := s.sbClient.PatchDB("articles", queryParams, payload, "")
	if err != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, a := range s.fallback {
			if a.ID == id {
				if req.Title != nil {
					s.fallback[i].Title = *req.Title
				}
				if req.Excerpt != nil {
					s.fallback[i].Excerpt = *req.Excerpt
				}
				if req.Content != nil {
					s.fallback[i].Content = *req.Content
				}
				if req.Category != nil {
					s.fallback[i].Category = strings.ToUpper(*req.Category)
				}
				s.fallback[i].UpdatedAt = now
				return &s.fallback[i], nil
			}
		}
		return nil, fmt.Errorf("artikel tidak ditemukan: %w", err)
	}

	var updated []Article
	if err := json.Unmarshal(respBytes, &updated); err == nil && len(updated) > 0 {
		return &updated[0], nil
	}

	return s.GetArticleBySlugOrID(id)
}

// DeleteArticle deletes an article
func (s *articleService) DeleteArticle(id string) error {
	queryParams := fmt.Sprintf("id=eq.%s", url.QueryEscape(id))
	err := s.sbClient.DeleteDB("articles", queryParams, "")
	if err != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		newFallback := make([]Article, 0, len(s.fallback))
		for _, a := range s.fallback {
			if a.ID != id {
				newFallback = append(newFallback, a)
			}
		}
		s.fallback = newFallback
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	newFallback := make([]Article, 0, len(s.fallback))
	for _, a := range s.fallback {
		if a.ID != id {
			newFallback = append(newFallback, a)
		}
	}
	s.fallback = newFallback
	return nil
}

func (s *articleService) filterFallback(onlyPublished bool, category, search string) []Article {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Article
	cat := strings.ToUpper(strings.TrimSpace(category))
	q := strings.ToLower(strings.TrimSpace(search))

	for _, a := range s.fallback {
		if onlyPublished && !a.IsPublished {
			continue
		}
		if cat != "" && cat != "ALL" && strings.ToUpper(a.Category) != cat {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(a.Title), q) && !strings.Contains(strings.ToLower(a.Excerpt), q) {
			continue
		}
		result = append(result, a)
	}

	return result
}
