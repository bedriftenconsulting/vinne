package orange

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/randco/service-payment/internal/providers"
)

// VerifyWallet verifies a mobile money wallet
func (p *Provider) VerifyWallet(ctx context.Context, req *providers.VerifyWalletRequest) (*providers.VerifyWalletResponse, error) {
	ctx, span := tracer.Start(ctx, "orange.provider.verify_wallet",
		trace.WithAttributes(
			attribute.String("wallet_number", req.WalletNumber),
			attribute.String("wallet_provider", req.WalletProvider),
			attribute.String("reference", req.Reference),
		))
	defer span.End()

	// Normalize provider name to Orange API format (MTN, TELECEL, AIRTELTIGO)
	normalizedProvider := normalizeProviderName(req.WalletProvider)

	// Prepare request body (matching Orange Extensibility API contract)
	body := map[string]interface{}{
		"walletNumber":   req.WalletNumber,
		"walletProvider": normalizedProvider, // MTN, TELECEL, AIRTELTIGO
		"reference":      req.Reference,
	}

	p.log.Info("Wallet verification request to Orange API",
		"wallet_number", req.WalletNumber,
		"wallet_provider_original", req.WalletProvider,
		"wallet_provider_normalized", normalizedProvider,
		"reference", req.Reference,
		"request_body", body)

	// API response structure (from Orange Extensibility API documentation page 6)
	var apiResp struct {
		Status  int    `json:"status"` // 1=success, 0=invalid, 2=duplicate, 3=pending
		Message string `json:"message"`
		Data    struct {
			Name string `json:"name"` // Account name associated with the wallet
		} `json:"data"`
		TimeStamp string `json:"timeStamp"`
	}

	// Make authenticated request (Orange Extensibility API: Enquiry/wallet)
	if err := p.makeAuthenticatedRequest(ctx, "POST", "/Enquiry/wallet", body, &apiResp); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("wallet verification failed: %w", err)
	}

	// Determine if wallet is valid based on status code
	isValid := apiResp.Status == 1

	span.SetAttributes(
		attribute.Bool("is_valid", isValid),
		attribute.String("account_name", apiResp.Data.Name),
		attribute.Int("orange_status_code", apiResp.Status),
	)

	p.log.Info("Wallet verification response from Orange API",
		"status_code", apiResp.Status,
		"message", apiResp.Message,
		"account_name", apiResp.Data.Name,
		"is_valid", isValid)

	// Map to standard response
	return &providers.VerifyWalletResponse{
		IsValid:        isValid,
		AccountName:    apiResp.Data.Name,
		WalletNumber:   req.WalletNumber,   // Echo back from request
		WalletProvider: req.WalletProvider, // Echo back from request
		Reference:      req.Reference,
		ProviderData: map[string]interface{}{
			"status":    apiResp.Status,
			"message":   apiResp.Message,
			"timestamp": apiResp.TimeStamp,
		},
	}, nil
}

// VerifyBankAccount verifies a bank account
func (p *Provider) VerifyBankAccount(ctx context.Context, req *providers.VerifyBankAccountRequest) (*providers.VerifyBankAccountResponse, error) {
	ctx, span := tracer.Start(ctx, "orange.provider.verify_bank_account",
		trace.WithAttributes(
			attribute.String("account_number", req.AccountNumber),
			attribute.String("bank_code", req.BankCode),
			attribute.String("reference", req.Reference),
		))
	defer span.End()

	// Prepare request body
	body := map[string]interface{}{
		"account_number": req.AccountNumber,
		"bank_code":      req.BankCode,
		"reference":      req.Reference,
	}

	// API response structure
	var apiResp struct {
		Status  int    `json:"status"` // 1=success, 0=invalid, 2=duplicate, 3=pending
		Message string `json:"message"`
		Data    struct {
			AccountName   string `json:"account_name"`
			AccountNumber string `json:"account_number"`
			BankCode      string `json:"bank_code"`
			BankName      string `json:"bank_name"`
			IsValid       bool   `json:"is_valid"`
		} `json:"data"`
	}

	// Make authenticated request
	if err := p.makeAuthenticatedRequest(ctx, "POST", "/api/v1/enquiry/bank-account", body, &apiResp); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("bank account verification failed: %w", err)
	}

	span.SetAttributes(
		attribute.Bool("is_valid", apiResp.Data.IsValid),
		attribute.String("account_name", apiResp.Data.AccountName),
		attribute.String("bank_name", apiResp.Data.BankName),
	)

	// Map to standard response
	return &providers.VerifyBankAccountResponse{
		IsValid:       apiResp.Data.IsValid,
		AccountName:   apiResp.Data.AccountName,
		AccountNumber: apiResp.Data.AccountNumber,
		BankCode:      apiResp.Data.BankCode,
		BankName:      apiResp.Data.BankName,
		Reference:     req.Reference,
		ProviderData: map[string]interface{}{
			"status":  apiResp.Status,
			"message": apiResp.Message,
		},
	}, nil
}

// VerifyIdentity verifies an identity (KYC)
func (p *Provider) VerifyIdentity(ctx context.Context, req *providers.VerifyIdentityRequest) (*providers.VerifyIdentityResponse, error) {
	ctx, span := tracer.Start(ctx, "orange.provider.verify_identity",
		trace.WithAttributes(
			attribute.String("identity_type", req.IdentityType),
			attribute.String("identity_number", req.IdentityNumber),
			attribute.Bool("full_kyc", req.FullKYC),
			attribute.String("reference", req.Reference),
		))
	defer span.End()

	// Prepare request body
	body := map[string]interface{}{
		"identity_type":   req.IdentityType,
		"identity_number": req.IdentityNumber,
		"full_kyc":        req.FullKYC,
		"reference":       req.Reference,
	}

	// API response structure
	var apiResp struct {
		Status  int    `json:"status"` // 1=success, 0=invalid, 2=duplicate, 3=pending
		Message string `json:"message"`
		Data    struct {
			IsValid        bool                   `json:"is_valid"`
			FullName       string                 `json:"full_name"`
			DateOfBirth    *time.Time             `json:"date_of_birth"`
			Nationality    string                 `json:"nationality"`
			IdentityNumber string                 `json:"identity_number"`
			CardValidFrom  *time.Time             `json:"card_valid_from"`
			CardValidTo    *time.Time             `json:"card_valid_to"`
			FullKYCData    map[string]interface{} `json:"full_kyc_data,omitempty"`
		} `json:"data"`
	}

	// Make authenticated request
	if err := p.makeAuthenticatedRequest(ctx, "POST", "/api/v1/enquiry/identity", body, &apiResp); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("identity verification failed: %w", err)
	}

	span.SetAttributes(
		attribute.Bool("is_valid", apiResp.Data.IsValid),
		attribute.String("full_name", apiResp.Data.FullName),
	)

	// Map to standard response
	response := &providers.VerifyIdentityResponse{
		IsValid:        apiResp.Data.IsValid,
		FullName:       apiResp.Data.FullName,
		DateOfBirth:    apiResp.Data.DateOfBirth,
		Nationality:    apiResp.Data.Nationality,
		IdentityNumber: apiResp.Data.IdentityNumber,
		CardValidFrom:  apiResp.Data.CardValidFrom,
		CardValidTo:    apiResp.Data.CardValidTo,
		Reference:      req.Reference,
		ProviderData: map[string]interface{}{
			"status":  apiResp.Status,
			"message": apiResp.Message,
		},
	}

	// Include full KYC data if requested
	if req.FullKYC && apiResp.Data.FullKYCData != nil {
		for k, v := range apiResp.Data.FullKYCData {
			response.ProviderData[k] = v
		}
	}

	return response, nil
}

// DebitWallet debits a mobile money wallet
func (p *Provider) DebitWallet(ctx context.Context, req *providers.DebitWalletRequest) (*providers.TransactionResponse, error) {
	ctx, span := tracer.Start(ctx, "orange.provider.debit_wallet",
		trace.WithAttributes(
			attribute.String("wallet_number", req.WalletNumber),
			attribute.String("wallet_provider", req.WalletProvider),
			attribute.Float64("amount", req.Amount),
			attribute.String("currency", req.Currency),
			attribute.String("reference", req.Reference),
		))
	defer span.End()

	// Normalize provider name to Orange API format (MTN, TELECEL, AIRTELTIGO)
	normalizedProvider := normalizeProviderName(req.WalletProvider)

	p.log.Info("Initiating wallet debit via Orange API",
		"wallet_number", req.WalletNumber,
		"wallet_provider_original", req.WalletProvider,
		"wallet_provider_normalized", normalizedProvider,
		"amount_pesewas", req.Amount,
		"amount_ghs", req.Amount/100.0,
		"reference", req.Reference,
		"customer_name", req.CustomerName)

	// Prepare request body (matching Orange Extensibility API contract)
	body := map[string]interface{}{
		"walletNumber":    req.WalletNumber,
		"walletProvider":  normalizedProvider, // MTN, TELECEL, AIRTELTIGO
		"amount":          req.Amount / 100.0, // Convert pesewas to cedis
		"narration":       req.Narration,
		"reference":       req.Reference,
		"customerName":    req.CustomerName,
		"customerRemarks": req.CustomerRemarks,
	}

	// Add callback URL if configured (for webhook notifications)
	if p.config.CallbackURL != "" {
		body["callbackUrl"] = p.config.CallbackURL
		p.log.Debug("Adding callback URL to wallet debit request",
			"callback_url", p.config.CallbackURL,
			"reference", req.Reference)
	}

	p.log.Debug("Wallet debit request body prepared",
		"endpoint", "/Transfer/walletDebit",
		"body", body)

	// API response structure (from Orange Extensibility API documentation)
	var apiResp struct {
		Status  int    `json:"status"` // 1=success, 0=failed, 2=duplicate, 3=pending
		Message string `json:"message"`
		Data    struct {
			TransactionID string     `json:"transactionId"` // Orange uses camelCase
			Reference     string     `json:"reference"`
			Beneficiary   string     `json:"beneficiary"`
			Amount        float64    `json:"amount"` // In cedis
			TransType     string     `json:"transType"`
			RequestDate   *time.Time `json:"requestDate"`
			ApproveDate   *time.Time `json:"approveDate"`
			Narration     string     `json:"narration"`
		} `json:"data"`
		TimeStamp string `json:"timeStamp"`
	}

	// Make authenticated request (Orange Extensibility API: Transfer/walletDebit)
	p.log.Debug("Sending wallet debit request to Orange API",
		"endpoint", "/Transfer/walletDebit",
		"wallet_number", req.WalletNumber,
		"wallet_provider", normalizedProvider,
		"amount_cedis", req.Amount/100.0,
		"reference", req.Reference)

	if err := p.makeAuthenticatedRequest(ctx, "POST", "/Transfer/walletDebit", body, &apiResp); err != nil {
		span.RecordError(err)
		p.log.Error("Wallet debit request failed",
			"error", err,
			"error_type", fmt.Sprintf("%T", err),
			"wallet_number", req.WalletNumber,
			"wallet_provider", req.WalletProvider,
			"amount", req.Amount,
			"reference", req.Reference)
		return nil, fmt.Errorf("wallet debit failed: %w", err)
	}

	p.log.Info("Wallet debit response received from Orange API",
		"status_code", apiResp.Status,
		"message", apiResp.Message,
		"transaction_id", apiResp.Data.TransactionID,
		"amount", apiResp.Data.Amount,
		"reference", apiResp.Data.Reference,
		"trans_type", apiResp.Data.TransType,
		"request_date", apiResp.Data.RequestDate,
		"approve_date", apiResp.Data.ApproveDate)

	// Map Orange status code to transaction status
	p.log.Debug("Mapping Orange status code to transaction status",
		"orange_status_code", apiResp.Status,
		"orange_message", apiResp.Message,
		"reference", req.Reference,
		"provider_transaction_id", apiResp.Data.TransactionID)

	var status providers.TransactionStatus
	switch apiResp.Status {
	case 1:
		status = providers.StatusSuccess
		p.log.Debug("Orange status 1 -> SUCCESS (wallet debit)", "reference", req.Reference)
	case 0:
		status = providers.StatusFailed
		p.log.Debug("Orange status 0 -> FAILED (wallet debit)", "reference", req.Reference)
	case 3:
		status = providers.StatusPending
		p.log.Info("Orange status 3 -> PENDING (wallet debit) - Transaction accepted for processing",
			"reference", req.Reference,
			"provider_transaction_id", apiResp.Data.TransactionID,
			"message", apiResp.Message)
	case 2:
		status = providers.StatusDuplicate
		p.log.Warn("Orange status 2 -> DUPLICATE (wallet debit)", "reference", req.Reference)
	default:
		status = providers.StatusPending
		p.log.Warn("Unknown Orange status code, defaulting to PENDING",
			"orange_status_code", apiResp.Status,
			"reference", req.Reference)
	}

	span.SetAttributes(
		attribute.String("transaction_id", apiResp.Data.TransactionID),
		attribute.String("status", string(status)),
		attribute.Int("orange_status_code", apiResp.Status),
	)

	p.log.Debug("Transaction status mapped successfully",
		"final_status", string(status),
		"success", status == providers.StatusSuccess || status == providers.StatusPending,
		"reference", req.Reference)

	// Determine RequestedAt and CompletedAt
	var requestedAt time.Time
	if apiResp.Data.RequestDate != nil {
		requestedAt = *apiResp.Data.RequestDate
	} else {
		requestedAt = time.Now()
	}

	// Map to standard response
	return &providers.TransactionResponse{
		Success:       status == providers.StatusSuccess || status == providers.StatusPending,
		TransactionID: apiResp.Data.TransactionID,
		Reference:     apiResp.Data.Reference,
		Status:        status,
		Amount:        req.Amount,
		Currency:      req.Currency,
		Beneficiary:   apiResp.Data.Beneficiary,
		RequestedAt:   requestedAt,
		CompletedAt:   apiResp.Data.ApproveDate, // Orange uses ApproveDate for completion
		Message:       apiResp.Message,
		ProviderData: map[string]interface{}{
			"orange_status_code": apiResp.Status,
			"trans_type":         apiResp.Data.TransType,
			"timestamp":          apiResp.TimeStamp,
		},
	}, nil
}

// CreditWallet credits a mobile money wallet
func (p *Provider) CreditWallet(ctx context.Context, req *providers.CreditWalletRequest) (*providers.TransactionResponse, error) {
	ctx, span := tracer.Start(ctx, "orange.provider.credit_wallet",
		trace.WithAttributes(
			attribute.String("wallet_number", req.WalletNumber),
			attribute.String("wallet_provider", req.WalletProvider),
			attribute.Float64("amount", req.Amount),
			attribute.String("currency", req.Currency),
			attribute.String("reference", req.Reference),
		))
	defer span.End()

	// Normalize provider name to Orange API format (MTN, TELECEL, AIRTELTIGO)
	normalizedProvider := normalizeProviderName(req.WalletProvider)

	p.log.Info("Initiating wallet credit via Orange API",
		"wallet_number", req.WalletNumber,
		"wallet_provider_original", req.WalletProvider,
		"wallet_provider_normalized", normalizedProvider,
		"amount_pesewas", req.Amount,
		"amount_ghs", req.Amount/100.0,
		"reference", req.Reference,
		"customer_name", req.CustomerName)

	// Prepare request body (matching Orange Extensibility API contract)
	body := map[string]interface{}{
		"walletNumber":    req.WalletNumber,
		"walletProvider":  normalizedProvider, // MTN, TELECEL, AIRTELTIGO
		"amount":          req.Amount / 100.0, // Convert pesewas to cedis
		"narration":       req.Narration,
		"reference":       req.Reference,
		"customerName":    req.CustomerName,
		"customerRemarks": req.CustomerRemarks,
	}

	// Add callback URL if configured (for webhook notifications)
	if p.config.CallbackURL != "" {
		body["callbackUrl"] = p.config.CallbackURL
		p.log.Debug("Adding callback URL to wallet credit request",
			"callback_url", p.config.CallbackURL,
			"reference", req.Reference)
	}

	// API response structure (from Orange Extensibility API documentation)
	var apiResp struct {
		Status  int    `json:"status"` // 1=success, 0=failed, 2=duplicate, 3=pending
		Message string `json:"message"`
		Data    struct {
			TransactionID string     `json:"transactionId"` // Orange uses camelCase
			Reference     string     `json:"reference"`
			Beneficiary   string     `json:"beneficiary"`
			Amount        float64    `json:"amount"` // In cedis
			TransType     string     `json:"transType"`
			RequestDate   *time.Time `json:"requestDate"`
			ApproveDate   *time.Time `json:"approveDate"`
			Narration     string     `json:"narration"`
		} `json:"data"`
		TimeStamp string `json:"timeStamp"`
	}

	// Make authenticated request (Orange Extensibility API: Transfer/walletCredit)
	if err := p.makeAuthenticatedRequest(ctx, "POST", "/Transfer/walletCredit", body, &apiResp); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("wallet credit failed: %w", err)
	}

	p.log.Info("Wallet credit response received from Orange API",
		"status_code", apiResp.Status,
		"message", apiResp.Message,
		"transaction_id", apiResp.Data.TransactionID,
		"amount", apiResp.Data.Amount,
		"reference", apiResp.Data.Reference,
		"trans_type", apiResp.Data.TransType)

	// Map Orange status code to transaction status
	var status providers.TransactionStatus
	switch apiResp.Status {
	case 1:
		status = providers.StatusSuccess
	case 0:
		status = providers.StatusFailed
	case 3:
		status = providers.StatusPending
	case 2:
		status = providers.StatusDuplicate
	default:
		status = providers.StatusPending
	}

	span.SetAttributes(
		attribute.String("transaction_id", apiResp.Data.TransactionID),
		attribute.String("status", string(status)),
		attribute.Int("orange_status_code", apiResp.Status),
	)

	// Determine RequestedAt and CompletedAt
	var requestedAt time.Time
	if apiResp.Data.RequestDate != nil {
		requestedAt = *apiResp.Data.RequestDate
	} else {
		requestedAt = time.Now()
	}

	// Map to standard response
	return &providers.TransactionResponse{
		Success:       status == providers.StatusSuccess || status == providers.StatusPending,
		TransactionID: apiResp.Data.TransactionID,
		Reference:     apiResp.Data.Reference,
		Status:        status,
		Amount:        req.Amount,
		Currency:      req.Currency,
		Beneficiary:   apiResp.Data.Beneficiary,
		RequestedAt:   requestedAt,
		CompletedAt:   apiResp.Data.ApproveDate,
		Message:       apiResp.Message,
		ProviderData: map[string]interface{}{
			"orange_status_code": apiResp.Status,
			"trans_type":         apiResp.Data.TransType,
			"timestamp":          apiResp.TimeStamp,
		},
	}, nil
}

// TransferToBank transfers funds to a bank account
func (p *Provider) TransferToBank(ctx context.Context, req *providers.BankTransferRequest) (*providers.TransactionResponse, error) {
	ctx, span := tracer.Start(ctx, "orange.provider.transfer_to_bank",
		trace.WithAttributes(
			attribute.String("account_number", req.AccountNumber),
			attribute.String("bank_code", req.BankCode),
			attribute.Float64("amount", req.Amount),
			attribute.String("currency", req.Currency),
			attribute.String("reference", req.Reference),
		))
	defer span.End()

	// Prepare request body
	body := map[string]interface{}{
		"account_number":   req.AccountNumber,
		"bank_code":        req.BankCode,
		"amount":           req.Amount / 100.0, // Convert pesewas to cedis
		"currency":         req.Currency,
		"narration":        req.Narration,
		"reference":        req.Reference,
		"beneficiary_name": req.BeneficiaryName,
		"customer_remarks": req.CustomerRemarks,
		"metadata":         req.Metadata,
	}

	// API response structure
	var apiResp struct {
		Status  int    `json:"status"` // 1=success, 0=failed, 2=duplicate, 3=pending
		Message string `json:"message"`
		Data    struct {
			TransactionID string     `json:"transactionId"` // Orange uses camelCase
			Reference     string     `json:"reference"`
			Beneficiary   string     `json:"beneficiary"`
			Amount        float64    `json:"amount"` // In cedis
			TransType     string     `json:"transType"`
			RequestDate   *time.Time `json:"requestDate"`
			ApproveDate   *time.Time `json:"approveDate"`
			Narration     string     `json:"narration"`
		} `json:"data"`
		TimeStamp string `json:"timeStamp"`
	}

	// Make authenticated request
	if err := p.makeAuthenticatedRequest(ctx, "POST", "/api/v1/transfer/bank", body, &apiResp); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("bank transfer failed: %w", err)
	}

	p.log.Info("Bank transfer response received from Orange API",
		"status_code", apiResp.Status,
		"message", apiResp.Message,
		"transaction_id", apiResp.Data.TransactionID,
		"amount", apiResp.Data.Amount,
		"reference", apiResp.Data.Reference)

	// Map Orange status code to transaction status
	var status providers.TransactionStatus
	switch apiResp.Status {
	case 1:
		status = providers.StatusSuccess
	case 0:
		status = providers.StatusFailed
	case 3:
		status = providers.StatusPending
	case 2:
		status = providers.StatusDuplicate
	default:
		status = providers.StatusPending
	}

	span.SetAttributes(
		attribute.String("transaction_id", apiResp.Data.TransactionID),
		attribute.String("status", string(status)),
		attribute.Int("orange_status_code", apiResp.Status),
	)

	// Determine RequestedAt and CompletedAt
	var requestedAt time.Time
	if apiResp.Data.RequestDate != nil {
		requestedAt = *apiResp.Data.RequestDate
	} else {
		requestedAt = time.Now()
	}

	// Map to standard response
	return &providers.TransactionResponse{
		Success:       status == providers.StatusSuccess || status == providers.StatusPending,
		TransactionID: apiResp.Data.TransactionID,
		Reference:     apiResp.Data.Reference,
		Status:        status,
		Amount:        req.Amount,
		Currency:      req.Currency,
		Beneficiary:   apiResp.Data.Beneficiary,
		RequestedAt:   requestedAt,
		CompletedAt:   apiResp.Data.ApproveDate,
		Message:       apiResp.Message,
		ProviderData: map[string]interface{}{
			"orange_status_code": apiResp.Status,
			"trans_type":         apiResp.Data.TransType,
			"timestamp":          apiResp.TimeStamp,
		},
	}, nil
}

// CheckTransactionStatus checks the status of a transaction via Orange statusCheck API
func (p *Provider) CheckTransactionStatus(ctx context.Context, transactionID, reference string) (*providers.TransactionStatusResponse, error) {
	ctx, span := tracer.Start(ctx, "orange.provider.check_transaction_status",
		trace.WithAttributes(
			attribute.String("transaction_id", transactionID),
			attribute.String("reference", reference),
		))
	defer span.End()

	p.log.Info("Checking transaction status via Orange API",
		"transaction_id", transactionID,
		"reference", reference)

	// Prepare request body (matching Orange Extensibility API: Transfer/statusCheck)
	body := map[string]interface{}{
		"transactionId": transactionID, // Optional if reference provided
		"reference":     reference,     // Client reference
	}

	// API response structure (from Orange documentation)
	var apiResp struct {
		Status  int    `json:"status"` // 1=success, 0=failed, 3=pending, 2=duplicate
		Message string `json:"message"`
		Data    struct {
			TransactionID string     `json:"transactionId"`
			Reference     string     `json:"reference"`
			Beneficiary   string     `json:"beneficiary"`
			Amount        float64    `json:"amount"`      // In cedis
			RequestDate   *time.Time `json:"requestDate"` // When initiated
			ApproveDate   *time.Time `json:"approveDate"` // When approved (null if pending)
		} `json:"data"`
	}

	// Make authenticated request (Orange Extensibility API: Transfer/statusCheck)
	if err := p.makeAuthenticatedRequest(ctx, "POST", "/Transfer/statusCheck", body, &apiResp); err != nil {
		span.RecordError(err)
		p.log.Error("Transaction status check failed",
			"error", err,
			"transaction_id", transactionID,
			"reference", reference)
		return nil, fmt.Errorf("status check failed: %w", err)
	}

	p.log.Info("Transaction status response received from Orange API",
		"status_code", apiResp.Status,
		"message", apiResp.Message,
		"transaction_id", apiResp.Data.TransactionID,
		"reference", apiResp.Data.Reference,
		"approve_date", apiResp.Data.ApproveDate)

	// Map Orange status code to standard status
	var status providers.TransactionStatus
	switch apiResp.Status {
	case 1:
		status = providers.StatusSuccess
	case 0:
		status = providers.StatusFailed
	case 3:
		status = providers.StatusPending
	case 2:
		status = providers.StatusDuplicate
	default:
		status = providers.StatusPending
	}

	span.SetAttributes(
		attribute.String("transaction_id", apiResp.Data.TransactionID),
		attribute.String("status", string(status)),
		attribute.Int("provider_status_code", apiResp.Status),
	)

	// Convert amount from cedis to pesewas
	amount := int64(apiResp.Data.Amount * 100)

	// Determine completed timestamp (ApproveDate if status is success)
	var completedAt *time.Time
	if status == providers.StatusSuccess && apiResp.Data.ApproveDate != nil {
		completedAt = apiResp.Data.ApproveDate
	}

	// Map to standard response
	return &providers.TransactionStatusResponse{
		TransactionID:      apiResp.Data.TransactionID,
		Reference:          apiResp.Data.Reference,
		Status:             status,
		ProviderStatusCode: fmt.Sprintf("%d", apiResp.Status), // Store raw Orange status code
		Amount:             float64(amount),
		Beneficiary:        apiResp.Data.Beneficiary,
		RequestedAt:        apiResp.Data.RequestDate,
		CompletedAt:        completedAt,
		Message:            apiResp.Message,
		ProviderData: map[string]interface{}{
			"orange_status_code": apiResp.Status,
			"request_date":       apiResp.Data.RequestDate,
			"approve_date":       apiResp.Data.ApproveDate,
		},
	}, nil
}

// normalizeProviderName normalizes provider names to Orange API format
// Orange API expects: MTN, TELECEL, AIRTELTIGO (uppercase, no suffixes)
func normalizeProviderName(provider string) string {
	// Convert to uppercase for comparison
	upperProvider := strings.ToUpper(provider)

	switch {
	case strings.Contains(upperProvider, "MTN"):
		return "MTN"
	case strings.Contains(upperProvider, "TELECEL") || strings.Contains(upperProvider, "VODAFONE"):
		return "TELECEL"
	case strings.Contains(upperProvider, "AIRTEL") || strings.Contains(upperProvider, "TIGO"):
		return "AIRTELTIGO"
	default:
		// If no match found, try to extract first part before underscore/space and uppercase it
		if len(provider) > 0 {
			for i, char := range provider {
				if char == '_' || char == ' ' {
					return strings.ToUpper(provider[:i])
				}
			}
		}
		// Last resort: just uppercase the whole string
		return strings.ToUpper(provider)
	}
}
