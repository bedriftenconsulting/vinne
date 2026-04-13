package clients

import (
	"context"
	"fmt"

	paymentpb "github.com/randco/randco-microservices/proto/payment/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PaymentClient struct {
	conn   *grpc.ClientConn
	client paymentpb.PaymentServiceClient
}

func NewPaymentClient(address string) (*PaymentClient, error) {
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to payment service: %w", err)
	}

	return &PaymentClient{
		conn:   conn,
		client: paymentpb.NewPaymentServiceClient(conn),
	}, nil
}

func (c *PaymentClient) InitiateDeposit(ctx context.Context, req *paymentpb.InitiateDepositRequest) (*paymentpb.InitiateDepositResponse, error) {
	resp, err := c.client.InitiateDeposit(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate deposit: %w", err)
	}
	return resp, nil
}

func (c *PaymentClient) GetDepositStatus(ctx context.Context, req *paymentpb.GetDepositStatusRequest) (*paymentpb.GetDepositStatusResponse, error) {
	resp, err := c.client.GetDepositStatus(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get deposit status: %w", err)
	}
	return resp, nil
}

func (c *PaymentClient) InitiateWithdrawal(ctx context.Context, req *paymentpb.InitiateWithdrawalRequest) (*paymentpb.InitiateWithdrawalResponse, error) {
	resp, err := c.client.InitiateWithdrawal(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate withdrawal: %w", err)
	}
	return resp, nil
}

func (c *PaymentClient) GetWithdrawalStatus(ctx context.Context, req *paymentpb.GetWithdrawalStatusRequest) (*paymentpb.GetWithdrawalStatusResponse, error) {
	resp, err := c.client.GetWithdrawalStatus(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get withdrawal status: %w", err)
	}
	return resp, nil
}

func (c *PaymentClient) InitiateBankTransfer(ctx context.Context, req *paymentpb.InitiateBankTransferRequest) (*paymentpb.InitiateBankTransferResponse, error) {
	resp, err := c.client.InitiateBankTransfer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate bank transfer: %w", err)
	}
	return resp, nil
}

func (c *PaymentClient) GetBankTransferStatus(ctx context.Context, req *paymentpb.GetBankTransferStatusRequest) (*paymentpb.GetBankTransferStatusResponse, error) {
	resp, err := c.client.GetBankTransferStatus(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get bank transfer status: %w", err)
	}
	return resp, nil
}

func (c *PaymentClient) VerifyWallet(ctx context.Context, req *paymentpb.VerifyWalletRequest) (*paymentpb.VerifyWalletResponse, error) {
	resp, err := c.client.VerifyWallet(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to verify wallet: %w", err)
	}
	return resp, nil
}

func (c *PaymentClient) VerifyBankAccount(ctx context.Context, req *paymentpb.VerifyBankAccountRequest) (*paymentpb.VerifyBankAccountResponse, error) {
	resp, err := c.client.VerifyBankAccount(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to verify bank account: %w", err)
	}
	return resp, nil
}

func (c *PaymentClient) VerifyIdentity(ctx context.Context, req *paymentpb.VerifyIdentityRequest) (*paymentpb.VerifyIdentityResponse, error) {
	resp, err := c.client.VerifyIdentity(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to verify identity: %w", err)
	}
	return resp, nil
}

func (c *PaymentClient) GetTransaction(ctx context.Context, req *paymentpb.GetTransactionRequest) (*paymentpb.GetTransactionResponse, error) {
	resp, err := c.client.GetTransaction(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}
	return resp, nil
}

func (c *PaymentClient) ListTransactions(ctx context.Context, req *paymentpb.ListTransactionsRequest) (*paymentpb.ListTransactionsResponse, error) {
	resp, err := c.client.ListTransactions(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}
	return resp, nil
}

func (c *PaymentClient) CancelTransaction(ctx context.Context, req *paymentpb.CancelTransactionRequest) (*paymentpb.CancelTransactionResponse, error) {
	resp, err := c.client.CancelTransaction(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel transaction: %w", err)
	}
	return resp, nil
}

func (c *PaymentClient) ListProviders(ctx context.Context, req *paymentpb.ListProvidersRequest) (*paymentpb.ListProvidersResponse, error) {
	resp, err := c.client.ListProviders(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list providers: %w", err)
	}
	return resp, nil
}

func (c *PaymentClient) GetProviderHealth(ctx context.Context, req *paymentpb.GetProviderHealthRequest) (*paymentpb.GetProviderHealthResponse, error) {
	resp, err := c.client.GetProviderHealth(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider health: %w", err)
	}
	return resp, nil
}

func (c *PaymentClient) ProcessWebhook(ctx context.Context, req *paymentpb.ProcessWebhookRequest) (*paymentpb.ProcessWebhookResponse, error) {
	resp, err := c.client.ProcessWebhook(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to process webhook: %w", err)
	}
	return resp, nil
}

func (c *PaymentClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
