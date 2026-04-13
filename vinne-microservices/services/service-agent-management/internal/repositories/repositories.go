package repositories

import (
	"github.com/jmoiron/sqlx"
)

// Repositories aggregates all repository interfaces
type Repositories struct {
	// Agent repositories
	Agent       AgentRepository
	AgentStatus AgentStatusRepository

	// Retailer repositories
	Retailer             RetailerRepository
	RetailerRelationship RetailerRelationshipRepository
	RetailerStatus       RetailerStatusRepository

	// POS Device repositories
	POSDevice            POSDeviceRepository
	POSDeviceAssignment  POSDeviceAssignmentRepository
	POSDeviceStatus      POSDeviceStatusRepository
	POSDeviceMaintenance POSDeviceMaintenanceRepository

	// Relationship repositories
	AgentRetailer AgentRetailerRepository

	// KYC repositories
	AgentKYC    AgentKYCRepository
	RetailerKYC RetailerKYCRepository

	// Performance repository
	Performance PerformanceRepository
}

// NewRepositories creates all repository implementations
func NewRepositories(db *sqlx.DB) *Repositories {
	return &Repositories{
		// Agent repositories
		Agent:       NewAgentRepository(db),
		AgentStatus: NewAgentStatusRepository(db),

		// Retailer repositories
		Retailer:             NewRetailerRepository(db),
		RetailerRelationship: NewRetailerRelationshipRepository(db),
		RetailerStatus:       NewRetailerStatusRepository(db),

		// POS Device repositories
		POSDevice:            NewPOSDeviceRepository(db),
		POSDeviceAssignment:  NewPOSDeviceAssignmentRepository(db),
		POSDeviceStatus:      NewPOSDeviceStatusRepository(db),
		POSDeviceMaintenance: NewPOSDeviceMaintenanceRepository(db),

		// Relationship repositories
		AgentRetailer: NewAgentRetailerRepository(db),

		// KYC repositories
		AgentKYC:    NewAgentKYCRepository(db),
		RetailerKYC: NewRetailerKYCRepository(db),

		// Performance repository
		Performance: NewPerformanceRepository(db),
	}
}
