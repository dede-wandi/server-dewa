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
	ApproveTopUp(adminUser *supabase.User, token string, transactionCode, notes string) (*WalletTransaction, *Wallet, error)
	RejectTopUp(adminUser *supabase.User, token string, transactionCode, reason string) (*WalletTransaction, error)
	GetPendingTopUps() []*WalletTransaction
	Refund(caller *supabase.User, token string, req RefundRequest) (*WalletTransaction, *Wallet, error)
	AdjustBalance(adminUser *supabase.User, token string, req AdjustRequest) (*WalletTransaction, *Wallet, error)

	// PIN & Transfer Security
	SetupPIN(userID, token string, req SetupPINRequest) error
	ChangePIN(userID, token string, req ChangePINRequest) error
	VerifyPIN(userID, pin string) error
	Transfer(caller *supabase.User, token string, req TransferRequest) (*WalletTransaction, *Wallet, error)

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
	// 1. Try Supabase RPCs
	if s.sbClient != nil {
		rpcPayload := map[string]interface{}{
			"p_user_id": userID,
		}

		// 1a. Try admin_get_user_detail (Live in Supabase schema cache)
		if respBytes, err := s.sbClient.CallRPC("admin_get_user_detail", rpcPayload, ""); err == nil && len(respBytes) > 0 {
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
				if isPinSet, ok := rpcRes["is_pin_set"].(bool); ok {
					wal.IsPINSet = isPinSet
				}
				if pinHash, ok := rpcRes["pin_hash"].(string); ok && pinHash != "" {
					wal.PINHash = pinHash
					wal.IsPINSet = true
				}
				s.repo.SaveWallet(wal)
				return wal, nil
			}
		}

		// 1b. Try get_user_wallet_info
		if respBytes, err := s.sbClient.CallRPC("get_user_wallet_info", rpcPayload, token); err == nil && len(respBytes) > 0 {
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
				if isPinSet, ok := rpcRes["is_pin_set"].(bool); ok {
					wal.IsPINSet = isPinSet
				}
				if pinHash, ok := rpcRes["pin_hash"].(string); ok && pinHash != "" {
					wal.PINHash = pinHash
					wal.IsPINSet = true
				}
				s.repo.SaveWallet(wal)
				return wal, nil
			}
		}

		// 1c. Try get_all_registered_users_wallets (Live in Supabase schema cache)
		if respBytes, err := s.sbClient.CallRPC("get_all_registered_users_wallets", nil, ""); err == nil && len(respBytes) > 0 {
			var list []map[string]interface{}
			if err := json.Unmarshal(respBytes, &list); err == nil {
				for _, item := range list {
					if uid, ok := item["user_id"].(string); ok && strings.EqualFold(uid, userID) {
						wal := s.repo.GetOrCreateWallet(userID, email)
						if bal, ok := item["balance"].(float64); ok {
							wal.Balance = bal
						}
						if pts, ok := item["points"].(float64); ok {
							wal.Points = int64(pts)
						}
						if locked, ok := item["is_locked"].(bool); ok {
							wal.IsLocked = locked
						}
						if isPinSet, ok := item["is_pin_set"].(bool); ok {
							wal.IsPINSet = isPinSet
						}
						s.repo.SaveWallet(wal)
						return wal, nil
					}
				}
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

	wal, _ := s.repo.GetWallet(userID)
	if wal != nil && wal.IsPINSet {
		pin := strings.TrimSpace(req.PIN)
		if pin == "" {
			return nil, nil, errors.New("PIN transaksi wajib disertakan untuk melakukan pembayaran")
		}
		if !CheckPIN(pin, wal.PINHash) {
			return nil, nil, errors.New("PIN transaksi salah. Pembayaran ditolak.")
		}
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
	wal = s.repo.GetOrCreateWallet(userID, email)
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
	notes := strings.TrimSpace(req.Notes)

	// 1. Try Supabase RPC request_app_wallet_topup (returns status PENDING)
	if s.sbClient != nil && token != "" {
		rpcPayload := map[string]interface{}{
			"p_user_id": userID,
			"p_amount":  req.Amount,
			"p_channel": channel,
			"p_notes":   notes,
		}
		if respBytes, err := s.sbClient.CallRPC("request_app_wallet_topup", rpcPayload, token); err == nil {
			var rpcRes map[string]interface{}
			if err := json.Unmarshal(respBytes, &rpcRes); err == nil && rpcRes["success"] == true {
				wal, _ := s.GetBalance(userID, email, token)
				if wal == nil {
					wal = s.repo.GetOrCreateWallet(userID, email)
				}

				trxCode, _ := rpcRes["transaction_code"].(string)
				paymentCode, _ := rpcRes["payment_code"].(string)
				trx := &WalletTransaction{
					ID:              fmt.Sprintf("%v", rpcRes["transaction_id"]),
					TransactionCode: trxCode,
					UserID:          userID,
					Type:            TypeCredit,
					Category:        CategoryTopup,
					Amount:          req.Amount,
					BalanceBefore:   wal.Balance,
					BalanceAfter:    wal.Balance, // Saldo tidak bertambah saat masih PENDING
					Title:           fmt.Sprintf("Permintaan Top Up Saldo via %s", channel),
					Description:     "Menunggu pembayaran dan verifikasi admin / callback payment gateway",
					ReferenceID:     paymentCode,
					AdminNotes:      notes,
					Status:          "PENDING",
					CreatedAt:       time.Now(),
				}
				s.repo.AddTransaction(trx)
				return trx, wal, nil
			}
		}
	}

	// 2. In-Memory fallback (status PENDING, balance NOT added)
	wal, _ := s.GetBalance(userID, email, token)
	if wal == nil {
		wal = s.repo.GetOrCreateWallet(userID, email)
	}

	trxCode := fmt.Sprintf("TOP-%s-%06d", time.Now().Format("20060102"), rand.Intn(900000)+100000)
	cleanChannel := strings.ToUpper(strings.ReplaceAll(channel, " ", ""))
	refID := fmt.Sprintf("VA-%s-%d", cleanChannel, time.Now().Unix())

	trx := &WalletTransaction{
		ID:              fmt.Sprintf("top-%d", time.Now().UnixNano()),
		TransactionCode: trxCode,
		UserID:          userID,
		Type:            TypeCredit,
		Category:        CategoryTopup,
		Amount:          req.Amount,
		BalanceBefore:   wal.Balance,
		BalanceAfter:    wal.Balance, // Saldo tetap sebelum disetujui
		Title:           fmt.Sprintf("Permintaan Top Up Saldo via %s", channel),
		Description:     "Menunggu pembayaran dan verifikasi admin / callback payment gateway",
		ReferenceID:     refID,
		AdminNotes:      notes,
		Status:          "PENDING",
		CreatedAt:       time.Now(),
	}
	s.repo.AddTransaction(trx)
	return trx, wal, nil
}

func (s *walletService) ApproveTopUp(adminUser *supabase.User, token string, transactionCode, notes string) (*WalletTransaction, *Wallet, error) {
	transactionCode = strings.TrimSpace(transactionCode)
	if transactionCode == "" {
		return nil, nil, errors.New("Kode transaksi (transaction_code) wajib disertakan")
	}

	// 1. Try Supabase RPC approve_app_wallet_topup
	if s.sbClient != nil && token != "" {
		var adminID interface{} = nil
		if adminUser != nil && adminUser.ID != "" {
			adminID = adminUser.ID
		}
		rpcPayload := map[string]interface{}{
			"p_transaction_code": transactionCode,
			"p_admin_id":         adminID,
			"p_admin_notes":      notes,
		}
		if respBytes, err := s.sbClient.CallRPC("approve_app_wallet_topup", rpcPayload, token); err == nil {
			var rpcRes map[string]interface{}
			if err := json.Unmarshal(respBytes, &rpcRes); err == nil && rpcRes["success"] == true {
				targetUID, _ := rpcRes["user_id"].(string)
				balAfter, _ := rpcRes["balance_after"].(float64)
				balBefore, _ := rpcRes["balance_before"].(float64)
				amt, _ := rpcRes["amount"].(float64)

				wal := s.repo.GetOrCreateWallet(targetUID, "")
				wal.Balance = balAfter
				s.repo.SaveWallet(wal)

				trx, _ := s.repo.FindTransactionByCode(transactionCode)
				if trx == nil {
					trx = &WalletTransaction{
						ID:              fmt.Sprintf("%v", rpcRes["transaction_id"]),
						TransactionCode: transactionCode,
						UserID:          targetUID,
						Type:            TypeCredit,
						Category:        CategoryTopup,
						Amount:          amt,
						ReferenceID:     transactionCode,
						CreatedAt:       time.Now(),
					}
					s.repo.AddTransaction(trx)
				}
				trx.Status = "SUCCESS"
				trx.BalanceBefore = balBefore
				trx.BalanceAfter = balAfter
				trx.AdminNotes = notes
				if adminUser != nil {
					trx.AdminID = adminUser.ID
					trx.AdminEmail = adminUser.Email
				}
				trx.Title = "Top Up Saldo (Disetujui Admin)"
				trx.Description = fmt.Sprintf("Deposit saldo telah diverifikasi dan disetujui. Catatan: %s", notes)
				s.repo.UpdateTransaction(trx)
				return trx, wal, nil
			}
		}
	}

	// 2. In-Memory fallback
	trx, targetUserID := s.repo.FindTransactionByCode(transactionCode)
	if trx == nil {
		return nil, nil, fmt.Errorf("Transaksi top up '%s' tidak ditemukan", transactionCode)
	}

	if trx.Status != "PENDING" {
		return nil, nil, fmt.Errorf("Transaksi '%s' tidak dapat disetujui karena status saat ini adalah %s", transactionCode, trx.Status)
	}

	wal, _ := s.GetBalance(targetUserID, "", token)
	if wal == nil {
		wal = s.repo.GetOrCreateWallet(targetUserID, "")
	}

	balanceBefore := wal.Balance
	wal.Balance += trx.Amount
	balanceAfter := wal.Balance
	s.repo.SaveWallet(wal)

	trx.Status = "SUCCESS"
	trx.BalanceBefore = balanceBefore
	trx.BalanceAfter = balanceAfter
	trx.Title = strings.Replace(trx.Title, "Permintaan ", "", 1)
	trx.Description = fmt.Sprintf("Deposit saldo telah disetujui. %s", notes)
	if adminUser != nil {
		trx.AdminID = adminUser.ID
		trx.AdminEmail = adminUser.Email
	}
	trx.AdminNotes = notes
	s.repo.UpdateTransaction(trx)

	return trx, wal, nil
}

func (s *walletService) RejectTopUp(adminUser *supabase.User, token string, transactionCode, reason string) (*WalletTransaction, error) {
	transactionCode = strings.TrimSpace(transactionCode)
	if transactionCode == "" {
		return nil, errors.New("Kode transaksi (transaction_code) wajib disertakan")
	}

	if reason == "" {
		reason = "Ditolak oleh Admin"
	}

	// 1. Try Supabase RPC reject_app_wallet_topup
	if s.sbClient != nil && token != "" {
		var adminID interface{} = nil
		if adminUser != nil && adminUser.ID != "" {
			adminID = adminUser.ID
		}
		rpcPayload := map[string]interface{}{
			"p_transaction_code": transactionCode,
			"p_admin_id":         adminID,
			"p_reason":           reason,
		}
		if respBytes, err := s.sbClient.CallRPC("reject_app_wallet_topup", rpcPayload, token); err == nil {
			var rpcRes map[string]interface{}
			if err := json.Unmarshal(respBytes, &rpcRes); err == nil && rpcRes["success"] == true {
				trx, _ := s.repo.FindTransactionByCode(transactionCode)
				if trx != nil {
					trx.Status = "FAILED"
					trx.AdminNotes = reason
					s.repo.UpdateTransaction(trx)
					return trx, nil
				}
			}
		}
	}

	// 2. In-Memory fallback
	trx, _ := s.repo.FindTransactionByCode(transactionCode)
	if trx == nil {
		return nil, fmt.Errorf("Transaksi top up '%s' tidak ditemukan", transactionCode)
	}

	if trx.Status != "PENDING" {
		return nil, fmt.Errorf("Transaksi '%s' tidak dapat ditolak karena status saat ini adalah %s", transactionCode, trx.Status)
	}

	trx.Status = "FAILED"
	trx.AdminNotes = reason
	trx.Description = fmt.Sprintf("Permintaan top up ditolak. Alasan: %s", reason)
	if adminUser != nil {
		trx.AdminID = adminUser.ID
		trx.AdminEmail = adminUser.Email
	}
	s.repo.UpdateTransaction(trx)

	return trx, nil
}


func (s *walletService) GetPendingTopUps() []*WalletTransaction {
	return s.repo.GetPendingTopUps()
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

	adminID := "system"
	adminEmail := "System"
	if adminUser != nil {
		if adminUser.ID != "" {
			adminID = adminUser.ID
		}
		if adminUser.Email != "" {
			adminEmail = adminUser.Email
		}
	}
	title := fmt.Sprintf("Koreksi Saldo oleh Super Admin (%s)", adminEmail)
	desc := fmt.Sprintf("Penyesuaian saldo manual: %s", req.Reason)
	if req.AdminNotes != "" {
		desc += fmt.Sprintf(" [Catatan: %s]", req.AdminNotes)
	}

	// 1. Try Supabase RPC
	if s.sbClient != nil && token != "" {
		var adminID interface{} = nil
		if adminUser != nil && adminUser.ID != "" {
			adminID = adminUser.ID
		}
		rpcPayload := map[string]interface{}{
			"p_target_user_id": req.TargetUserID,
			"p_type":           req.Type,
			"p_amount":         req.Amount,
			"p_reason":         req.Reason,
			"p_admin_id":       adminID,
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

	// 2. In-Memory fallback (sync live balance first if available)
	wal, _ := s.GetBalance(req.TargetUserID, "", token)
	if wal == nil {
		wal = s.repo.GetOrCreateWallet(req.TargetUserID, "")
	}
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
		AdminID:         adminID,
		AdminEmail:      adminEmail,
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

func (s *walletService) SetupPIN(userID, token string, req SetupPINRequest) error {
	pin := strings.TrimSpace(req.PIN)
	confirmPIN := strings.TrimSpace(req.ConfirmPIN)

	if pin == "" {
		return errors.New("PIN transaksi wajib diisi")
	}
	if err := ValidatePIN(pin); err != nil {
		return err
	}
	if pin != confirmPIN {
		return errors.New("Konfirmasi PIN tidak cocok dengan PIN")
	}

	wal, err := s.GetBalance(userID, "", token)
	if err != nil {
		return err
	}

	if wal.IsPINSet && wal.PINHash != "" {
		return errors.New("PIN transaksi sudah pernah dibuat. Gunakan fitur ubah PIN jika ingin mengganti.")
	}

	hashed, err := HashPIN(pin)
	if err != nil {
		return fmt.Errorf("Gagal mengenkripsi PIN: %w", err)
	}

	// 1. Try Supabase RPC
	if s.sbClient != nil && token != "" {
		rpcPayload := map[string]interface{}{
			"p_user_id":  userID,
			"p_pin_hash": hashed,
		}
		_, _ = s.sbClient.CallRPC("setup_wallet_pin", rpcPayload, token)
	}

	wal.PINHash = hashed
	wal.IsPINSet = true
	s.repo.SaveWallet(wal)
	return nil
}

func (s *walletService) ChangePIN(userID, token string, req ChangePINRequest) error {
	oldPIN := strings.TrimSpace(req.OldPIN)
	newPIN := strings.TrimSpace(req.NewPIN)
	confirmPIN := strings.TrimSpace(req.ConfirmPIN)

	if oldPIN == "" {
		return errors.New("PIN lama wajib diisi")
	}
	if newPIN == "" {
		return errors.New("PIN baru wajib diisi")
	}
	if err := ValidatePIN(newPIN); err != nil {
		return err
	}
	if newPIN != confirmPIN {
		return errors.New("Konfirmasi PIN baru tidak cocok")
	}
	if oldPIN == newPIN {
		return errors.New("PIN baru tidak boleh sama dengan PIN lama")
	}

	wal, err := s.GetBalance(userID, "", token)
	if err != nil {
		return err
	}

	if !wal.IsPINSet || wal.PINHash == "" {
		return errors.New("PIN transaksi belum dibuat. Silakan buat PIN terlebih dahulu.")
	}

	if !CheckPIN(oldPIN, wal.PINHash) {
		return errors.New("PIN lama tidak sesuai / salah")
	}

	hashed, err := HashPIN(newPIN)
	if err != nil {
		return fmt.Errorf("Gagal mengenkripsi PIN baru: %w", err)
	}

	// 1. Try Supabase RPC
	if s.sbClient != nil && token != "" {
		rpcPayload := map[string]interface{}{
			"p_user_id":      userID,
			"p_new_pin_hash": hashed,
		}
		_, _ = s.sbClient.CallRPC("change_wallet_pin", rpcPayload, token)
	}

	wal.PINHash = hashed
	wal.IsPINSet = true
	s.repo.SaveWallet(wal)
	return nil
}

func (s *walletService) VerifyPIN(userID, pin string) error {
	wal, ok := s.repo.GetWallet(userID)
	if !ok || !wal.IsPINSet || wal.PINHash == "" {
		return errors.New("PIN transaksi belum dibuat. Silakan buat PIN terlebih dahulu.")
	}
	if !CheckPIN(pin, wal.PINHash) {
		return errors.New("PIN transaksi salah")
	}
	return nil
}

func (s *walletService) Transfer(caller *supabase.User, token string, req TransferRequest) (*WalletTransaction, *Wallet, error) {
	if caller == nil {
		return nil, nil, errors.New("Unauthorized: sesi pengguna tidak valid")
	}
	if req.Amount <= 0 {
		return nil, nil, errors.New("Nominal transfer harus lebih besar dari 0")
	}
	if req.Amount < 1000 {
		return nil, nil, errors.New("Minimal transfer saldo adalah Rp 1.000")
	}

	pin := strings.TrimSpace(req.PIN)
	if pin == "" {
		return nil, nil, errors.New("PIN transaksi wajib disertakan untuk melakukan transfer")
	}

	// 1. Ambil data dompet pengirim & verifikasi PIN
	senderWallet, err := s.GetBalance(caller.ID, caller.Email, token)
	if err != nil {
		return nil, nil, err
	}
	if !senderWallet.IsPINSet || senderWallet.PINHash == "" {
		return nil, nil, errors.New("PIN transaksi belum dibuat. Silakan buat PIN terlebih dahulu sebelum transfer.")
	}
	if !CheckPIN(pin, senderWallet.PINHash) {
		return nil, nil, errors.New("PIN transaksi salah. Transfer dibatalkan.")
	}

	if senderWallet.IsLocked {
		return nil, nil, errors.New("Dompet Anda sedang dikunci. Silakan hubungi CS DewaCell.")
	}

	if senderWallet.Balance < req.Amount {
		return nil, nil, fmt.Errorf("Saldo Anda tidak mencukupi. Saldo saat ini: Rp %s, Dibutuhkan: Rp %s",
			formatRupiah(int64(senderWallet.Balance)), formatRupiah(int64(req.Amount)))
	}

	// 2. Tentukan target user
	targetID := strings.TrimSpace(req.TargetUserID)
	targetEmail := strings.TrimSpace(req.TargetEmail)

	if targetID == caller.ID || (targetEmail != "" && strings.EqualFold(targetEmail, caller.Email)) {
		return nil, nil, errors.New("Tidak dapat mentransfer saldo ke akun sendiri")
	}

	var targetWallet *Wallet
	if targetID != "" {
		targetWallet, _ = s.repo.GetWallet(targetID)
	}
	if targetWallet == nil && targetEmail != "" {
		targetWallet, _ = s.repo.FindWalletByEmail(targetEmail)
	}

	if targetWallet == nil {
		if targetID != "" {
			targetWallet = s.repo.GetOrCreateWallet(targetID, targetEmail)
		} else if targetEmail != "" {
			targetWallet = s.repo.GetOrCreateWallet("usr-"+strings.ReplaceAll(targetEmail, "@", "-at-"), targetEmail)
		} else {
			return nil, nil, errors.New("Tujuan transfer tidak valid. Sertakan target_user_id atau target_email penerima.")
		}
	}

	if targetWallet.UserID == caller.ID {
		return nil, nil, errors.New("Tidak dapat mentransfer saldo ke akun sendiri")
	}

	if targetWallet.IsLocked {
		return nil, nil, errors.New("Dompet penerima sedang dibekukan / dikunci.")
	}

	notes := strings.TrimSpace(req.Notes)
	if notes == "" {
		notes = "Transfer saldo DewaPay"
	}

	// 3. Coba Supabase RPC jika targetID adalah UUID
	if s.sbClient != nil && token != "" && len(targetWallet.UserID) == 36 {
		rpcPayload := map[string]interface{}{
			"p_sender_id":   caller.ID,
			"p_receiver_id": targetWallet.UserID,
			"p_amount":      req.Amount,
			"p_notes":       notes,
		}
		if respBytes, err := s.sbClient.CallRPC("transfer_app_wallet_balance", rpcPayload, token); err == nil {
			var rpcRes map[string]interface{}
			if err := json.Unmarshal(respBytes, &rpcRes); err == nil && rpcRes["success"] == true {
				senderWallet.Balance -= req.Amount
				s.repo.SaveWallet(senderWallet)

				targetWallet.Balance += req.Amount
				s.repo.SaveWallet(targetWallet)

				senderTrxCode, _ := rpcRes["sender_transaction_code"].(string)
				if senderTrxCode == "" {
					senderTrxCode = fmt.Sprintf("TFO-%s-%06d", time.Now().Format("20060102"), time.Now().UnixNano()%1000000)
				}

				senderTrx := &WalletTransaction{
					ID:              "trx-" + senderTrxCode,
					TransactionCode: senderTrxCode,
					UserID:          caller.ID,
					Type:            TypeDebit,
					Category:        CategoryTransferOut,
					Amount:          req.Amount,
					BalanceBefore:   senderWallet.Balance + req.Amount,
					BalanceAfter:    senderWallet.Balance,
					Title:           "Transfer Saldo ke " + targetWallet.UserEmail,
					Description:     notes,
					ReferenceID:     targetWallet.UserID,
					Status:          "SUCCESS",
					CreatedAt:       time.Now(),
				}
				s.repo.AddTransaction(senderTrx)
				return senderTrx, senderWallet, nil
			}
		}
	}

	// 4. Memory execution fallback
	senderBalanceBefore := senderWallet.Balance
	senderBalanceAfter := senderBalanceBefore - req.Amount
	senderWallet.Balance = senderBalanceAfter
	s.repo.SaveWallet(senderWallet)

	targetBalanceBefore := targetWallet.Balance
	targetBalanceAfter := targetBalanceBefore + req.Amount
	targetWallet.Balance = targetBalanceAfter
	s.repo.SaveWallet(targetWallet)

	trxCodeOut := fmt.Sprintf("TFO-%s-%06d", time.Now().Format("20060102"), time.Now().UnixNano()%1000000)
	trxCodeIn := fmt.Sprintf("TFI-%s-%06d", time.Now().Format("20060102"), time.Now().UnixNano()%1000000)

	senderTrx := &WalletTransaction{
		ID:              "trx-" + trxCodeOut,
		TransactionCode: trxCodeOut,
		UserID:          caller.ID,
		Type:            TypeDebit,
		Category:        CategoryTransferOut,
		Amount:          req.Amount,
		BalanceBefore:   senderBalanceBefore,
		BalanceAfter:    senderBalanceAfter,
		Title:           "Transfer Saldo ke " + targetWallet.UserEmail,
		Description:     notes,
		ReferenceID:     trxCodeIn,
		Status:          "SUCCESS",
		CreatedAt:       time.Now(),
	}
	s.repo.AddTransaction(senderTrx)

	recipientTrx := &WalletTransaction{
		ID:              "trx-" + trxCodeIn,
		TransactionCode: trxCodeIn,
		UserID:          targetWallet.UserID,
		Type:            TypeCredit,
		Category:        CategoryTransferIn,
		Amount:          req.Amount,
		BalanceBefore:   targetBalanceBefore,
		BalanceAfter:    targetBalanceAfter,
		Title:           "Terima Saldo dari " + caller.Email,
		Description:     notes,
		ReferenceID:     trxCodeOut,
		Status:          "SUCCESS",
		CreatedAt:       time.Now(),
	}
	s.repo.AddTransaction(recipientTrx)

	return senderTrx, senderWallet, nil
}
