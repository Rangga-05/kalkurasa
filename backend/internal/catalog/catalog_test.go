package catalog

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"kalkurasa/backend/internal/auth"
)

func TestIngredientStripBuyPriceForStaff(t *testing.T) {
	bp := decimal.NewFromInt(275000)
	ing := Ingredient{
		ID:          "11111111-1111-1111-1111-111111111111",
		Name:        "Tepung Terigu",
		UseUnit:     "kg",
		BuyUnit:     "sak",
		BuyUnitSize: decimal.NewFromInt(25),
		BuyPrice:    &bp,
		Stock:       decimal.NewFromInt(3),
		MinStock:    decimal.NewFromInt(5),
		Rounding:    "loose",
		IsActive:    true,
	}

	// Untuk manager, buy_price harus muncul di JSON
	mgrCopy := ing
	mgrCopy.stripForRole(auth.RoleManager)
	bMgr, err := json.Marshal(mgrCopy)
	if err != nil {
		t.Fatalf("marshal manager ingredient: %v", err)
	}
	if !strings.Contains(string(bMgr), `"buy_price"`) {
		t.Errorf("expected buy_price in manager JSON, got: %s", string(bMgr))
	}

	// Untuk staff, buy_price tidak boleh muncul di JSON
	staffCopy := ing
	staffCopy.stripForRole(auth.RoleStaff)
	bStaff, err := json.Marshal(staffCopy)
	if err != nil {
		t.Fatalf("marshal staff ingredient: %v", err)
	}
	if strings.Contains(string(bStaff), `"buy_price"`) {
		t.Errorf("expected buy_price to be omitted for staff, got: %s", string(bStaff))
	}
}

