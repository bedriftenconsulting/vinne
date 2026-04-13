package services

import (
	"context"

	"github.com/randco/randco-microservices/services/service-agent-management/internal/models"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/repositories"
)

type posService struct {
	repos *repositories.Repositories
}

func NewPOSDeviceService(repos *repositories.Repositories) POSDeviceService {
	service := &posService{repos: repos}
	return service
}

func (s *posService) ListPOSDevices(ctx context.Context, filter *repositories.POSDeviceFilters, page int32, pageSize int32) ([]models.POSDevice, int, error) {
	devices, err := s.repos.POSDevice.List(ctx, *filter)
	if err != nil {
		return nil, 0, err
	}
	return devices, len(devices), nil
}
