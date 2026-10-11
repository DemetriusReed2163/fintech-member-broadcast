package announcement

import "testing"

func TestDecidePaymentBroadcast(t *testing.T) {
	tests := []struct {
		name      string
		event     PaymentEvent
		broadcast bool
		reason    string
		wantErr   bool
	}{
		{
			name:      "settled low risk payment is announced",
			event:     PaymentEvent{PaymentID: "pay_101", AccountID: "acct_7", Status: "settled", RiskLevel: "low", Amount: 2599, Currency: "usd"},
			broadcast: true,
			reason:    "eligible_for_member_notice",
		},
		{
			name:   "elevated risk is held for review",
			event:  PaymentEvent{PaymentID: "pay_102", AccountID: "acct_7", Status: "settled", RiskLevel: "high", Amount: 2599, Currency: "usd"},
			reason: "risk_review_required",
		},
		{
			name:   "pending payment is not announced",
			event:  PaymentEvent{PaymentID: "pay_103", AccountID: "acct_7", Status: "pending", RiskLevel: "low", Amount: 2599, Currency: "usd"},
			reason: "payment_not_settled",
		},
		{
			name:    "missing audit identity is invalid",
			event:   PaymentEvent{Status: "settled", RiskLevel: "low", Amount: 2599, Currency: "usd"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decide(tt.event)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decide() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && (got.Broadcast != tt.broadcast || got.Reason != tt.reason) {
				t.Fatalf("Decide() = %+v, want broadcast=%v reason=%q", got, tt.broadcast, tt.reason)
			}
		})
	}
}
