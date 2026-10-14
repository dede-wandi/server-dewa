package wallet

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/dede-wandi/server-dewa/internal/supabase"
)

// WalletService defines domain business logic for wallets, payments, deposits, and refunds
type WalletService interface {
	GetBalance(userID, email, token string) (*Wallet, error)
	GetHistory(userID string) []*WalletTransaction
	Pay(userID, email, token string, req PayRequest) (*WalletTransaction, *Wallet, error)
	TopUp(userID, email, token string, req TopUpRequest) (*WalletTransaction, *Wallet, error)
	Refund(caller *supabase.User, token string, req RefundRequest) (*WalletTransaction, *Wallet, error)
	AdjustBalance(adminUser *supabase.User, token string, req AdjustRequest) (*WalletTransaction, *Wallet, error)

	GetAllWallets() []*Wallet
	GetUserWallet(userID string) (*Wallet, []*WalletTransaction, error)
	SetUserLock(userID string, locked bool) (*Wallet, error)
	GetAllTransactions() []*WalletTransaction
	GetStats() map[string]interface{}
}

type walletService struct {
	repo         Repository
	pointService PointService
	sbClient     *supabase.Client
}

// NewWalletService creates a new instance of WalletService
func NewWalletService(repo Repository, pointService PointService, sbClient *supabase.Client) WalletService {
	return &walletService{
		repo:         repo,
		pointService: pointService,
		sbClient:     sbClient,
	}
}

func (s *walletService) GetBalance(userID, email, token string) (*Wallet, error) {
	// 1. Try Supabase RPC get_user_wallet_info
	if s.sbClient != nil {
		rpcPayload := map[string]interface{}{
			"p_user_id": userID,
		}
		if respBytes, err := s.sbClient.CallRPC("get_user_wallet_info", rpcPayload, token); err == nil {
			var rpcRes map[string]interface{}
			if err := json.Unmarshal(respBytes, &rpcRes); err == nil && rpcRes["user_id"] != nil {
				wal := s.repo.GetOrCreateWallet(userID, email)
				if bal, ok := rpcRes["balance"].(float64); ok {
					wal.Balance = bal
				}
				if pts, ok := rpcRes["points"].(float64); ok {
					wal.Points = int64(pts)
				}
				if locked, ok := rpcRes["is_locked"].(bool); ok {
					wal.IsLocked = locked
				}
				s.repo.SaveWallet(wal)
				return wal, nil
			}
		}
	}

	// 2. Realtime runtime store (clean zero-default)
	wal := s.repo.GetOrCreateWallet(userID, email)
	return wal, nil
}

func (s *walletService) GetHistory(userID string) []*WalletTransaction {
	// 1. Try Supabase RPC get_user_wallet_transactions
	if s.sbClient != nil {
		rpcPayload := map[string]interface{}{
			"p_user_id": userID,
			"p_limit":   50,
		}
		if respBytes, err := s.sbClient.CallRPC("get_user_wallet_transactions", rpcPayload, ""); err == nil {
			var dbTrxs []*WalletTransaction
			if err := json.Unmarshal(respBytes, &dbTrxs); err == nil && len(dbTrxs) > 0 {
				return dbTrxs
			}
		}
	}

	return s.repo.GetTransactionsByUser(userID)
}

func (s *walletService) Pay(userID, email, token string, req PayRequest) (*WalletTransaction, *Wallet, error) {
	if req.Amount <= 0 {
		return nil, nil, errors.New("Nominal pembayaran harus lebih besar dari 0")
	}
	if req.Title == "" {
		req.Title = "Pembayaran Transaksi PPOB DewaCell"
	}
	if req.Category == "" {
		req.Category = CategoryPayment
	}

	// 1. Try Supabase RPC
	if s.sbClient != nil && token != "" {
		rpcPayload := map[string]interface{}{
			"p_user_id":      userID,
			"p_type":         TypeDebit,
			"p_category":     req.Category,
			"p_amount":       req.Amount,
			"p_title":        req.Title,
			"p_description":  req.Description,
			"p_reference_id": req.ReferenceID,
		}
		if respBytes, err := s.sbClient.CallRPC("process_app_wallet_mutation", rpcPayload, token); err == nil {
			var rpcRes map[string]interface{}
			if err := json.Unmarshal(respBytes, &rpcRes); err == nil && rpcRes["success"] == true {
				wal := s.repo.GetOrCreateWallet(userID, email)
				balAfter, _ := rpcRes["balance_after"].(float64)
				wal.Balance = balAfter
				s.repo.SaveWallet(wal)

				trxCode, _ := rpcRes["transaction_code"].(string)
				trx := &WalletTransaction{
					ID:              fmt.Sprintf("%v", rpcRes["transaction_id"]),
					TransactionCode: trxCode,
					UserID:          userID,
					Type:            TypeDebit,
					Category:        req.Category,
					Amount:          req.Amount,
					BalanceBefore:   rpcRes["balance_before"].(float64),
					BalanceAfter:    balAfter,
					Title:           req.Title,
					Description:     req.Description,
					ReferenceID:     req.ReferenceID,
					Status:          "SUCCESS",
					CreatedAt:       time.Now(),
				}
				s.repo.AddTransaction(trx)

				// Log point reward
				s.pointService.AddRewardPoints(
					userID,
					int64(req.Amount*0.01),
					fmt.Sprintf("Cashback Belanja: %s", req.Title),
					fmt.Sprintf("Reward 1%% otomatis dari transaksi %s", trxCode),
					trxCode,
				)

				return trx, wal, nil
			}
		}
	}

	// 2. In-Memory fallback
	wal := s.repo.GetOrCreateWallet(userID, email)
	if wal.IsLocked {
		return nil, nil, errors.New("Dompet sedang dikunci")
	}
	if wal.Balance < req.Amount {
		return nil, nil, fmt.Errorf("Saldo tidak mencukupi. Saldo saat ini: Rp %s, dibutuhkan: Rp %s",
			formatRupiah(int64(wal.Balance)), formatRupiah(int64(req.Amount)))
	}

	balanceBefore := wal.Balance
	wal.Balance -= req.Amount
	balanceAfter := wal.Balance
	s.repo.SaveWallet(wal)

	trxCode := fmt.Sprintf("TRX-%s-%06d", time.Now().Format("20060102"), rand.Intn(900000)+100000)
	trx := &WalletTransaction{
		ID:              fmt.Sprintf("trx-%d", time.Now().UnixNano()),
		TransactionCode: trxCode,
		UserID:          userID,
		Type:            TypeDebit,
		Category:        req.Category,
		Amount:          req.Amount,
		BalanceBefore:   balanceBefore,
		BalanceAfter:    balanceAfter,
		Title:           req.Title,
		Description:     req.Description,
		ReferenceID:     req.ReferenceID,
		Status:          "SUCCESS",
		CreatedAt:       time.Now(),
	}
	s.repo.AddTransaction(trx)

	s.pointService.AddRewardPoints(
		userID,
		int64(req.Amount*0.01),
		fmt.Sprintf("Cashback Belanja: %s", req.Title),
		fmt.Sprintf("Reward 1%% otomatis dari transaksi %s", trxCode),
		trxCode,
	)

	return trx, wal, nil
}

func (s *walletService) TopUp(userID, email, token string, req TopUpRequest) (*WalletTransaction, *Wallet, error) {
	if req.Amount <= 0 {
		return nil, nil, errors.New("Nominal top up harus lebih besar dari 0")
	}
	channel := strings.TrimSpace(req.Channel)
	if channel == "" {
		channel = "BCA Virtual Account"
	}

	// 1. Try Supabase RPC
	if s.sbClient != nil && token != "" {
		rpcPayload := map[string]interface{}{
			"p_user_id":      userID,
			"p_type":         TypeCredit,
			"p_category":     CategoryTopup,
			"p_amount":       req.Amount,
			"p_title":        fmt.Sprintf("Top Up Saldo via %s", channel),
			"p_description":  fmt.Sprintf("Deposit saldo instan melalui metode pembayaran %s", channel),
			"p_reference_id": fmt.Sprintf("VA-%s-%d", strings.ReplaceAll(channel, " ", ""), time.Now().Unix()),
		}
		if respBytes, err := s.sbClient.CallRPC("process_app_wallet_mutation", rpcPayload, token); err == nil {
			var rpcRes map[string]interface{}
			if err := json.Unmarshal(respBytes, &rpcRes); err == nil && rpcRes["success"] == true {
				wal := s.repo.GetOrCreateWallet(userID, email)
				balAfter, _ := rpcRes["balance_after"].(float64)
				wal.Balance = balAfter
				s.repo.SaveWallet(wal)

				trxCode, _ := rpcRes["transaction_code"].(string)
				trx := &WalletTransaction{
					ID:              fmt.Sprintf("%v", rpcRes["transaction_id"]),
					TransactionCode: trxCode,
					UserID:          userID,
					Type:            TypeCredit,
					Category:        CategoryTopup,
					Amount:          req.Amount,
					BalanceBefore:   rpcRes["balance_before"].(float64),
					BalanceAfter:    balAfter,
					Title:           fmt.Sprintf("Top Up Saldo via %s", channel),
					Description:     fmt.Sprintf("Deposit saldo instan melalui metode pembayaran %s", channel),
					ReferenceID:     rpcPayload["p_reference_id"].(string),
					Status:          "SUCCESS",
					CreatedAt:       time.Now(),
				}
				s.repo.AddTransaction(trx)
				return trx, wal, nil
			}
		}
	}

	// 2. In-Memory fallback
	wal := s.repo.GetOrCreateWallet(userID, email)
	balanceBefore := wal.Balance
	wal.Balance += req.Amount
	balanceAfter := wal.Balance
	s.repo.SaveWallet(wal)

	trxCode := fmt.Sprintf("TOP-%s-%06d", time.Now().Format("20060102"), rand.Intn(900000)+100000)
	trx := &WalletTransaction{
		ID:              fmt.Sprintf("top-%d", time.Now().UnixNano()),
		TransactionCode: trxCode,
		UserID:          userID,
		Type:            TypeCredit,
		Category:        CategoryTopup,
		Amount:          req.Amount,
		BalanceBefore:   balanceBefore,
		BalanceAfter:    balanceAfter,
		Title:           fmt.Sprintf("Top Up Saldo via %s", channel),
		Description:     fmt.Sprintf("Deposit saldo instan melalui metode pembayaran %s", channel),
		ReferenceID:     fmt.Sprintf("VA-%s-%d", strings.ReplaceAll(channel, " ", ""), time.Now().Unix()),
		Status:          "SUCCESS",
		CreatedAt:       time.Now(),
	}
	s.repo.AddTransaction(trx)
	return trx, wal, nil
}

func (s *walletService) Refund(caller *supabase.User, token string, req RefundRequest) (*WalletTransaction, *Wallet, error) {
	req.TransactionCode = strings.TrimSpace(req.TransactionCode)
	if req.TransactionCode == "" {
		return nil, nil, errors.New("Kode transaksi (transaction_code) wajib disertakan")
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "Transaksi gagal diproses oleh sistem / provider partner"
	}

	isAdmin := caller.HasAnyRole("admin", "superadmin")

	// 1. Try Supabase RPC
	if s.sbClient != nil && token != "" {
		rpcPayload := map[string]interface{}{
			"p_transaction_code": req.TransactionCode,
			"p_reason":           reason,
		}
		if respBytes, err := s.sbClient.CallRPC("refund_app_wallet_transaction", rpcPayload, token); err == nil {
			var rpcRes map[string]interface{}
			if err := json.Unmarshal(respBytes, &rpcRes); err == nil && rpcRes["success"] == true {
				trxCode, _ := rpcRes["transaction_code"].(string)
				balAfter, _ := rpcRes["balance_after"].(float64)
				balBefore, _ := rpcRes["balance_before"].(float64)
				amt, _ := rpcRes["amount"].(float64)
				targetUID, _ := rpcRes["user_id"].(string)
				if targetUID == "" {
					if req.TargetUserID != "" {
						targetUID = req.TargetUserID
					} else {
						targetUID = caller.ID
					}
				}

				wal := s.repo.GetOrCreateWallet(targetUID, "")
				wal.Balance = balAfter
				s.repo.SaveWallet(wal)

				trx := &WalletTransaction{
					ID:              fmt.Sprintf("%v", rpcRes["transaction_id"]),
					TransactionCode: trxCode,
					UserID:          targetUID,
					Type:            TypeCredit,
					Category:        CategoryRefund,
					Amount:          amt,
					BalanceBefore:   balBefore,
					BalanceAfter:    balAfter,
					Title:           rpcRes["title"].(string),
					Description:     rpcRes["description"].(string),
					ReferenceID:     req.TransactionCode,
					AdminID:         caller.ID,
					AdminEmail:      caller.Email,
					AdminNotes:      reason,
					Status:          "SUCCESS",
					CreatedAt:       time.Now(),
				}
				s.repo.AddTransaction(trx)
				return trx, wal, nil
			}
		}
	}

	// 2. High-performance O(1) indexed lookup in Repository
	origTrx, targetUserID := s.repo.FindTransactionByCode(req.TransactionCode)

	if !isAdmin && origTrx != nil && targetUserID != caller.ID {
		return nil, nil, errors.New("Anda tidak memiliki izin untuk me-refund transaksi milik pengguna lain")
	}

	if targetUserID == "" {
		if req.TargetUserID != "" {
			targetUserID = req.TargetUserID
		} else {
			targetUserID = caller.ID
		}
	}

	refundAmount := req.Amount
	origTitle := req.TransactionCode

	if origTrx != nil {
		if origTrx.Type != TypeDebit {
			return nil, nil, errors.New("Hanya transaksi DEBIT (pembayaran/pengeluaran) yang dapat di-refund")
		}

		if s.repo.IsReferenceRefunded(origTrx.TransactionCode) || s.repo.IsReferenceRefunded(origTrx.ID) {
			return nil, nil, fmt.Errorf("Transaksi %s sudah pernah di-refund sebelumnya!", origTrx.TransactionCode)
		}

		refundAmount = origTrx.Amount
		origTitle = origTrx.Title
	} else if refundAmount <= 0 {
		return nil, nil, fmt.Errorf("Transaksi '%s' tidak ditemukan di buku besar dan nominal refund tidak diisi.", req.TransactionCode)
	}

	wal := s.repo.GetOrCreateWallet(targetUserID, "")
	balanceBefore := wal.Balance
	wal.Balance += refundAmount
	balanceAfter := wal.Balance
	s.repo.SaveWallet(wal)

	refundCode := fmt.Sprintf("REF-%s-%06d", time.Now().Format("20060102"), rand.Intn(900000)+100000)
	refundTrx := &WalletTransaction{
		ID:              fmt.Sprintf("ref-%d", time.Now().UnixNano()),
		TransactionCode: refundCode,
		UserID:          targetUserID,
		Type:            TypeCredit,
		Category:        CategoryRefund,
		Amount:          refundAmount,
		BalanceBefore:   balanceBefore,
		BalanceAfter:    balanceAfter,
		Title:           fmt.Sprintf("Refund Dana: %s", origTitle),
		Description:     fmt.Sprintf("Pengembalian dana saldo atas transaksi %s. Alasan: %s", req.TransactionCode, reason),
		ReferenceID:     req.TransactionCode,
		AdminID:         caller.ID,
		AdminEmail:      caller.Email,
		AdminNotes:      reason,
		Status:          "SUCCESS",
		CreatedAt:       time.Now(),
	}

	s.repo.AddTransaction(refundTrx)
	return refundTrx, wal, nil
}

func (s *walletService) AdjustBalance(adminUser *supabase.User, token string, req AdjustRequest) (*WalletTransaction, *Wallet, error) {
	req.TargetUserID = strings.TrimSpace(req.TargetUserID)
	req.Type = strings.ToUpper(strings.TrimSpace(req.Type))
	req.Reason = strings.TrimSpace(req.Reason)

	if req.TargetUserID == "" {
		return nil, nil, errors.New("Target user ID wajib disertakan")
	}
	if req.Type != TypeCredit && req.Type != TypeDebit {
		return nil, nil, errors.New("Tipe penyesuaian saldo harus 'CREDIT' (tambah saldo) atau 'DEBIT' (kurang saldo)")
	}
	if req.Amount <= 0 {
		return nil, nil, errors.New("Nominal penyesuaian harus lebih besar dari 0")
	}
	if req.Reason == "" {
		return nil, nil, errors.New("Alasan penyesuaian saldo wajib diisi untuk keperluan audit")
	}

	title := fmt.Sprintf("Koreksi Saldo oleh Super Admin (%s)", adminUser.Email)
	desc := fmt.Sprintf("Penyesuaian saldo manual: %s", req.Reason)
	if req.AdminNotes != "" {
		desc += fmt.Sprintf(" [Catatan: %s]", req.AdminNotes)
	}

	// 1. Try Supabase RPC
	if s.sbClient != nil && token != "" {
		rpcPayload := map[string]interface{}{
			"p_target_user_id": req.TargetUserID,
			"p_type":           req.Type,
			"p_amount":         req.Amount,
			"p_reason":         req.Reason,
			"p_admin_notes":    req.AdminNotes,
		}
		if respBytes, err := s.sbClient.CallRPC("admin_adjust_app_wallet", rpcPayload, token); err == nil {
			var rpcRes map[string]interface{}
			if err := json.Unmarshal(respBytes, &rpcRes); err == nil && rpcRes["success"] == true {
				wal := s.repo.GetOrCreateWallet(req.TargetUserID, "")
				balAfter, _ := rpcRes["balance_after"].(float64)
				wal.Balance = balAfter
				s.repo.SaveWallet(wal)

				trxCode, _ := rpcRes["transaction_code"].(string)
				trx := &WalletTransaction{
					ID:              fmt.Sprintf("%v", rpcRes["transaction_id"]),
					TransactionCode: trxCode,
					UserID:          req.TargetUserID,
					Type:            req.Type,
					Category:        CategoryAdjustment,
					Amount:          req.Amount,
					BalanceBefore:   rpcRes["balance_before"].(float64),
					BalanceAfter:    balAfter,
					Title:           title,
					Description:     desc,
					ReferenceID:     trxCode,
					AdminID:         adminUser.ID,
					AdminEmail:      adminUser.Email,
					AdminNotes:      req.Reason,
					Status:          "SUCCESS",
					CreatedAt:       time.Now(),
				}
				s.repo.AddTransaction(trx)
				return trx, wal, nil
			}
		}
	}

	// 2. In-Memory fallback
	wal := s.repo.GetOrCreateWallet(req.TargetUserID, "")
	balanceBefore := wal.Balance
	var balanceAfter float64

	if req.Type == TypeCredit {
		wal.Balance += req.Amount
		balanceAfter = wal.Balance
	} else {
		if wal.Balance < req.Amount {
			return nil, nil, fmt.Errorf("Saldo pengguna (Rp %s) tidak mencukupi untuk pemotongan penyesuaian Rp %s",
				formatRupiah(int64(wal.Balance)), formatRupiah(int64(req.Amount)))
		}
		wal.Balance -= req.Amount
		balanceAfter = wal.Balance
	}

	s.repo.SaveWallet(wal)

	trxCode := fmt.Sprintf("ADJ-%s-%06d", time.Now().Format("20060102"), rand.Intn(900000)+100000)
	trx := &WalletTransaction{
		ID:              fmt.Sprintf("adj-%d", time.Now().UnixNano()),
		TransactionCode: trxCode,
		UserID:          req.TargetUserID,
		Type:            req.Type,
		Category:        CategoryAdjustment,
		Amount:          req.Amount,
		BalanceBefore:   balanceBefore,
		BalanceAfter:    balanceAfter,
		Title:           title,
		Description:     desc,
		ReferenceID:     trxCode,
		AdminID:         adminUser.ID,
		AdminEmail:      adminUser.Email,
		AdminNotes:      req.Reason,
		Status:          "SUCCESS",
		CreatedAt:       time.Now(),
	}

	s.repo.AddTransaction(trx)
	return trx, wal, nil
}

func (s *walletService) GetAllWallets() []*Wallet {
	return s.repo.GetAllWallets()
}

func (s *walletService) GetUserWallet(userID string) (*Wallet, []*WalletTransaction, error) {
	wal, ok := s.repo.GetWallet(userID)
	if !ok {
		var err error
		wal, err = s.GetBalance(userID, "", "")
		if err != nil || wal == nil {
			return nil, nil, errors.New("Dompet pengguna tidak ditemukan")
		}
	}
	trxs := s.GetHistory(userID)
	if trxs == nil {
		trxs = []*WalletTransaction{}
	}
	return wal, trxs, nil
}

func (s *walletService) SetUserLock(userID string, locked bool) (*Wallet, error) {
	if s.sbClient != nil {
		rpcPayload := map[string]interface{}{
			"p_target_user_id": userID,
			"p_locked":         locked,
		}
		s.sbClient.CallRPC("admin_toggle_wallet_lock", rpcPayload, "")
	}
	wal, err := s.repo.SetLock(userID, locked)
	if err != nil || wal == nil {
		return nil, errors.New("Pengguna tidak ditemukan")
	}
	return wal, nil
}

func (s *walletService) GetAllTransactions() []*WalletTransaction {
	return s.repo.GetAllTransactions()
}

func (s *walletService) GetStats() map[string]interface{} {
	return s.repo.GetStats()
}
