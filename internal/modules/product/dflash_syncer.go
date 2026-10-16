package product

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dede-wandi/server-dewa/internal/supabase"
)

// DefaultDFlashReportURL URL publik laporan harga dan status realtime DFLASH
const DefaultDFlashReportURL = "https://report.dflash.co.id/harga.js.php?id=bd5ddc64723bb29352906c243512e116fe46fac768e449ba68546505d54565b82a5478d225d993b06a0406ab75297d48354b-1354"

// DFlashReportItem mewakili satu baris data SKU dari report DFLASH
type DFlashReportItem struct {
	SKU       string  `json:"sku"`
	Name      string  `json:"name"`
	CostPrice float64 `json:"cost_price"`
	Status    string  `json:"status"` // "Open", "Gangguan", dll
}

// SKUChangeItem objek perubahan per-produk yang disimpan dalam array log (hemat resource DB)
type SKUChangeItem struct {
	SKU       string  `json:"sku"`
	Name      string  `json:"name"`
	Type      string  `json:"type"` // "STATUS", "PRICE", "STATUS_AND_PRICE"
	OldStatus string  `json:"old_status,omitempty"`
	NewStatus string  `json:"new_status,omitempty"`
	OldCost   float64 `json:"old_cost,omitempty"`
	NewCost   float64 `json:"new_cost,omitempty"`
	OldPrice  float64 `json:"old_price,omitempty"`
	NewPrice  float64 `json:"new_price,omitempty"`
}

// SyncResult hasil eksekusi sinkronisasi katalog DFLASH
type SyncResult struct {
	TotalChecked  int             `json:"total_checked"`
	TotalUpdated  int             `json:"total_updated"`
	PriceUpdated  int             `json:"price_updated"`
	StatusUpdated int             `json:"status_updated"`
	GangguanCount int             `json:"gangguan_count"`
	ActiveCount   int             `json:"active_count"`
	SyncedAt      time.Time       `json:"synced_at"`
	ExecutionMs   int64           `json:"execution_ms"`
	Changes       []SKUChangeItem `json:"changes"`
	Message       string          `json:"message"`
}

// SyncLogEntry mewakili satu sesi log sinkronisasi (array changes di dalamnya)
type SyncLogEntry struct {
	ID            string          `json:"id"`
	JobKey        string          `json:"job_key"`
	Status        string          `json:"status"`
	TotalChecked  int             `json:"total_checked"`
	TotalUpdated  int             `json:"total_updated"`
	GangguanCount int             `json:"gangguan_count"`
	PriceUpdated  int             `json:"price_updated"`
	StatusUpdated int             `json:"status_updated"`
	ExecutionMs   int64           `json:"execution_ms"`
	Changes       []SKUChangeItem `json:"changes"`
	ErrorMessage  string          `json:"error_message,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// CronConfig konfigurasi interval cronjob DFLASH
type CronConfig struct {
	IntervalMinutes int        `json:"interval_minutes"`
	IsEnabled       bool       `json:"is_enabled"`
	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
	NextRunAt       *time.Time `json:"next_run_at,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// DFlashSyncer mengelola sinkronisasi realtime status & harga produk DFLASH
type DFlashSyncer struct {
	reportURL       string
	httpClient      *http.Client
	mu              sync.Mutex
	intervalMinutes int
	isEnabled       bool
	lastRunAt       *time.Time
	nextRunAt       *time.Time
	logs            []SyncLogEntry
	resetTickerCh   chan int
	lastResult      *SyncResult
}

// NewDFlashSyncer membuat instance baru DFlashSyncer
func NewDFlashSyncer(customReportURL string) *DFlashSyncer {
	url := strings.TrimSpace(customReportURL)
	if url == "" {
		url = DefaultDFlashReportURL
	}
	return &DFlashSyncer{
		reportURL:       url,
		intervalMinutes: 10,
		isEnabled:       true,
		httpClient: &http.Client{
			Timeout: 25 * time.Second,
		},
		logs:          make([]SyncLogEntry, 0, 30),
		resetTickerCh: make(chan int, 5),
	}
}

// LoadCronSettingsFromDB memuat konfigurasi interval tersimpan dari Supabase
func (s *DFlashSyncer) LoadCronSettingsFromDB(sbClient *supabase.Client) {
	if sbClient == nil {
		return
	}
	dbBytes, err := sbClient.GetDB("cron_settings", "key=eq.dflash_sync&select=interval_minutes,is_enabled,last_run_at,next_run_at", "")
	if err == nil && len(dbBytes) > 0 {
		var settings []struct {
			IntervalMinutes int        `json:"interval_minutes"`
			IsEnabled       bool       `json:"is_enabled"`
			LastRunAt       *time.Time `json:"last_run_at"`
			NextRunAt       *time.Time `json:"next_run_at"`
		}
		if err := json.Unmarshal(dbBytes, &settings); err == nil && len(settings) > 0 {
			s.mu.Lock()
			if settings[0].IntervalMinutes > 0 {
				s.intervalMinutes = settings[0].IntervalMinutes
			}
			s.isEnabled = settings[0].IsEnabled
			s.lastRunAt = settings[0].LastRunAt
			s.nextRunAt = settings[0].NextRunAt
			s.mu.Unlock()
			log.Printf("⚙️ [CRON-DFLASH] Pengaturan dimuat dari DB: interval=%d menit, enabled=%v", s.intervalMinutes, s.isEnabled)
		}
	}
}

// GetCronConfig mengembalikan konfigurasi interval cronjob saat ini
func (s *DFlashSyncer) GetCronConfig() CronConfig {
	s.mu.Lock()
	defer s.mu.Unlock()

	return CronConfig{
		IntervalMinutes: s.intervalMinutes,
		IsEnabled:       s.isEnabled,
		LastRunAt:       s.lastRunAt,
		NextRunAt:       s.nextRunAt,
		UpdatedAt:       time.Now(),
	}
}

// UpdateCronSettings memperbarui interval cronjob dan me-reset timer secara realtime
func (s *DFlashSyncer) UpdateCronSettings(sbClient *supabase.Client, intervalMinutes int, isEnabled bool) (*CronConfig, error) {
	if intervalMinutes < 1 {
		intervalMinutes = 1 // Minimal 1 menit
	}
	if intervalMinutes > 1440 {
		intervalMinutes = 1440 // Maksimal 24 jam
	}

	s.mu.Lock()
	s.intervalMinutes = intervalMinutes
	s.isEnabled = isEnabled
	nextRun := time.Now().Add(time.Duration(intervalMinutes) * time.Minute)
	s.nextRunAt = &nextRun
	s.mu.Unlock()

	// 1. Simpan ke tabel cron_settings di database
	if sbClient != nil {
		upsertPayload := map[string]interface{}{
			"key":              "dflash_sync",
			"interval_minutes": intervalMinutes,
			"is_enabled":       isEnabled,
			"next_run_at":      nextRun,
			"updated_at":       time.Now(),
		}
		_, err := sbClient.PostDB("cron_settings", upsertPayload, "resolution=merge-duplicates", "")
		if err != nil {
			log.Printf("⚠️ Gagal menyimpan cron_settings ke DB: %v", err)
		}
	}

	// 2. Beritahu background worker untuk mereset ticker
	select {
	case s.resetTickerCh <- intervalMinutes:
	default:
	}

	log.Printf("⏱️ [CRON-DFLASH] Interval berhasil diperbarui ke %d menit (Enabled: %v)", intervalMinutes, isEnabled)
	cfg := s.GetCronConfig()
	return &cfg, nil
}

// GetSyncLogs mengembalikan riwayat log sinkronisasi
func (s *DFlashSyncer) GetSyncLogs(sbClient *supabase.Client, limit int) []SyncLogEntry {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	// Coba ambil dari DB jika tersedia
	if sbClient != nil {
		query := fmt.Sprintf("job_key=eq.dflash_sync&order=created_at.desc&limit=%d", limit)
		dbBytes, err := sbClient.GetDB("cron_sync_logs", query, "")
		if err == nil && len(dbBytes) > 0 {
			var dbLogs []SyncLogEntry
			if err := json.Unmarshal(dbBytes, &dbLogs); err == nil && len(dbLogs) > 0 {
				return dbLogs
			}
		}
	}

	// Fallback ke in-memory logs
	s.mu.Lock()
	defer s.mu.Unlock()
	res := make([]SyncLogEntry, len(s.logs))
	copy(res, s.logs)
	return res
}

// FetchReport mengambil dan mem-parse seluruh data SKU dari laporan DFLASH
func (s *DFlashSyncer) FetchReport() (map[string]DFlashReportItem, error) {
	req, err := http.NewRequest("GET", s.reportURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat request HTTP ke report DFLASH: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi server report DFLASH: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server report DFLASH mengembalikan status HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca body response report DFLASH: %w", err)
	}
	html := string(body)

	trRegex := regexp.MustCompile(`(?s)<tr class="td[12]">(.*?)</tr>`)
	tdRegex := regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
	tagRegex := regexp.MustCompile(`<[^>]+>`)

	trMatches := trRegex.FindAllStringSubmatch(html, -1)
	if len(trMatches) == 0 {
		return nil, fmt.Errorf("tidak ditemukan baris data produk pada report DFLASH (format HTML berubah atau kosong)")
	}

	items := make(map[string]DFlashReportItem, len(trMatches))
	for _, m := range trMatches {
		if len(m) < 2 {
			continue
		}
		tdMatches := tdRegex.FindAllStringSubmatch(m[1], -1)
		cols := make([]string, 0, len(tdMatches))
		for _, td := range tdMatches {
			if len(td) >= 2 {
				clean := strings.TrimSpace(tagRegex.ReplaceAllString(td[1], ""))
				cols = append(cols, clean)
			}
		}

		if len(cols) >= 4 {
			sku := strings.ToUpper(strings.TrimSpace(cols[0]))
			if sku == "" {
				continue
			}
			name := strings.TrimSpace(cols[1])
			priceClean := strings.ReplaceAll(strings.TrimSpace(cols[2]), ".", "")
			priceClean = strings.ReplaceAll(priceClean, ",", ".")
			costPrice, _ := strconv.ParseFloat(priceClean, 64)
			statusStr := strings.TrimSpace(cols[3])

			items[sku] = DFlashReportItem{
				SKU:       sku,
				Name:      name,
				CostPrice: costPrice,
				Status:    statusStr,
			}
		}
	}

	return items, nil
}

// SyncCatalog melakukan sinkronisasi harga & status antara report DFLASH dan database DewaCell
func (s *DFlashSyncer) SyncCatalog(sbClient *supabase.Client, repo Repository) (*SyncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	startTime := time.Now()

	// 1. Fetch seluruh item dari report DFLASH
	dflashItems, err := s.FetchReport()
	if err != nil {
		// Log error run
		s.recordLog(sbClient, SyncLogEntry{
			JobKey:       "dflash_sync",
			Status:       "ERROR",
			ExecutionMs:  time.Since(startTime).Milliseconds(),
			ErrorMessage: err.Error(),
			Changes:      []SKUChangeItem{},
			CreatedAt:    time.Now(),
		})
		return nil, err
	}

	// 2. Ambil seluruh produk yang ada di database DewaCell
	var dbProducts []struct {
		ID           string  `json:"id"`
		VendorCode   string  `json:"vendor_code"`
		BuyerSKUCode string  `json:"buyer_sku_code"`
		Code         string  `json:"code"`
		Name         string  `json:"name"`
		CostPrice    float64 `json:"cost_price"`
		Price        float64 `json:"price"`
		Status       string  `json:"status"`
		IsActive     bool    `json:"is_active"`
	}

	if sbClient != nil {
		dbBytes, err := sbClient.GetDB("ppob_products", "select=id,vendor_code,buyer_sku_code,code,name,cost_price,price,status,is_active", "")
		if err == nil && len(dbBytes) > 0 {
			_ = json.Unmarshal(dbBytes, &dbProducts)
		}
	}

	result := &SyncResult{
		TotalChecked: len(dbProducts),
		SyncedAt:     startTime,
		Changes:      make([]SKUChangeItem, 0),
	}

	// 3. Bandingkan dan update produk yang terdaftar di DFLASH
	for _, p := range dbProducts {
		sku := strings.ToUpper(strings.TrimSpace(p.BuyerSKUCode))
		if sku == "" {
			sku = strings.ToUpper(strings.TrimSpace(p.Code))
		}

		dItem, exists := dflashItems[sku]
		if !exists && strings.Contains(sku, "-") {
			parts := strings.Split(sku, "-")
			dItem, exists = dflashItems[parts[len(parts)-1]]
		}
		if !exists && p.Code != "" {
			codeUpper := strings.ToUpper(strings.TrimSpace(p.Code))
			dItem, exists = dflashItems[codeUpper]
		}

		// Jika SKU tidak ditemukan dalam laporan DFLASH, lewati produk ini
		if !exists {
			continue
		}

		// Evaluasi perubahan status & harga
		statusChanged := false
		priceChanged := false
		vendorChanged := false
		if strings.ToUpper(strings.TrimSpace(p.VendorCode)) != "DFLASH" {
			vendorChanged = true
		}

		newStatus := strings.ToUpper(strings.TrimSpace(p.Status))
		newCostPrice := p.CostPrice
		newPrice := p.Price
		newIsActive := p.IsActive

		// Aturan Status:
		if strings.Contains(strings.ToLower(dItem.Status), "gangguan") {
			result.GangguanCount++
			if newStatus != "INACTIVE" && newStatus != "GANGGUAN" {
				newStatus = "GANGGUAN"
				newIsActive = true
				statusChanged = true
				result.StatusUpdated++
			}
		} else {
			result.ActiveCount++
			if newStatus == "GANGGUAN" {
				newStatus = "ACTIVE"
				newIsActive = true
				statusChanged = true
				result.StatusUpdated++
			}
		}

		// Aturan Harga:
		if dItem.CostPrice > 0 && dItem.CostPrice != p.CostPrice {
			currentMargin := p.Price - p.CostPrice
			if currentMargin < 0 {
				currentMargin = 200
			}
			newCostPrice = dItem.CostPrice
			newPrice = newCostPrice + currentMargin
			priceChanged = true
			result.PriceUpdated++
		}

		// Jika ada perubahan status, harga, atau vendor_code
		if statusChanged || priceChanged || vendorChanged {
			if statusChanged || priceChanged {
				changeType := "STATUS"
				if statusChanged && priceChanged {
					changeType = "STATUS_AND_PRICE"
				} else if priceChanged {
					changeType = "PRICE"
				}

				changeItem := SKUChangeItem{
					SKU:       sku,
					Name:      p.Name,
					Type:      changeType,
					OldStatus: p.Status,
					NewStatus: newStatus,
					OldCost:   p.CostPrice,
					NewCost:   newCostPrice,
					OldPrice:  p.Price,
					NewPrice:  newPrice,
				}
				result.Changes = append(result.Changes, changeItem)
			}

			if sbClient != nil {
				patchData := map[string]interface{}{
					"status":      newStatus,
					"is_active":   newIsActive,
					"cost_price":  newCostPrice,
					"price":       newPrice,
					"vendor_code": "DFLASH",
					"updated_at":  time.Now(),
				}

				filter := fmt.Sprintf("id=eq.%s", p.ID)
				_, patchErr := sbClient.PatchDB("ppob_products", filter, patchData, "")
				if patchErr == nil {
					result.TotalUpdated++
				} else {
					log.Printf("⚠️ Gagal memperbarui produk %s (%s) ke DB: %v", p.Code, p.Name, patchErr)
				}
			}
		}
	}

	// 4. Muat ulang cache in-memory
	if repo != nil && sbClient != nil {
		_, _ = repo.SyncFromSupabase(sbClient)
	}

	result.ExecutionMs = time.Since(startTime).Milliseconds()
	result.Message = fmt.Sprintf(
		"Sinkronisasi DFLASH selesai (%d ms). %d produk dicek, %d SKU diperbarui (%d Gangguan, %d harga).",
		result.ExecutionMs,
		result.TotalChecked,
		len(result.Changes),
		result.GangguanCount,
		result.PriceUpdated,
	)
	s.lastResult = result
	now := time.Now()
	s.lastRunAt = &now
	next := now.Add(time.Duration(s.intervalMinutes) * time.Minute)
	s.nextRunAt = &next

	// 5. Simpan log ringkas (array JSONB) ke database Supabase
	s.recordLog(sbClient, SyncLogEntry{
		JobKey:        "dflash_sync",
		Status:        "SUCCESS",
		TotalChecked:  result.TotalChecked,
		TotalUpdated:  len(result.Changes),
		GangguanCount: result.GangguanCount,
		PriceUpdated:  result.PriceUpdated,
		StatusUpdated: result.StatusUpdated,
		ExecutionMs:   result.ExecutionMs,
		Changes:       result.Changes,
		CreatedAt:     now,
	})

	log.Printf("🔄 [DFLASH-SYNC] %s", result.Message)
	return result, nil
}

// recordLog menyimpan histori log dengan array perubahan ke Supabase & ring buffer memori
func (s *DFlashSyncer) recordLog(sbClient *supabase.Client, entry SyncLogEntry) {
	// 1. Simpan ke buffer memori
	if len(s.logs) >= 30 {
		s.logs = s.logs[1:]
	}
	s.logs = append(s.logs, entry)

	// 2. Simpan 1 baris ke tabel cron_sync_logs (kolom changes adalah array JSONB)
	if sbClient != nil {
		payload := map[string]interface{}{
			"job_key":        entry.JobKey,
			"status":         entry.Status,
			"total_checked":  entry.TotalChecked,
			"total_updated":  entry.TotalUpdated,
			"gangguan_count": entry.GangguanCount,
			"price_updated":  entry.PriceUpdated,
			"status_updated": entry.StatusUpdated,
			"execution_ms":   entry.ExecutionMs,
			"changes":        entry.Changes,
			"error_message":  entry.ErrorMessage,
			"created_at":     entry.CreatedAt,
		}

		_, err := sbClient.PostDB("cron_sync_logs", payload, "", "")
		if err != nil {
			log.Printf("⚠️ Gagal menyimpan log cron_sync_logs: %v", err)
		}

		// Update juga last_run_at di cron_settings
		settingsPatch := map[string]interface{}{
			"last_run_at": entry.CreatedAt,
			"next_run_at": s.nextRunAt,
			"updated_at":  time.Now(),
		}
		_, _ = sbClient.PatchDB("cron_settings", "key=eq.dflash_sync", settingsPatch, "")
	}
}

// StartAutoCronJob menjalankan ticker background terjadwal untuk auto update harga & status
func (s *DFlashSyncer) StartAutoCronJob(ctx context.Context, sbClient *supabase.Client, repo Repository, initialInterval time.Duration) {
	// Muat interval tersimpan dari DB
	s.LoadCronSettingsFromDB(sbClient)

	interval := time.Duration(s.intervalMinutes) * time.Minute
	if interval <= 0 {
		interval = 10 * time.Minute
	}

	go func() {
		log.Printf("⏱️ [CRON-DFLASH] Background Auto-Cronjob DFLASH aktif (interval: %v)", interval)

		// Sinkronisasi awal 5 detik setelah server menyala
		time.Sleep(5 * time.Second)
		if _, err := s.SyncCatalog(sbClient, repo); err != nil {
			log.Printf("⚠️ [CRON-DFLASH] Error sinkronisasi awal: %v", err)
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("🛑 [CRON-DFLASH] Auto-cronjob DFLASH dihentikan.")
				return

			case newIntervalMinutes := <-s.resetTickerCh:
				ticker.Stop()
				newDuration := time.Duration(newIntervalMinutes) * time.Minute
				ticker = time.NewTicker(newDuration)
				log.Printf("🔄 [CRON-DFLASH] Ticker berhasil di-reset dengan interval baru: %v", newDuration)

			case <-ticker.C:
				if s.isEnabled {
					if _, err := s.SyncCatalog(sbClient, repo); err != nil {
						log.Printf("⚠️ [CRON-DFLASH] Gagal sinkronisasi otomatis: %v", err)
					}
				}
			}
		}
	}()
}
