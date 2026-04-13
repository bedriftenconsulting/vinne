package repositories

import "database/sql"

type Repositories struct {
	Player     PlayerRepository
	PlayerAuth PlayerAuthRepository
	Session    SessionRepository
	Device     DeviceRepository
	OTP        OTPRepository
}

func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{
		Player:     NewPlayerRepository(db),
		PlayerAuth: NewPlayerAuthRepository(db),
		Session:    NewSessionRepository(db),
		Device:     NewDeviceRepository(db),
		OTP:        NewOTPRepository(db),
	}
}
