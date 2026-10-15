package product

import (
	"testing"
	"time"
)

func TestProductCatalogAndCache(t *testing.T) {
	repo := NewCachedRepository(1 * time.Minute)
	repo.Save(&Product{
		ID:           "test-1",
		CategoryID:   "cat-pulsa",
		Category:     CategoryPulsa,
		VendorCode:   "MANUAL",
		BuyerSKUCode: "TSEL10",
		Code:         "TSEL10",
		Name:         "Telkomsel 10.000",
		Brand:        "Telkomsel",
		CostPrice:    10200,
		Price:        11200,
		Status:       "ACTIVE",
	})
	svc := NewService(repo, nil)

	// Test list all products
	all := svc.ListProducts("", "", "")
	if len(all) == 0 {
		t.Fatalf("expected products to be loaded, got %d", len(all))
	}

	// Test category filtering
	pulsaList := svc.ListProducts("pulsa", "", "")
	if len(pulsaList) == 0 {
		t.Fatalf("expected pulsa products, got 0")
	}
	for _, p := range pulsaList {
		if p.Category != "pulsa" {
			t.Errorf("expected category 'pulsa', got '%s'", p.Category)
		}
	}

	// Test detail by code
	prod, err := svc.GetProduct("TSEL10")
	if err != nil {
		t.Fatalf("expected TSEL10 to exist, got error: %v", err)
	}
	if prod.Code != "TSEL10" {
		t.Errorf("expected code TSEL10, got %s", prod.Code)
	}

	// Test category listing
	cats := svc.GetCategories()
	if len(cats) == 0 {
		t.Errorf("expected non-empty categories")
	}

	// Test brand listing
	brands := svc.GetBrands("pulsa")
	if len(brands) == 0 {
		t.Errorf("expected brands for pulsa")
	}
}
