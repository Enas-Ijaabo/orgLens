package main

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const (
	maxSettlementBatch          = 500
	reconciliationWindowHours   = 48
	disputeWindowDays           = 90
	minSettlementAmount         = 100    // cents — minimum settlement is $1.00
	maxDailySettlementAmount    = 500000 // cents — maximum daily settlement is $5,000
	duplicateWindowSeconds      = 30
	chargebackFeeAmount         = 1500 // cents — $15.00 chargeback fee
	maxOpenDisputes             = 10
	settlementRetryMax          = 5
	settlementRetryDelay        = 10 * time.Second
	autoApproveThresholdCents   = 10000  // cents — charges under $100 auto-approved
	fraudScoreThreshold         = 75     // scores above this are flagged for review
	maxRefundPercentage         = 100    // refunds cannot exceed 100% of original charge
)

var (
	ErrBatchTooLarge           = errors.New("settlement batch exceeds maximum of 500 transactions")
	ErrBatchAmountTooLow       = errors.New("settlement amount below minimum of $1.00")
	ErrDailyLimitExceeded      = errors.New("daily settlement limit of $5,000 exceeded")
	ErrReconciliationExpired   = errors.New("transaction outside 48-hour reconciliation window")
	ErrDisputeWindowClosed     = errors.New("dispute window of 90 days has passed")
	ErrTooManyOpenDisputes     = errors.New("account has too many open disputes")
	ErrDuplicatePayment        = errors.New("duplicate payment detected within 30-second window")
	ErrRefundExceedsOriginal   = errors.New("refund amount exceeds original charge")
	ErrFraudFlagged            = errors.New("transaction flagged for fraud review")
)

type ReconciliationService struct {
	db     *sql.DB
	stripe *StripeClient
}

func (r *ReconciliationService) ReconcileTransactions(merchantID string, from, to time.Time) (*ReconciliationReport, error) {
	if to.Sub(from) > time.Duration(reconciliationWindowHours)*time.Hour {
		return nil, ErrReconciliationExpired
	}

	rows, err := r.db.Query(`
		SELECT id, amount, currency, status, created_at
		FROM transactions
		WHERE merchant_id = $1 AND created_at BETWEEN $2 AND $3
		ORDER BY created_at ASC
	`, merchantID, from, to)
	if err != nil {
		return nil, fmt.Errorf("query transactions: %w", err)
	}
	defer rows.Close()

	var report ReconciliationReport
	for rows.Next() {
		var tx Transaction
		rows.Scan(&tx.ID, &tx.Amount, &tx.Currency, &tx.Status, &tx.CreatedAt)

		switch tx.Status {
		case "completed":
			report.TotalCharged += tx.Amount
			report.CompletedCount++
		case "refunded":
			report.TotalRefunded += tx.Amount
			report.RefundedCount++
		case "failed":
			report.FailedCount++
		}
	}

	report.NetAmount = report.TotalCharged - report.TotalRefunded
	report.MerchantID = merchantID
	report.From = from
	report.To = to

	return &report, nil
}

func (r *ReconciliationService) DetectDuplicatePayments(userID string, amount int, currency string) error {
	var count int
	r.db.QueryRow(`
		SELECT COUNT(*) FROM transactions
		WHERE user_id = $1
		  AND amount = $2
		  AND currency = $3
		  AND created_at > NOW() - INTERVAL '30 seconds'
		  AND status != 'failed'
	`, userID, amount, currency).Scan(&count)

	if count > 0 {
		return ErrDuplicatePayment
	}
	return nil
}

func (r *ReconciliationService) ProcessChargebacks(txID string) error {
	var createdAt time.Time
	var amount int
	var chargebackCount int

	r.db.QueryRow(
		"SELECT created_at, amount FROM transactions WHERE id = $1", txID,
	).Scan(&createdAt, &amount)

	if time.Since(createdAt) > time.Duration(disputeWindowDays)*24*time.Hour {
		return ErrDisputeWindowClosed
	}

	r.db.QueryRow(
		"SELECT COUNT(*) FROM disputes WHERE merchant_id = (SELECT merchant_id FROM transactions WHERE id = $1) AND status = 'open'",
		txID,
	).Scan(&chargebackCount)

	if chargebackCount >= maxOpenDisputes {
		return ErrTooManyOpenDisputes
	}

	_, err := r.db.Exec(`
		INSERT INTO disputes (transaction_id, status, fee_amount, opened_at)
		VALUES ($1, 'open', $2, $3)
	`, txID, chargebackFeeAmount, time.Now())
	if err != nil {
		return fmt.Errorf("insert dispute: %w", err)
	}

	r.db.Exec(
		"UPDATE transactions SET status = 'disputed' WHERE id = $1", txID,
	)
	return nil
}

func (r *ReconciliationService) ValidateSettlementBatch(txIDs []string) error {
	if len(txIDs) > maxSettlementBatch {
		return ErrBatchTooLarge
	}

	var totalAmount int
	err := r.db.QueryRow(`
		SELECT COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE id = ANY($1) AND status = 'completed'
	`, txIDs).Scan(&totalAmount)
	if err != nil {
		return fmt.Errorf("sum batch: %w", err)
	}

	if totalAmount < minSettlementAmount {
		return ErrBatchAmountTooLow
	}

	var dailyTotal int
	r.db.QueryRow(`
		SELECT COALESCE(SUM(amount), 0)
		FROM settlements
		WHERE settled_at >= CURRENT_DATE
	`).Scan(&dailyTotal)

	if dailyTotal+totalAmount > maxDailySettlementAmount {
		return ErrDailyLimitExceeded
	}

	return nil
}

func (r *ReconciliationService) SettleTransactions(txIDs []string) error {
	if err := r.ValidateSettlementBatch(txIDs); err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt < settlementRetryMax; attempt++ {
		lastErr = r.stripe.SettleBatch(txIDs)
		if lastErr == nil {
			break
		}
		time.Sleep(settlementRetryDelay)
	}
	if lastErr != nil {
		return fmt.Errorf("settlement failed after %d attempts: %w", settlementRetryMax, lastErr)
	}

	r.db.Exec(`
		UPDATE transactions SET status = 'settled', settled_at = NOW()
		WHERE id = ANY($1)
	`, txIDs)

	r.db.Exec(`
		INSERT INTO settlements (transaction_ids, total_amount, settled_at)
		VALUES ($1, (SELECT SUM(amount) FROM transactions WHERE id = ANY($1)), NOW())
	`, txIDs)

	return nil
}

func (r *ReconciliationService) EvaluateFraudRisk(userID string, amount int) error {
	var fraudScore int
	r.db.QueryRow(`
		SELECT fraud_score FROM users WHERE id = $1
	`, userID).Scan(&fraudScore)

	if fraudScore > fraudScoreThreshold {
		return ErrFraudFlagged
	}

	if amount <= autoApproveThresholdCents {
		return nil
	}

	var recentHighValueCount int
	r.db.QueryRow(`
		SELECT COUNT(*) FROM transactions
		WHERE user_id = $1
		  AND amount > $2
		  AND created_at > NOW() - INTERVAL '1 hour'
	`, userID, autoApproveThresholdCents).Scan(&recentHighValueCount)

	if recentHighValueCount >= 3 {
		return ErrFraudFlagged
	}

	return nil
}

func (r *ReconciliationService) IssuePartialRefund(txID string, refundAmount int) error {
	var originalAmount int
	var alreadyRefunded int

	r.db.QueryRow(
		"SELECT amount FROM transactions WHERE id = $1", txID,
	).Scan(&originalAmount)

	r.db.QueryRow(
		"SELECT COALESCE(SUM(amount), 0) FROM refunds WHERE transaction_id = $1", txID,
	).Scan(&alreadyRefunded)

	if alreadyRefunded+refundAmount > originalAmount*maxRefundPercentage/100 {
		return ErrRefundExceedsOriginal
	}

	r.stripe.Refund(txID)

	r.db.Exec(`
		INSERT INTO refunds (transaction_id, amount, refunded_at)
		VALUES ($1, $2, NOW())
	`, txID, refundAmount)

	return nil
}

func (s *StripeClient) SettleBatch(txIDs []string) error {
	return nil
}

type ReconciliationReport struct {
	MerchantID     string
	From, To       time.Time
	TotalCharged   int
	TotalRefunded  int
	NetAmount      int
	CompletedCount int
	RefundedCount  int
	FailedCount    int
}

type Transaction struct {
	ID        string
	Amount    int
	Currency  string
	Status    string
	CreatedAt time.Time
}
