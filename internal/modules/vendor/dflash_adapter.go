package vendor

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// DefaultDFlashCenters daftar IP & host center resmi DFLASH dengan urutan prioritas failover
var DefaultDFlashCenters = []string{
	"http://45.76.183.72:666",   // [Utama]
	"http://108.136.97.161:666", // [Cadangan 1]
	"http://45.76.183.72:8883",  // [Cadangan 2]
	"http://108.136.97.161:8883",// [Cadangan 3]
	"http://api.dflash.co.id:666", // [Domain 1]
	"http://api2.dflash.co.id:666",// [Domain 2]
}

// DFlashAdapter implementasi adapter OOP untuk server vendor DFLASH (OtomaX Engine)
// Mendukung Multi-Center High Availability dengan Automatic Failover
type DFlashAdapter struct {
	memberID string
	pin      string
	password string
	centers  []string
}

// NewDFlashAdapter membuat instance DFlashAdapter dengan multi-center failover
func NewDFlashAdapter(memberID, pin, password, baseURL string) *DFlashAdapter {
	if memberID == "" {
		memberID = os.Getenv("DFLASH_MEMBER_ID")
	}
	if pin == "" {
		pin = os.Getenv("DFLASH_PIN")
	}
	if password == "" {
		password = os.Getenv("DFLASH_PASSWORD")
	}

	// Kredensial resmi akun DFLASH jika env kosong
	if memberID == "" {
		memberID = "H2H02494"
	}
	if pin == "" {
		pin = "112211"
	}
	if password == "" {
		password = "DUNhil00@#"
	}

	var centers []string
	if baseURL != "" {
		centers = []string{strings.TrimRight(strings.TrimSpace(baseURL), "/")}
	} else if envCenters := os.Getenv("DFLASH_CENTERS"); envCenters != "" {
		for _, c := range strings.Split(envCenters, ",") {
			c = strings.TrimRight(strings.TrimSpace(c), "/")
			if c != "" {
				centers = append(centers, c)
			}
		}
	} else if envBase := os.Getenv("DFLASH_BASE_URL"); envBase != "" {
		centers = []string{strings.TrimRight(strings.TrimSpace(envBase), "/")}
	}

	// Gunakan 6 default centers DFLASH jika tidak ada override khusus
	if len(centers) == 0 {
		centers = make([]string, len(DefaultDFlashCenters))
		copy(centers, DefaultDFlashCenters)
	}

	return &DFlashAdapter{
		memberID: strings.TrimSpace(memberID),
		pin:      strings.TrimSpace(pin),
		password: strings.TrimSpace(password),
		centers:  centers,
	}
}

// GetCenters mengembalikan daftar IP Center yang aktif
func (d *DFlashAdapter) GetCenters() []string {
	return d.centers
}

// SetCenters mengatur ulang daftar IP Center
func (d *DFlashAdapter) SetCenters(centers []string) {
	if len(centers) > 0 {
		d.centers = centers
	}
}

// GetVendorCode mengembalikan kode unik vendor DFLASH
func (d *DFlashAdapter) GetVendorCode() string {
	return "DFLASH"
}

// ValidateSKU memastikan format SKU cocok dengan ketentuan DFLASH (OtomaX)
func (d *DFlashAdapter) ValidateSKU(sku string) bool {
	sku = strings.TrimSpace(sku)
	return len(sku) >= 3 && !strings.Contains(sku, " ")
}

// GenerateSignature menghasilkan signature sesuai rumus resmi DFLASH (OtomaX):
// $template = base64_encode(sha1('OtomaX|' + memberId + '|' + product + '|' + dest + '|' + refID + '|' + pin + '|' + password, true));
// $sign = str_replace('/', '_', str_replace('+', '-', rtrim($template, '=')));
func (d *DFlashAdapter) GenerateSignature(product, dest, refID string) string {
	raw := fmt.Sprintf("OtomaX|%s|%s|%s|%s|%s|%s",
		d.memberID,
		strings.TrimSpace(product),
		strings.TrimSpace(dest),
		strings.TrimSpace(refID),
		d.pin,
		d.password,
	)

	hasher := sha1.New()
	hasher.Write([]byte(raw))
	binaryHash := hasher.Sum(nil)

	// URL-Safe Base64 unpadded (mengganti '+' dengan '-', '/' dengan '_', dan menghapus '=')
	return base64.RawURLEncoding.EncodeToString(binaryHash)
}

// GenerateCheckSignature menghasilkan signature untuk cek saldo atau cek transaksi (parameter yang tidak ada dikosongkan)
// Contoh template cek saldo: OtomaX|DS0000||||8715|yunw)uc&@
func (d *DFlashAdapter) GenerateCheckSignature(refID string) string {
	return d.GenerateSignature("", "", refID)
}

// doRequestWithFailover mencoba mengirimkan request HTTP ke daftar IP center secara berurutan.
// Jika salah satu IP down/gangguan/timeout, sistem otomatis failover ke IP cadangan berikutnya.
func (d *DFlashAdapter) doRequestWithFailover(ctx context.Context, endpointWithQuery string) ([]byte, string, error) {
	if len(d.centers) == 0 {
		return nil, "", fmt.Errorf("tidak ada IP center DFLASH yang terkonfigurasi")
	}

	var lastErr error
	client := &http.Client{Timeout: 6 * time.Second}

	for idx, center := range d.centers {
		fullURL := center + endpointWithQuery
		httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
		if err != nil {
			lastErr = err
			continue
		}

		resp, err := client.Do(httpReq)
		if err != nil {
			log.Printf("[DFLASH FAILOVER] Center #%d (%s) tidak merespons: %v. Beralih ke center cadangan...", idx+1, center, err)
			lastErr = fmt.Errorf("center %s: %w", center, err)
			continue
		}

		// Jika server memberikan status 502/503/504, coba center berikutnya
		if resp.StatusCode >= 502 && resp.StatusCode <= 504 {
			resp.Body.Close()
			log.Printf("[DFLASH FAILOVER] Center #%d (%s) mengembalikan status HTTP %d. Beralih ke center cadangan...", idx+1, center, resp.StatusCode)
			lastErr = fmt.Errorf("center %s returned HTTP %d", center, resp.StatusCode)
			continue
		}

		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}

		log.Printf("[DFLASH] Berhasil terhubung ke Center: %s", center)
		return bodyBytes, center, nil
	}

	return nil, "", fmt.Errorf("seluruh %d IP Center DFLASH tidak dapat dihubungi. Terakhir: %v", len(d.centers), lastErr)
}

// OrderProduct memproses transaksi pengisian produk PPOB ke DFLASH dengan automatic failover
func (d *DFlashAdapter) OrderProduct(ctx context.Context, req *VendorOrderRequest) (*VendorOrderResponse, error) {
	if !d.ValidateSKU(req.BuyerSKUCode) {
		return nil, fmt.Errorf("kode produk/SKU '%s' tidak valid untuk server vendor DFLASH", req.BuyerSKUCode)
	}

	// 1. Generate signature resmi DFlash
	sign := d.GenerateSignature(req.BuyerSKUCode, req.CustomerNo, req.RefID)

	targetEndpoint := fmt.Sprintf("/trx?memberID=%s&product=%s&dest=%s&refID=%s&sign=%s",
		d.memberID, req.BuyerSKUCode, req.CustomerNo, req.RefID, sign)

	if d.memberID == "" {
		return nil, fmt.Errorf("kredensial vendor DFLASH belum dikonfigurasi (memberID kosong)")
	}

	// 3. Eksekusi request HTTP ke API DFLASH dengan failover otomatis
	bodyBytes, usedCenter, err := d.doRequestWithFailover(ctx, targetEndpoint)
	if err != nil {
		return nil, fmt.Errorf("gagal memproses transaksi ke server DFlash: %v", err)
	}

	status, sn, msg, rawResp := d.parseDFlashResponse(bodyBytes, usedCenter, req.RefID, req.BuyerSKUCode, req.CustomerNo)

	vendorTrxID := cleanStringVal(rawResp["id"])
	if vendorTrxID == "" {
		vendorTrxID = cleanStringVal(rawResp["trx_id"])
	}
	if vendorTrxID == "" {
		vendorTrxID = fmt.Sprintf("DFLASH-%s", req.RefID)
	}

	return &VendorOrderResponse{
		VendorCode:   "DFLASH",
		VendorTrxID:  vendorTrxID,
		RefID:        req.RefID,
		BuyerSKUCode: req.BuyerSKUCode,
		CustomerNo:   req.CustomerNo,
		Price:        req.MaxPrice,
		Status:       status,
		SerialNumber: sn,
		Message:      fmt.Sprintf("[%s] %s", usedCenter, msg),
		RawResponse:  rawResp,
		CreatedAt:    time.Now(),
	}, nil
}

// parseDFlashResponse memproses seluruh format respons dari DFLASH/OtomaX (JSON, XML, maupun Plain Text)
func (d *DFlashAdapter) parseDFlashResponse(bodyBytes []byte, usedCenter, refID, buyerSKU, dest string) (status string, sn string, msg string, rawResp map[string]interface{}) {
	rawStr := strings.TrimSpace(string(bodyBytes))
	rawResp = make(map[string]interface{})

	// 1. Coba decode JSON jika server mengembalikan JSON
	if err := json.Unmarshal(bodyBytes, &rawResp); err == nil && len(rawResp) > 0 {
		rawStatus := cleanStringVal(rawResp["status"])
		rawStatusText := cleanStringVal(rawResp["status_text"])
		if rawStatusText == "" {
			rawStatusText = cleanStringVal(rawResp["message"])
		}
		if rawStatusText == "" {
			rawStatusText = cleanStringVal(rawResp["keterangan"])
		}
		sn = cleanStringVal(rawResp["sn"])
		status = d.normalizeStatusCode(rawStatus, rawStatusText)
		msg = rawStatusText
		if msg == "" {
			msg = rawStatus
		}
		return
	}

	// 2. Format Plain Text Resmi DFLASH / OtomaX
	rawResp["raw_text"] = rawStr
	upper := strings.ToUpper(rawStr)
	msg = rawStr

	// Indikator GAGAL / PENOLAKAN DARI VENDOR DFLASH
	isFailed := strings.Contains(upper, "TDK DIPROSES") ||
		strings.Contains(upper, "TIDAK DIPROSES") ||
		strings.Contains(upper, "TDK BISA DIPROSES") ||
		strings.Contains(upper, "STOK TIDAK CUKUP") ||
		strings.Contains(upper, "SALDO TIDAK CUKUP") ||
		strings.Contains(upper, "STOK KOSONG") ||
		strings.Contains(upper, "GAGAL") ||
		strings.Contains(upper, "FAILED") ||
		strings.Contains(upper, "GANGGUAN") ||
		strings.Contains(upper, "INVALID") ||
		strings.Contains(upper, "SALAH") ||
		strings.Contains(upper, "KOSONG") ||
		strings.Contains(upper, "TIDAK ADA STOK") ||
		strings.Contains(upper, "BELUM TERDAFTAR") ||
		strings.Contains(upper, "TIDAK TERDAFTAR") ||
		strings.Contains(upper, "SIGNATURE DOES NOT MACTH") ||
		strings.Contains(upper, "SIGNATURE") ||
		strings.Contains(upper, "WHITELIST") ||
		strings.Contains(upper, "CUT OFF") ||
		strings.Contains(upper, "CLOSED") ||
		strings.Contains(upper, "ERROR")

	if isFailed {
		status = "FAILED"
		if strings.Contains(upper, "STOK TIDAK CUKUP") || strings.Contains(upper, "SALDO TIDAK CUKUP") {
			msg = "Stok / Saldo agen di server DFLASH tidak mencukupi"
		} else if strings.Contains(upper, "SIGNATURE") {
			msg = "Signature otentikasi DFLASH ditolak server vendor"
		} else if strings.Contains(upper, "BELUM TERDAFTAR") || strings.Contains(upper, "WHITELIST") {
			msg = "IP server belum terdaftar di whitelist DFLASH"
		} else if strings.Contains(upper, "TDK DIPROSES") {
			msg = "Transaksi tidak diproses oleh server DFLASH"
		}
		return
	}

	// Indikator SUKSES
	isSuccess := (strings.Contains(upper, "SUKSES") || strings.Contains(upper, "SUCCESS") || strings.Contains(upper, "BERHASIL")) &&
		!strings.Contains(upper, "TDK") && !strings.Contains(upper, "TIDAK")

	if isSuccess {
		status = "SUCCESS"
		sn = extractSNFromText(rawStr)
		return
	}

	// Indikator PENDING / ANTRIAN
	isPending := strings.Contains(upper, "DIPROSES") ||
		strings.Contains(upper, "ANTRIAN") ||
		strings.Contains(upper, "MENUNGGU") ||
		strings.Contains(upper, "PENDING")

	if isPending {
		status = "PENDING"
		return
	}

	// Default jika respons vendor di luar dugaan: anggap FAILED agar aman bagi saldo user
	status = "FAILED"
	msg = fmt.Sprintf("Respons server DFLASH ditolak: %s", rawStr)
	return
}

func extractSNFromText(text string) string {
	patterns := []string{
		`(?i)(?:SN|TOKEN|STRUK)[:\s/]+([A-Za-z0-9\-/\.]+)`,
		`(?i)VOUCHER[:\s/]+([A-Za-z0-9\-/\.]+)`,
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		matches := re.FindStringSubmatch(text)
		if len(matches) > 1 {
			sn := strings.TrimSpace(matches[1])
			return strings.TrimRight(sn, ".,; ")
		}
	}
	return ""
}

// normalizeStatusCode memetakan kode status numerik & teks resmi OtomaX / DFLASH
func (d *DFlashAdapter) normalizeStatusCode(rawStatus, rawStatusText string) string {
	statusStr := strings.TrimSpace(rawStatus)
	switch statusStr {
	case "20":
		return "SUCCESS"
	case "0", "1", "2", "22":
		return "PENDING"
	case "40", "42", "43", "44", "45", "47", "50", "51", "52", "53", "55", "56", "99":
		return "FAILED"
	}

	upper := strings.ToUpper(rawStatusText)
	if upper == "" {
		upper = strings.ToUpper(statusStr)
	}
	if strings.Contains(upper, "SUKSES") || strings.Contains(upper, "SUCCESS") || upper == "00" {
		return "SUCCESS"
	}
	if strings.Contains(upper, "GAGAL") || strings.Contains(upper, "FAILED") ||
		strings.Contains(upper, "SALDO TIDAK CUKUP") || strings.Contains(upper, "GANGGUAN") ||
		strings.Contains(upper, "KOSONG") || strings.Contains(upper, "NO DATA") ||
		strings.Contains(upper, "INVALID") || strings.Contains(upper, "SALAH") ||
		strings.Contains(upper, "TIDAK TERDAFTAR") || strings.Contains(upper, "BELUM TERDAFTAR") ||
		strings.Contains(upper, "WHITELIST") || strings.Contains(upper, "ERROR") ||
		strings.Contains(upper, "TDK DIPROSES") {
		return "FAILED"
	}
	return "PENDING"
}

// CheckStatus mengecek status transaksi ke DFLASH berdasarkan refID internal
// Format resmi: http://[ip_dflash]/check?memberID=[memberid]&refID=[refid]&sign=[sign]
// Template signature check refID: OtomaX|memberID|||refID|pin|password
func (d *DFlashAdapter) CheckStatus(ctx context.Context, refID string) (*VendorOrderResponse, error) {
	sign := d.GenerateCheckSignature(refID)
	checkEndpoint := fmt.Sprintf("/check?memberID=%s&refID=%s&sign=%s", d.memberID, refID, sign)

	if d.memberID == "" {
		return nil, fmt.Errorf("kredensial vendor DFLASH belum dikonfigurasi (memberID kosong)")
	}

	bodyBytes, usedCenter, err := d.doRequestWithFailover(ctx, checkEndpoint)
	if err != nil {
		return nil, fmt.Errorf("gagal mengecek status transaksi ke server DFlash: %v", err)
	}

	status, sn, msg, rawResp := d.parseDFlashResponse(bodyBytes, usedCenter, refID, "", "")

	vendorTrxID := cleanStringVal(rawResp["id"])
	if vendorTrxID == "" {
		vendorTrxID = cleanStringVal(rawResp["trx_id"])
	}
	if vendorTrxID == "" {
		vendorTrxID = fmt.Sprintf("DFLASH-%s", refID)
	}

	return &VendorOrderResponse{
		VendorCode:   "DFLASH",
		VendorTrxID:  vendorTrxID,
		RefID:        refID,
		Status:       status,
		SerialNumber: sn,
		Message:      fmt.Sprintf("[%s] %s", usedCenter, msg),
		RawResponse:  rawResp,
		CreatedAt:    time.Now(),
	}, nil
}

// CheckBalance mengecek saldo akun agen di server DFLASH dengan automatic failover
// Format resmi: http://[ip_dflash]/balance?memberID=[memberid]&sign=[sign]
// Template signature saldo: OtomaX|memberID||||pin|password
func (d *DFlashAdapter) CheckBalance(ctx context.Context) (map[string]interface{}, error) {
	if d.memberID == "" {
		return nil, fmt.Errorf("kredensial vendor DFLASH belum dikonfigurasi (memberID kosong)")
	}

	sign := d.GenerateCheckSignature("")
	balanceEndpoint := fmt.Sprintf("/balance?memberID=%s&sign=%s", d.memberID, sign)

	bodyBytes, usedCenter, err := d.doRequestWithFailover(ctx, balanceEndpoint)
	if err != nil {
		return nil, fmt.Errorf("gagal mengecek saldo ke server DFlash: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return map[string]interface{}{
			"vendor_code":   "DFLASH",
			"active_center": usedCenter,
			"raw_text":      string(bodyBytes),
			"checked_at":    time.Now().Format(time.RFC3339),
		}, nil
	}

	result["active_center"] = usedCenter
	return result, nil
}

// ParseCallback membedah webhook callback dari DFLASH
// Mendukung format JSON resmi DFLASH: refid, status (20, 40-56), status_text, sn, message, keterangan
func (d *DFlashAdapter) ParseCallback(payload []byte, signature string) (*VendorCallbackPayload, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("gagal membaca format JSON callback DFLASH: %v", err)
	}

	refID := cleanStringVal(raw["ref_id"])
	if refID == "" {
		refID = cleanStringVal(raw["refid"])
	}

	trxID := cleanStringVal(raw["trx_id"])
	if trxID == "" {
		trxID = cleanStringVal(raw["id"])
	}
	if trxID == "" {
		trxID = refID
	}

	rawStatus := cleanStringVal(raw["status"])
	rawStatusText := cleanStringVal(raw["status_text"])
	if rawStatusText == "" {
		rawStatusText = cleanStringVal(raw["message"])
	}

	status := d.normalizeStatusCode(rawStatus, rawStatusText)

	sn := cleanStringVal(raw["sn"])

	message := cleanStringVal(raw["message"])
	if message == "" {
		message = cleanStringVal(raw["keterangan"])
		if message == "" {
			message = rawStatusText
		}
	}

	return &VendorCallbackPayload{
		VendorCode:   "DFLASH",
		VendorTrxID:  trxID,
		RefID:        refID,
		Status:       status,
		SerialNumber: sn,
		Message:      message,
		RawPayload:   raw,
		ReceivedAt:   time.Now(),
	}, nil
}

func cleanStringVal(val interface{}) string {
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
