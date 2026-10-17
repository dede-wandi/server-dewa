package agency

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/dede-wandi/server-dewa/internal/modules/wallet"
	"github.com/dede-wandi/server-dewa/internal/supabase"
)

// Service defines business operations for agency management
type Service interface {
	GetOrCreateProfile(userID, defaultName string) (*AgencyProfile, error)
	UpdateProfile(userID string, req UpdateProfileRequest) (*AgencyProfile, error)
	GetDashboard(userID, defaultName string) (*AgencyDashboardResponse, error)
	ListMembers(agentID string) ([]*AgencyMember, error)
	GetMember(agentID, memberID string) (*AgencyMember, error)
	RegisterMember(agentID string, req RegisterMemberRequest) (*AgencyMember, error)
	GetMemberMarkups(agentID, memberID string) ([]*AgentCategoryMarkup, error)
	UpdateMemberMarkups(agentID, memberID string, markups []CategoryMarkupInput) error
	GetDefaultMarkups(agentID string) ([]*AgentCategoryMarkup, error)
	UpdateDefaultMarkups(agentID string, markups []CategoryMarkupInput) error
	CopyMarkups(agentID string, req CopyMarkupsRequest) error
	GetUplineMarkupForUser(userID string) (map[string]float64, string, error)
	ListCommissions(agentID string) ([]*wallet.WalletTransaction, error)
}

type service struct {
	sbClient      *supabase.Client
	walletService wallet.WalletService
	mu            sync.RWMutex

	// In-memory caches for fast responses
	profiles      map[string]*AgencyProfile        // userID -> Profile
	membersByAgent map[string][]*AgencyMember      // agentID -> list of members
	memberUpline  map[string]string                // memberID -> agentID
	markups       map[string]*AgentCategoryMarkup  // "agentID:memberID:categoryID" -> Markup
}

// NewService creates a new agency service
func NewService(sbClient *supabase.Client, walletService wallet.WalletService) Service {
	s := &service{
		sbClient:       sbClient,
		walletService:  walletService,
		profiles:       make(map[string]*AgencyProfile),
		membersByAgent: make(map[string][]*AgencyMember),
		memberUpline:   make(map[string]string),
		markups:        make(map[string]*AgentCategoryMarkup),
	}

	// Initial sync from DB
	go s.syncFromDatabase()

	return s
}

func (s *service) syncFromDatabase() {
	if s.sbClient == nil {
		return
	}

	// 1. Sync Profiles
	if respBytes, err := s.sbClient.GetDB("agency_profiles", "select=*", ""); err == nil {
		var profiles []*AgencyProfile
		if err := json.Unmarshal(respBytes, &profiles); err == nil {
			s.mu.Lock()
			for _, p := range profiles {
				s.profiles[p.UserID] = p
			}
			s.mu.Unlock()
		}
	}

	// 2. Sync Members
	if respBytes, err := s.sbClient.GetDB("agency_members", "select=*", ""); err == nil {
		var members []*AgencyMember
		if err := json.Unmarshal(respBytes, &members); err == nil {
			s.mu.Lock()
			for _, m := range members {
				s.memberUpline[m.MemberID] = m.AgentID
				s.membersByAgent[m.AgentID] = append(s.membersByAgent[m.AgentID], m)
			}
			s.mu.Unlock()
		}
	}

	// 3. Sync Markups
	if respBytes, err := s.sbClient.GetDB("agent_category_markups", "select=*", ""); err == nil {
		var markups []*AgentCategoryMarkup
		if err := json.Unmarshal(respBytes, &markups); err == nil {
			s.mu.Lock()
			for _, mk := range markups {
				mID := ""
				if mk.MemberID != nil {
					mID = *mk.MemberID
				}
				key := fmt.Sprintf("%s:%s:%s", mk.AgentID, mID, mk.CategoryID)
				s.markups[key] = mk
			}
			s.mu.Unlock()
		}
	}
}

func (s *service) GetOrCreateProfile(userID, defaultName string) (*AgencyProfile, error) {
	cleanName := strings.TrimSpace(defaultName)

	s.mu.RLock()
	p, ok := s.profiles[userID]
	s.mu.RUnlock()
	if ok && p != nil {
		if cleanName != "" && (p.BusinessName == "" || p.BusinessName == "Agen Mitra DewaCell" || p.BusinessName == "Mitra Agen DewaCell") {
			p.BusinessName = cleanName
			p.UpdatedAt = time.Now()
			if s.sbClient != nil {
				filter := fmt.Sprintf("user_id=eq.%s", userID)
				_, _ = s.sbClient.PatchDB("agency_profiles", filter, map[string]interface{}{
					"business_name": cleanName,
					"updated_at":    p.UpdatedAt,
				}, "")
			}
		}
		return p, nil
	}

	// Check DB
	if s.sbClient != nil {
		filter := fmt.Sprintf("user_id=eq.%s", userID)
		if respBytes, err := s.sbClient.GetDB("agency_profiles", "select=*&"+filter, ""); err == nil {
			var list []*AgencyProfile
			if err := json.Unmarshal(respBytes, &list); err == nil && len(list) > 0 {
				existing := list[0]
				if cleanName != "" && (existing.BusinessName == "" || existing.BusinessName == "Agen Mitra DewaCell" || existing.BusinessName == "Mitra Agen DewaCell") {
					existing.BusinessName = cleanName
					existing.UpdatedAt = time.Now()
					_, _ = s.sbClient.PatchDB("agency_profiles", filter, map[string]interface{}{
						"business_name": cleanName,
						"updated_at":    existing.UpdatedAt,
					}, "")
				}
				s.mu.Lock()
				s.profiles[userID] = existing
				s.mu.Unlock()
				return existing, nil
			}
		}
	}

	// Auto-create new agency profile
	bName := cleanName
	if bName == "" {
		bName = "Akun Agen"
	}

	randDigits := fmt.Sprintf("%04d", rand.Intn(9000)+1000)
	refCode := fmt.Sprintf("AGEN%s", randDigits)

	newProfile := &AgencyProfile{
		ID:           fmt.Sprintf("prof-%d", time.Now().UnixNano()),
		UserID:       userID,
		ReferralCode: refCode,
		BusinessName: bName,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if s.sbClient != nil {
		record := map[string]interface{}{
			"user_id":       userID,
			"referral_code": refCode,
			"business_name": bName,
			"is_active":     true,
		}
		if respBytes, err := s.sbClient.PostDB("agency_profiles", record, "return=representation", ""); err == nil {
			var inserted []*AgencyProfile
			if err := json.Unmarshal(respBytes, &inserted); err == nil && len(inserted) > 0 {
				newProfile = inserted[0]
			}
		}
	}

	s.mu.Lock()
	s.profiles[userID] = newProfile
	s.mu.Unlock()

	return newProfile, nil
}

func (s *service) UpdateProfile(userID string, req UpdateProfileRequest) (*AgencyProfile, error) {
	prof, err := s.GetOrCreateProfile(userID, "")
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.BusinessName) != "" {
		prof.BusinessName = strings.TrimSpace(req.BusinessName)
		prof.UpdatedAt = time.Now()

		if s.sbClient != nil {
			filter := fmt.Sprintf("user_id=eq.%s", userID)
			_, _ = s.sbClient.PatchDB("agency_profiles", filter, map[string]interface{}{
				"business_name": prof.BusinessName,
				"updated_at":    prof.UpdatedAt,
			}, "")
		}
	}

	return prof, nil
}

func (s *service) GetDashboard(userID, defaultName string) (*AgencyDashboardResponse, error) {
	prof, err := s.GetOrCreateProfile(userID, defaultName)
	if err != nil {
		return nil, err
	}

	members, _ := s.ListMembers(userID)
	commissions, _ := s.ListCommissions(userID)

	var totalCommission float64
	var todayCommission float64
	todayStr := time.Now().Format("2006-01-02")

	for _, c := range commissions {
		if c == nil {
			continue
		}
		totalCommission += c.Amount
		if c.CreatedAt.Format("2006-01-02") == todayStr {
			todayCommission += c.Amount
		}
	}

	recentMembers := make([]*AgencyMember, 0)
	if len(members) > 5 {
		recentMembers = members[:5]
	} else {
		recentMembers = members
	}

	recentComms := make([]*wallet.WalletTransaction, 0)
	if len(commissions) > 5 {
		recentComms = commissions[:5]
	} else {
		recentComms = commissions
	}

	return &AgencyDashboardResponse{
		Profile:           prof,
		TotalMembers:      len(members),
		TotalCommission:   totalCommission,
		TodayCommission:   todayCommission,
		TotalOrders:       len(commissions),
		RecentMembers:     recentMembers,
		RecentCommissions: recentComms,
	}, nil
}

func (s *service) ListMembers(agentID string) ([]*AgencyMember, error) {
	if s.sbClient != nil {
		filter := fmt.Sprintf("agent_id=eq.%s&order=joined_at.desc", agentID)
		if respBytes, err := s.sbClient.GetDB("agency_members", "select=*&"+filter, ""); err == nil {
			var list []*AgencyMember
			if err := json.Unmarshal(respBytes, &list); err == nil {
				s.mu.Lock()
				s.membersByAgent[agentID] = list
				for _, m := range list {
					s.memberUpline[m.MemberID] = agentID
				}
				s.mu.Unlock()
				return list, nil
			}
		}
	}

	s.mu.RLock()
	cached := s.membersByAgent[agentID]
	s.mu.RUnlock()
	if len(cached) > 0 {
		return cached, nil
	}

	return []*AgencyMember{}, nil
}

func (s *service) GetMember(agentID, memberID string) (*AgencyMember, error) {
	members, err := s.ListMembers(agentID)
	if err != nil {
		return nil, err
	}
	for _, m := range members {
		if m.MemberID == memberID || m.ID == memberID {
			return m, nil
		}
	}
	return nil, errors.New("Anggota / Downline tidak ditemukan")
}

func (s *service) RegisterMember(agentID string, req RegisterMemberRequest) (*AgencyMember, error) {
	name := strings.TrimSpace(req.Name)
	phone := strings.TrimSpace(req.Phone)
	password := strings.TrimSpace(req.Password)

	if name == "" {
		return nil, errors.New("Nama anggota wajib diisi")
	}
	if phone == "" {
		return nil, errors.New("Nomor HP anggota wajib diisi")
	}
	if len(password) < 6 {
		return nil, errors.New("Password minimal 6 karakter")
	}

	// Format nomor HP ke 08xxxx
	cleanPhone := strings.ReplaceAll(phone, "-", "")
	cleanPhone = strings.ReplaceAll(cleanPhone, " ", "")
	cleanPhone = strings.ReplaceAll(cleanPhone, "+", "")
	if strings.HasPrefix(cleanPhone, "62") {
		cleanPhone = "0" + cleanPhone[2:]
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		email = fmt.Sprintf("%s@member.dewacell.com", cleanPhone)
	}

	bName := strings.TrimSpace(req.BusinessName)
	if bName == "" {
		bName = name
	}

	var newMember *AgencyMember
	var memberUserID string
	var lastErr error

	if s.sbClient != nil {
		// 1. Prioritas Utama: Panggil PostgreSQL RPC create_agency_member_user (Direct Database Creation)
		rpcPayload := map[string]interface{}{
			"p_agent_id":      agentID,
			"p_name":          name,
			"p_phone":         cleanPhone,
			"p_password":      password,
			"p_business_name": bName,
		}

		rpcRespBytes, err := s.sbClient.CallRPC("create_agency_member_user", rpcPayload, "")
		if err == nil {
			var rpcRes struct {
				Success     bool   `json:"success"`
				ID          string `json:"id"`
				AgentID     string `json:"agent_id"`
				MemberID    string `json:"member_id"`
				MemberName  string `json:"member_name"`
				MemberPhone string `json:"member_phone"`
				Status      string `json:"status"`
				JoinedAt    string `json:"joined_at"`
			}
			if err := json.Unmarshal(rpcRespBytes, &rpcRes); err == nil && rpcRes.MemberID != "" {
				parsedJoinedAt, _ := time.Parse(time.RFC3339, rpcRes.JoinedAt)
				if parsedJoinedAt.IsZero() {
					parsedJoinedAt = time.Now()
				}
				newMember = &AgencyMember{
					ID:          rpcRes.ID,
					AgentID:     rpcRes.AgentID,
					MemberID:    rpcRes.MemberID,
					MemberName:  rpcRes.MemberName,
					MemberPhone: rpcRes.MemberPhone,
					Status:      rpcRes.Status,
					JoinedAt:    parsedJoinedAt,
				}
				memberUserID = rpcRes.MemberID
			}
		} else {
			lastErr = err
			log.Printf("[RegisterMember] CallRPC create_agency_member_user: %v", err)
		}

		// 2. Fallback: AdminCreateUser jika secret key tersedia
		if newMember == nil && s.sbClient.HasSecretKey() {
			userMetadata := map[string]interface{}{
				"name":          name,
				"phone":         cleanPhone,
				"business_name": bName,
				"upline_id":     agentID,
				"role":          "user",
			}
			createUserPayload := map[string]interface{}{
				"email":         email,
				"password":      password,
				"email_confirm": true,
				"user_metadata": userMetadata,
			}
			newUser, err := s.sbClient.AdminCreateUser(createUserPayload)
			if err == nil && newUser != nil {
				memberUserID = newUser.ID
				record := map[string]interface{}{
					"agent_id":     agentID,
					"member_id":    memberUserID,
					"member_name":  name,
					"member_phone": cleanPhone,
					"status":       "ACTIVE",
				}
				if respBytes, err := s.sbClient.PostDB("agency_members", record, "return=representation", ""); err == nil {
					var inserted []*AgencyMember
					if err := json.Unmarshal(respBytes, &inserted); err == nil && len(inserted) > 0 {
						newMember = inserted[0]
					}
				}
			}
		}
	}

	// Jika masih gagal ke database, tolak pendaftaran dan kembalikan error jelas
	if newMember == nil || memberUserID == "" {
		errMsg := "Gagal menyimpan akun ke database."
		if lastErr != nil {
			errMsg = fmt.Sprintf("Gagal menyimpan akun ke database: %v. Pastikan migration 26 sudah dijalankan di Supabase.", lastErr)
		}
		return nil, errors.New(errMsg)
	}

	// Simpan custom markups jika ada
	if len(req.Markups) > 0 {
		_ = s.UpdateMemberMarkups(agentID, memberUserID, req.Markups)
	}

	// Update local cache dari data DB yang valid
	s.mu.Lock()
	s.memberUpline[memberUserID] = agentID
	s.membersByAgent[agentID] = append([]*AgencyMember{newMember}, s.membersByAgent[agentID]...)
	s.mu.Unlock()

	log.Printf("✅ [AGENCY] Anggota baru %s (%s) BERHASIL TERSIMPAN DI DATABASE oleh agen %s", name, cleanPhone, agentID)
	return newMember, nil
}

func (s *service) GetMemberMarkups(agentID, memberID string) ([]*AgentCategoryMarkup, error) {
	markups := make([]*AgentCategoryMarkup, 0)

	s.mu.RLock()
	for _, mk := range s.markups {
		if mk.AgentID == agentID && mk.MemberID != nil && *mk.MemberID == memberID {
			markups = append(markups, mk)
		}
	}
	s.mu.RUnlock()

	if len(markups) > 0 {
		return markups, nil
	}

	if s.sbClient != nil {
		filter := fmt.Sprintf("agent_id=eq.%s&member_id=eq.%s", agentID, memberID)
		if respBytes, err := s.sbClient.GetDB("agent_category_markups", "select=*&"+filter, ""); err == nil {
			var list []*AgentCategoryMarkup
			if err := json.Unmarshal(respBytes, &list); err == nil {
				s.mu.Lock()
				for _, mk := range list {
					key := fmt.Sprintf("%s:%s:%s", agentID, memberID, mk.CategoryID)
					s.markups[key] = mk
				}
				s.mu.Unlock()
				return list, nil
			}
		}
	}

	// Fallback to default markups of the agent
	return s.GetDefaultMarkups(agentID)
}

func (s *service) UpdateMemberMarkups(agentID, memberID string, markups []CategoryMarkupInput) error {
	for _, m := range markups {
		if strings.TrimSpace(m.CategoryID) == "" {
			continue
		}
		catID := strings.TrimSpace(m.CategoryID)
		markupAmt := m.MarkupAmount
		if markupAmt < 0 {
			markupAmt = 0
		}

		key := fmt.Sprintf("%s:%s:%s", agentID, memberID, catID)
		mID := memberID

		item := &AgentCategoryMarkup{
			ID:           fmt.Sprintf("mk-%d", time.Now().UnixNano()),
			AgentID:      agentID,
			MemberID:     &mID,
			CategoryID:   catID,
			CategoryName: m.CategoryName,
			MarkupAmount: markupAmt,
			UpdatedAt:    time.Now(),
		}

		s.mu.Lock()
		s.markups[key] = item
		s.mu.Unlock()

		if s.sbClient != nil {
			record := map[string]interface{}{
				"agent_id":      agentID,
				"member_id":     memberID,
				"category_id":   catID,
				"markup_amount": markupAmt,
				"updated_at":    time.Now(),
			}
			// Upsert to agent_category_markups
			_, _ = s.sbClient.PostDB("agent_category_markups", record, "resolution=merge-duplicates", "")
		}
	}
	return nil
}

func (s *service) GetDefaultMarkups(agentID string) ([]*AgentCategoryMarkup, error) {
	markups := make([]*AgentCategoryMarkup, 0)

	s.mu.RLock()
	for _, mk := range s.markups {
		if mk.AgentID == agentID && (mk.MemberID == nil || *mk.MemberID == "") {
			markups = append(markups, mk)
		}
	}
	s.mu.RUnlock()

	if len(markups) > 0 {
		return markups, nil
	}

	if s.sbClient != nil {
		filter := fmt.Sprintf("agent_id=eq.%s&member_id=is.null", agentID)
		if respBytes, err := s.sbClient.GetDB("agent_category_markups", "select=*&"+filter, ""); err == nil {
			var list []*AgentCategoryMarkup
			if err := json.Unmarshal(respBytes, &list); err == nil {
				s.mu.Lock()
				for _, mk := range list {
					key := fmt.Sprintf("%s::%s", agentID, mk.CategoryID)
					s.markups[key] = mk
				}
				s.mu.Unlock()
				return list, nil
			}
		}
	}

	return markups, nil
}

func (s *service) UpdateDefaultMarkups(agentID string, markups []CategoryMarkupInput) error {
	for _, m := range markups {
		if strings.TrimSpace(m.CategoryID) == "" {
			continue
		}
		catID := strings.TrimSpace(m.CategoryID)
		markupAmt := m.MarkupAmount
		if markupAmt < 0 {
			markupAmt = 0
		}

		key := fmt.Sprintf("%s::%s", agentID, catID)
		item := &AgentCategoryMarkup{
			ID:           fmt.Sprintf("mk-%d", time.Now().UnixNano()),
			AgentID:      agentID,
			MemberID:     nil,
			CategoryID:   catID,
			CategoryName: m.CategoryName,
			MarkupAmount: markupAmt,
			UpdatedAt:    time.Now(),
		}

		s.mu.Lock()
		s.markups[key] = item
		s.mu.Unlock()

		if s.sbClient != nil {
			record := map[string]interface{}{
				"agent_id":      agentID,
				"member_id":     nil,
				"category_id":   catID,
				"markup_amount": markupAmt,
				"updated_at":    time.Now(),
			}
			_, _ = s.sbClient.PostDB("agent_category_markups", record, "resolution=merge-duplicates", "")
		}
	}
	return nil
}

func (s *service) CopyMarkups(agentID string, req CopyMarkupsRequest) error {
	fromMarkups, err := s.GetMemberMarkups(agentID, req.FromMemberID)
	if err != nil || len(fromMarkups) == 0 {
		return errors.New("Markup dari anggota sumber tidak ditemukan")
	}

	inputs := make([]CategoryMarkupInput, 0)
	for _, fm := range fromMarkups {
		inputs = append(inputs, CategoryMarkupInput{
			CategoryID:   fm.CategoryID,
			CategoryName: fm.CategoryName,
			MarkupAmount: fm.MarkupAmount,
		})
	}

	for _, targetMemberID := range req.ToMemberIDs {
		targetID := strings.TrimSpace(targetMemberID)
		if targetID != "" {
			_ = s.UpdateMemberMarkups(agentID, targetID, inputs)
		}
	}

	return nil
}

func (s *service) GetUplineMarkupForUser(userID string) (map[string]float64, string, error) {
	if userID == "" {
		return nil, "", nil
	}

	s.mu.RLock()
	uplineID := s.memberUpline[userID]
	s.mu.RUnlock()

	if uplineID == "" && s.sbClient != nil {
		filter := fmt.Sprintf("member_id=eq.%s", userID)
		if respBytes, err := s.sbClient.GetDB("agency_members", "select=agent_id&"+filter, ""); err == nil {
			var list []struct {
				AgentID string `json:"agent_id"`
			}
			if err := json.Unmarshal(respBytes, &list); err == nil && len(list) > 0 {
				uplineID = list[0].AgentID
				s.mu.Lock()
				s.memberUpline[userID] = uplineID
				s.mu.Unlock()
			}
		}
	}

	if uplineID == "" {
		return nil, "", nil
	}

	// Fetch member markups
	memberMarkups, _ := s.GetMemberMarkups(uplineID, userID)
	markupMap := make(map[string]float64)

	for _, m := range memberMarkups {
		markupMap[m.CategoryID] = m.MarkupAmount
		if m.CategoryName != "" {
			markupMap[strings.ToLower(m.CategoryName)] = m.MarkupAmount
		}
	}

	return markupMap, uplineID, nil
}

func (s *service) ListCommissions(agentID string) ([]*wallet.WalletTransaction, error) {
	if s.walletService == nil {
		return []*wallet.WalletTransaction{}, nil
	}

	allTrx := s.walletService.GetAllTransactions()
	commissions := make([]*wallet.WalletTransaction, 0)

	for _, t := range allTrx {
		if t != nil && t.UserID == agentID && t.Category == wallet.CategoryCommission {
			commissions = append(commissions, t)
		}
	}

	// Also query DB if empty
	if len(commissions) == 0 && s.sbClient != nil {
		filter := fmt.Sprintf("user_id=eq.%s&category=eq.COMMISSION&order=created_at.desc", agentID)
		if respBytes, err := s.sbClient.GetDB("app_wallet_transactions", "select=*&"+filter, ""); err == nil {
			var list []*wallet.WalletTransaction
			if err := json.Unmarshal(respBytes, &list); err == nil {
				return list, nil
			}
		}
	}

	return commissions, nil
}
