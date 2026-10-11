package announcement

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type PaymentEvent struct {
	PaymentID string `json:"payment_id"`
	AccountID string `json:"account_id"`
	Status    string `json:"status"`
	RiskLevel string `json:"risk_level"`
	Amount    int64  `json:"amount_minor"`
	Currency  string `json:"currency"`
}

type Decision struct {
	Broadcast bool   `json:"broadcast"`
	Reason    string `json:"reason"`
}

type Notice struct {
	PaymentID string `json:"payment_id"`
	Status    string `json:"status"`
	Amount    int64  `json:"amount_minor"`
	Currency  string `json:"currency"`
	Occurred  string `json:"occurred_at"`
}

func Decide(event PaymentEvent) (Decision, error) {
	if strings.TrimSpace(event.PaymentID) == "" || strings.TrimSpace(event.AccountID) == "" {
		return Decision{}, errors.New("payment_id and account_id are required")
	}
	if event.Amount <= 0 || len(event.Currency) != 3 {
		return Decision{}, errors.New("amount_minor must be positive and currency must have three letters")
	}
	if event.Status != "settled" {
		return Decision{Broadcast: false, Reason: "payment_not_settled"}, nil
	}
	if event.RiskLevel != "low" {
		return Decision{Broadcast: false, Reason: "risk_review_required"}, nil
	}
	return Decision{Broadcast: true, Reason: "eligible_for_member_notice"}, nil
}

func BuildNotice(event PaymentEvent, now time.Time) Notice {
	return Notice{
		PaymentID: event.PaymentID,
		Status:    event.Status,
		Amount:    event.Amount,
		Currency:  strings.ToUpper(event.Currency),
		Occurred:  now.UTC().Format(time.RFC3339),
	}
}

func IdempotencyKey(event PaymentEvent) string {
	return fmt.Sprintf("payment-%s-%s", event.PaymentID, event.Status)
}
