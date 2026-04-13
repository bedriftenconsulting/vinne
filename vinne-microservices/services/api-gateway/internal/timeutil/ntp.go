package timeutil

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/beevik/ntp"
)

// NTPTimeService provides reliable time from NTP servers
type NTPTimeService struct {
	servers      []string
	mu           sync.RWMutex
	offset       time.Duration
	lastSync     time.Time
	ntpTime      time.Time // Absolute time from NTP server
	syncInterval time.Duration
}

// NewNTPTimeService creates a new NTP time service
func NewNTPTimeService(servers ...string) *NTPTimeService {
	if len(servers) == 0 {
		// Default to reliable NTP servers
		servers = []string{
			"time.cloudflare.com", // Cloudflare's NTP (fast and reliable)
			"time.google.com",     // Google's NTP
			"pool.ntp.org",        // NTP Pool Project
		}
	}

	service := &NTPTimeService{
		servers:      servers,
		syncInterval: 5 * time.Minute, // Sync every 5 minutes
	}

	// Initial sync
	_ = service.Sync()

	// Start background sync
	go service.backgroundSync()

	return service
}

// Now returns the current time from NTP with fallback to system time
func (n *NTPTimeService) Now() time.Time {
	n.mu.RLock()
	ntpTime := n.ntpTime
	lastSync := n.lastSync
	n.mu.RUnlock()

	// If we haven't synced successfully or sync is too old (>1 hour), use system time
	if lastSync.IsZero() || time.Since(lastSync) > time.Hour {
		return time.Now()
	}

	// Calculate how much time has passed since the last NTP sync
	// and add that to the NTP time we received
	timeSinceSync := time.Since(lastSync)
	return ntpTime.Add(timeSinceSync)
}

// Sync synchronizes with NTP servers
func (n *NTPTimeService) Sync() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var lastErr error
	for _, server := range n.servers {
		select {
		case <-ctx.Done():
			return fmt.Errorf("sync timeout: %w", lastErr)
		default:
		}

		response, err := ntp.QueryWithOptions(server, ntp.QueryOptions{
			Timeout: 2 * time.Second,
		})
		if err != nil {
			lastErr = err
			continue
		}

		// Store the absolute NTP time and offset
		n.mu.Lock()
		n.offset = response.ClockOffset
		n.ntpTime = response.Time // Absolute time from NTP server
		n.lastSync = time.Now()
		n.mu.Unlock()

		return nil
	}

	return fmt.Errorf("failed to sync with all NTP servers: %w", lastErr)
}

// backgroundSync periodically syncs with NTP servers
func (n *NTPTimeService) backgroundSync() {
	ticker := time.NewTicker(n.syncInterval)
	defer ticker.Stop()

	for range ticker.C {
		_ = n.Sync()
	}
}

// GetOffset returns the current clock offset
func (n *NTPTimeService) GetOffset() time.Duration {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.offset
}

// LastSyncTime returns the last successful sync time
func (n *NTPTimeService) LastSyncTime() time.Time {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.lastSync
}
