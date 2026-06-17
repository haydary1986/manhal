package store

import (
	"context"
	"testing"

	"github.com/erticaz/manhal/internal/domain"
)

func TestMemoryGiftCodes(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	_ = m.AddGiftCode(ctx, domain.GiftCode{Code: "ABC", Tier: domain.TierResearcher, Days: 30})

	if codes, _ := m.ListGiftCodes(ctx); len(codes) != 1 {
		t.Fatalf("list = %d, want 1", len(codes))
	}

	// Unknown code.
	if _, err := m.RedeemGiftCode(ctx, "NOPE", 1); err != ErrNotFound {
		t.Errorf("unknown => %v, want ErrNotFound", err)
	}

	// First redemption succeeds and carries the tier.
	g, err := m.RedeemGiftCode(ctx, "ABC", 7)
	if err != nil || g.Tier != domain.TierResearcher || g.RedeemedBy != 7 {
		t.Fatalf("redeem = (%+v, %v)", g, err)
	}

	// Second redemption is rejected.
	if _, err := m.RedeemGiftCode(ctx, "ABC", 8); err != ErrCodeUsed {
		t.Errorf("reuse => %v, want ErrCodeUsed", err)
	}
}
