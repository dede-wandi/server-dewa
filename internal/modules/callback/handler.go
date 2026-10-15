package callback

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dede-wandi/server-dewa/internal/modules/vendor"
	"github.com/dede-wandi/server-dewa/internal/supabase"
	"github.com/go-chi/chi/v5"
)

// Handler handles universal webhook callbacks from any PPOB vendor and payment gateway
type Handler struct {
	sbClient  *supabase.Client
	vendorMgr *vendor.VendorManager
}

// NewHandler constructs a new callback handler
func NewHandler(sbClient *supabase.Client) *Handler {
	return &Handler{
		sbClient:  sbClient,
		vendorMgr: vendor.GetGlobalManager(),
	}
}

// Routes mounts all global callback endpoints
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// 1. DOKUMENTASI WEBHOOK
	r.Get("/info", h.CallbackInfo)

	// 2. UNIVERSAL CALLBACK ENDPOINTS (Mendukung GET & POST)
	r.HandleFunc("/", h.HandleUniversalCallback)
	r.HandleFunc("/webhook", h.HandleUniversalCallback)
	r.HandleFunc("/notification", h.HandleUniversalCallback)
	r.HandleFunc("/callback-product", h.HandleUniversalCallback)
	r.HandleFunc("/payment/callback-product", h.HandleUniversalCallback)

	// 3. Backward Compatibility endpoints
	r.HandleFunc("/vendor", h.HandleUniversalCallback)
	r.HandleFunc("/vendor/{provider}", h.HandleUniversalCallback)
	r.HandleFunc("/payment", h.HandleUniversalCallback)
	r.HandleFunc("/payment/{provider}", h.HandleUniversalCallback)
	r.HandleFunc("/dflash", h.HandleUniversalCallback)

	return r
}

// ProductServiceRoutes mounts enterprise microservice style routes (e.g. /product-service/v2.0/payment/callback-product)
func (h *Handler) ProductServiceRoutes() chi.Router {
	r := chi.NewRouter()
	r.HandleFunc("/callback-product", h.HandleUniversalCallback)
	r.HandleFunc("/callback", h.HandleUniversalCallback)
	r.HandleFunc("/webhook", h.HandleUniversalCallback)
	r.HandleFunc("/", h.HandleUniversalCallback)
	return r
}

// CallbackInfo returns documentation of the global callback URL
func (h *Handler) CallbackInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	scheme := "https"
	if r.TLS == nil && !strings.Contains(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "http"
	}
	host := r.Host
	if host == "" {
		host = "43.173.26.120"
	}

	baseURL := fmt.Sprintf("%s://%s", scheme, host)
	primaryURL := fmt.Sprintf("%s/api/v1/callback", baseURL)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":            "success",
		"service":           "DEWACELL Global Callback Engine",
		"callback_url":      primaryURL,
		"supported_methods": []string{"POST (JSON & Form-URLencoded)", "GET (Query String)"},
		"note":              "Gunakan 'callback_url' untuk didaftarkan ke semua vendor PPOB dan Payment Gateway.",
	})
}

// HandleUniversalCallback handles all callbacks without needing vendor names in the URL
func (h *Handler) HandleUniversalCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 1. Tangani verifikasi Whitelist / Ping Uptime dari vendor (GET tanpa data transaksi)
	isPing := false
	if r.Method == http.MethodGet && len(r.URL.Query()) == 0 {
		isPing = true
	} else if r.Method == http.MethodGet {
		refCheck := r.URL.Query().Get("ref_id")
		if refCheck == "" {
			refCheck = r.URL.Query().Get("refid")
		}
		if refCheck == "" && r.URL.Query().Get("id") == "" && r.URL.Query().Get("status") == "" {
			isPing = true
		}
	}

	if isPing {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"status":    "UP",
			"service":   "DEWACELL Global Webhook & Callback Engine",
			"message":   "Endpoint callback aktif, terverifikasi dan siap untuk whitelist semua vendor",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// 2. Ekstraksi Data (dari Query String GET atau Body POST)
	dataMap := make(map[string]interface{})

	if r.Method == http.MethodGet || (r.Method == http.MethodPost && len(r.URL.Query()) > 0 && r.ContentLength <= 0) {
		for k, v := range r.URL.Query() {
			if len(v) > 0 {
				dataMap[k] = v[0]
			}
		}
	} else {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Gagal membaca body webhook: " + err.Error(),
			})
			return
		}

		if len(bodyBytes) > 0 {
			// Cek apakah JSON
			if bodyBytes[0] == '{' || bodyBytes[0] == '[' {
				_ = json.Unmarshal(bodyBytes, &dataMap)
				// Jika nested data (misal {"data": {...}})
				if nested, ok := dataMap["data"].(map[string]interface{}); ok {
					for k, v := range nested {
						dataMap[k] = v
					}
				}
			} else {
				// Form url-encoded
				_ = r.ParseForm()
				for k, v := range r.PostForm {
					if len(v) > 0 {
						dataMap[k] = v[0]
					}
				}
			}
		}
	}

	log.Printf("[GLOBAL CALLBACK] Received payload: %+v", dataMap)

	// 3. Normalisasi field transaksi (RefID, Status, Serial Number)
	refID := extractString(dataMap, "ref_id", "refid", "merchant_ref", "reference", "id", "order_id", "trx_id")
	statusRaw := extractString(dataMap, "status", "rc", "code", "result", "state")
	statusText := extractString(dataMap, "status_text", "statusText")
	sn := extractString(dataMap, "sn", "serial_number", "token", "voucher", "note")
	message := extractString(dataMap, "message", "keterangan", "desc", "msg")

	status := "PENDING"
	statusUpper := strings.ToUpper(statusRaw)
	statusTextUpper := strings.ToUpper(statusText)

	// Pemetaan Kode Status DFLASH / OtomaX & Universal Webhook:
	// 20 = Sukses (transaksi selesai)
	// 0, 1, 2, 22 = Pending / Sedang diproses / Menunggu jawaban / Masuk antrian
	// 40, 42, 43, 44, 45, 47, 50, 51, 52, 53, 55, 56, 99 = Gagal / Saldo tidak cukup / Stok kosong / dsb.
	if statusUpper == "20" || statusUpper == "00" ||
		strings.Contains(statusUpper, "SUKSES") || strings.Contains(statusUpper, "SUCCESS") ||
		strings.Contains(statusTextUpper, "SUKSES") || strings.Contains(statusTextUpper, "SUCCESS") ||
		statusUpper == "PAID" || statusUpper == "BERHASIL" {
		status = "SUCCESS"
	} else if statusUpper == "40" || statusUpper == "42" || statusUpper == "43" || statusUpper == "44" ||
		statusUpper == "45" || statusUpper == "47" || statusUpper == "50" || statusUpper == "51" ||
		statusUpper == "52" || statusUpper == "53" || statusUpper == "55" || statusUpper == "56" || statusUpper == "99" ||
		strings.Contains(statusUpper, "GAGAL") || strings.Contains(statusUpper, "FAILED") ||
		strings.Contains(statusTextUpper, "GAGAL") || strings.Contains(statusTextUpper, "TIDAK CUKUP") ||
		strings.Contains(statusTextUpper, "GANGGUAN") || strings.Contains(statusTextUpper, "KOSONG") ||
		statusUpper == "EXPIRED" || statusUpper == "BATAL" {
		status = "FAILED"
	}

	// 4. Update transaksi di database Supabase jika RefID ditemukan
	if h.sbClient != nil && refID != "" {
		updateData := map[string]interface{}{
			"status":        strings.ToLower(status),
			"serial_number": sn,
			"vendor_status": status,
			"updated_at":    time.Now(),
		}

		// Update berdasarkan reference_id
		filter := fmt.Sprintf("reference_id=eq.%s", refID)
		_, err := h.sbClient.PatchDB("transactions", filter, updateData, "")
		if err != nil {
			log.Printf("[GLOBAL CALLBACK] Note updating transactions by reference_id: %v", err)
			// Fallback update berdasarkan order_code
			filterOrder := fmt.Sprintf("order_code=eq.%s", refID)
			_, _ = h.sbClient.PatchDB("transactions", filterOrder, updateData, "")
		} else {
			log.Printf("[GLOBAL CALLBACK] Transaction %s updated to status %s (SN: %s, Msg: %s)", refID, status, sn, message)
		}
	} else if refID != "" {
		log.Printf("[GLOBAL CALLBACK] RefID %s status=%s (SN: %s, Msg: %s)", refID, status, sn, message)
	}

	// 5. Standar Response HTTP 200 OK Universal untuk semua vendor
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"status":   "acknowledged",
		"ref_id":   refID,
		"received": time.Now().Format(time.RFC3339),
		"message":  "Callback processed and acknowledged",
	})
}

// Helper untuk mencari value dari beberapa kemungkinan nama field
func extractString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if val, exists := m[k]; exists && val != nil {
			str := stringifyValue(val)
			if str != "" && str != "<nil>" {
				return str
			}
		}
	}
	return ""
}

func stringifyValue(val interface{}) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%.0f", v)
		}
		return fmt.Sprintf("%f", v)
	case float32:
		if v == float32(int64(v)) {
			return fmt.Sprintf("%.0f", float64(v))
		}
		return fmt.Sprintf("%f", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	default:
		s := strings.TrimSpace(fmt.Sprintf("%v", v))
		if s == "<nil>" {
			return ""
		}
		return s
	}
}
