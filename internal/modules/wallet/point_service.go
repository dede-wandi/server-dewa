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

// PointService defines business operations for DewaPoin rewards & redemptions
type PointService interface {
	GetHistory(userID string) []*PointTransaction
	RedeemPoints(userID, email, token string, req RedeemPointsRequest) (*PointTransaction, *Wallet, error)
	AdjustPoints(targetUserID, pType string, amount int64, reason, adminEmail string) (*PointTransaction, error)
	GetAllPointTransactions() []*PointTransaction
	AddRewardPoints(userID string, amount int64, title, description, refCode string) (*PointTransaction, error)
}

type pointService struct {
	repo     Repository
	sbClient *supabase.Client
}

// NewPointService creates a new instance of PointService
func NewPointService(repo Repository, sbClient *supabase.Client) PointService {
	return &pointService{
		repo:     repo,
		sbClient: sbClient,
	}
}

func (s *pointService) GetHistory(userID string) []*PointTransaction {
	return s.repo.GetPointTransactionsByUser(userID)
}

func (s *pointService) GetAllPointTransactions() []*PointTransaction {
	return s.repo.GetAllPointTransactions()
}

func (s *pointService) RedeemPoints(userID, email, token string, req RedeemPointsRequest) (*PointTransaction, *Wallet, error) {
	if req.Amount <= 0 {
		return nil, nil, errors.New("Nominal poin yang ditukarkan harus lebih besar dari 0")
	}

	title := "Penukaran Diskon Belanja DewaPay"
	desc := strings.TrimSpace(req.Description)
	if desc == "" {
		desc = fmt.Sprintf("Penukaran %d DewaPoin untuk voucher diskon belanja / pembayaran", req.Amount)
	}

	// 1. Try Supabase RPC if database procedure is available
	if s.sbClient != nil && token != "" {
		rpcPayload := map[string]interface{}{
			"p_user_id":     userID,
			"p_amount":      req.Amount,
			"p_description": desc,
		}
		if respBytes, err := s.sbClient.CallRPC("redeem_app_points", rpcPayload, token); err == nil {
			var rpcRes map[string]interface{}
			if err := json.Unmarshal(respBytes, &rpcRes); err == nil && rpcRes["success"] == true {
				trxCode, _ := rpcRes["transaction_code"].(string)
				ptsBefore := int64(rpcRes["points_before"].(float64))
				ptsAfter := int64(rpcRes["points_after"].(float64))

				wal := s.repo.GetOrCreateWallet(userID, email)
				wal.Points = ptsAfter
				s.repo.SaveWallet(wal)

				ptrx := &PointTransaction{
					ID:              fmt.Sprintf("%v", rpcRes["transaction_id"]),
					TransactionCode: trxCode,
					UserID:          userID,
					Type:            PointTypeDebit,
					Category:        PointCategoryRedeem,
					Amount:          req.Amount,
					PointsBefore:    ptsBefore,
					PointsAfter:     ptsAfter,
					Title:           title,
					Description:     desc,
					Status:          "SUCCESS",
					CreatedAt:       time.Now(),
				}
				s.repo.AddPointTransaction(ptrx)
				return ptrx, wal, nil
			}
		}
	}

	// 2. In-Memory business logic
	wal := s.repo.GetOrCreateWallet(userID, email)
	if wal.IsLocked {
		return nil, nil, errors.New("Dompet akun Anda sedang dikunci")
	}
	if wal.Points < req.Amount {
		return nil, nil, fmt.Errorf("Saldo DewaPoin tidak mencukupi. Saldo saat ini: %d Poin, dibutuhkan: %d Poin", wal.Points, req.Amount)
	}

	pointsBefore := wal.Points
	wal.Points -= req.Amount
	pointsAfter := wal.Points
	s.repo.SaveWallet(wal)

	trxCode := fmt.Sprintf("RED-%s-%06d", time.Now().Format("20060102"), rand.Intn(900000)+100000)
	ptrx := &PointTransaction{
		ID:              fmt.Sprintf("pt-%d", time.Now().UnixNano()),
		TransactionCode: trxCode,
		UserID:          userID,
		Type:            PointTypeDebit,
		Category:        PointCategoryRedeem,
		Amount:          req.Amount,
		PointsBefore:    pointsBefore,
		PointsAfter:     pointsAfter,
		Title:           title,
		Description:     desc,
		ReferenceID:     trxCode,
		Status:          "SUCCESS",
		CreatedAt:       time.Now(),
	}

	s.repo.AddPointTransaction(ptrx)
	return ptrx, wal, nil
}

func (s *pointService) AddRewardPoints(userID string, amount int64, title, description, refCode string) (*PointTransaction, error) {
	if amount <= 0 {
		return nil, nil
	}

	wal := s.repo.GetOrCreateWallet(userID, "")
	ptsBefore := wal.Points
	wal.Points += amount
	ptsAfter := wal.Points
	s.repo.SaveWallet(wal)

	trxCode := fmt.Sprintf("PTS-%s-%06d", time.Now().Format("20060102"), rand.Intn(900000)+100000)
	ptrx := &PointTransaction{
		ID:              fmt.Sprintf("pt-%d", time.Now().UnixNano()),
		TransactionCode: trxCode,
		UserID:          userID,
		Type:            PointTypeCredit,
		Category:        PointCategoryCashback,
		Amount:          amount,
		PointsBefore:    ptsBefore,
		PointsAfter:     ptsAfter,
		Title:           title,
		Description:     description,
		ReferenceID:     refCode,
		Status:          "SUCCESS",
		CreatedAt:       time.Now(),
	}

	s.repo.AddPointTransaction(ptrx)
	return ptrx, nil
}

func (s *pointService) AdjustPoints(targetUserID, pType string, amount int64, reason, adminEmail string) (*PointTransaction, error) {
	if amount <= 0 {
		return nil, errors.New("Nominal penyesuaian poin harus lebih besar dari 0")
	}

	wal := s.repo.GetOrCreateWallet(targetUserID, "")
	pType = strings.ToUpper(strings.TrimSpace(pType))

	var pointsBefore, pointsAfter int64
	var category string
	pointsBefore = wal.Points

	if pType == PointTypeCredit {
		category = PointCategoryAdjustment
		wal.Points += amount
		pointsAfter = wal.Points
	} else if pType == PointTypeDebit {
		category = PointCategoryAdjustment
		if wal.Points < amount {
			return nil, fmt.Errorf("Poin pengguna (%d Poin) tidak mencukupi untuk pemotongan %d Poin", wal.Points, amount)
		}
		wal.Points -= amount
		pointsAfter = wal.Points
	} else {
		return nil, errors.New("Tipe penyesuaian harus CREDIT atau DEBIT")
	}

	s.repo.SaveWallet(wal)

	trxCode := fmt.Sprintf("ADJ-PTS-%s-%06d", time.Now().Format("20060102"), rand.Intn(900000)+100000)
	ptrx := &PointTransaction{
		ID:              fmt.Sprintf("pt-adj-%d", time.Now().UnixNano()),
		TransactionCode: trxCode,
		UserID:          targetUserID,
		Type:            pType,
		Category:        category,
		Amount:          amount,
		PointsBefore:    pointsBefore,
		PointsAfter:     pointsAfter,
		Title:           fmt.Sprintf("Koreksi Poin oleh Admin (%s)", adminEmail),
		Description:     fmt.Sprintf("Penyesuaian manual: %s", reason),
		ReferenceID:     trxCode,
		Status:          "SUCCESS",
		CreatedAt:       time.Now(),
	}

	s.repo.AddPointTransaction(ptrx)
	return ptrx, nil
}
