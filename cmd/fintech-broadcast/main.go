package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/infrai-examples/fintech-member-broadcast/internal/announcement"
	realtimeapi "github.com/infrai-examples/fintech-member-broadcast/internal/infrai"
)

type service struct {
	publisher *realtimeapi.Publisher
	channel   string
	now       func() time.Time
	log       *slog.Logger
}

type response struct {
	PaymentID string                `json:"payment_id"`
	Decision  announcement.Decision `json:"decision"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	publisher, err := realtimeapi.NewPublisher(os.Getenv("INFRAI_API_KEY"), nil)
	if err != nil {
		logger.Error("configuration rejected", "error", err)
		os.Exit(1)
	}
	channel := os.Getenv("BROADCAST_CHANNEL")
	if channel == "" {
		channel = "fintech-members"
	}
	address := os.Getenv("LISTEN_ADDR")
	if address == "" {
		address = ":8080"
	}

	svc := &service{publisher: publisher, channel: channel, now: time.Now, log: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /payment-events", svc.handlePaymentEvent)
	server := &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	logger.Info("broadcast service listening", "address", address, "channel", channel)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func (s *service) handlePaymentEvent(w http.ResponseWriter, r *http.Request) {
	var event announcement.PaymentEvent
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payment event"})
		return
	}
	decision, err := announcement.Decide(event)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !decision.Broadcast {
		s.log.Info("payment event held", "payment_id", event.PaymentID, "account_id", event.AccountID, "reason", decision.Reason)
		writeJSON(w, http.StatusAccepted, response{PaymentID: event.PaymentID, Decision: decision})
		return
	}

	notice := announcement.BuildNotice(event, s.now())
	err = s.publisher.Publish(r.Context(), realtimeapi.PublishRequest{
		Channel:   s.channel,
		Event:     "payment.settled",
		Data:      notice,
		AccountID: event.AccountID,
	}, announcement.IdempotencyKey(event))
	if err != nil {
		status := http.StatusBadGateway
		var apiErr *realtimeapi.APIError
		if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
			status = apiErr.Status
		}
		s.log.Error("broadcast rejected", "payment_id", event.PaymentID, "account_id", event.AccountID, "error", err)
		writeJSON(w, status, map[string]string{"error": "announcement was not accepted"})
		return
	}
	s.log.Info("payment announcement published", "payment_id", event.PaymentID, "account_id", event.AccountID, "channel", s.channel)
	writeJSON(w, http.StatusAccepted, response{PaymentID: event.PaymentID, Decision: decision})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
