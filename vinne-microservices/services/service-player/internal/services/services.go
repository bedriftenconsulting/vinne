package services

import (
	"github.com/randco/service-player/internal/clients"
	"github.com/randco/service-player/internal/config"
	"github.com/randco/service-player/internal/repositories"
)

type Services struct {
	PlayerAuth   AuthService
	Registration RegistrationService
	Profile      ProfileService
	Session      SessionService
	Admin        AdminService
	OTP          OTPService
}

func NewServices(repo *repositories.Repositories, cfg *config.SecurityConfig, notificationClient *clients.NotificationClient, walletClient *clients.WalletClient) *Services {
	otpService := NewOTPService(repo.OTP, notificationClient, repo.Player)

	return &Services{
		PlayerAuth:   NewAuthService(repo.Player, repo.PlayerAuth, repo.Session, cfg),
		Registration: NewRegistrationService(repo.Player, repo.PlayerAuth, otpService, walletClient),
		Profile:      NewProfileService(repo.Player, repo.PlayerAuth),
		Session:      NewSessionService(repo.Session, repo.Device),
		Admin:        NewAdminService(repo.Player),
		OTP:          otpService,
	}
}
