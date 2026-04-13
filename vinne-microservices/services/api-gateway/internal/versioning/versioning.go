package versioning

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Version represents an API version
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Deprecated bool
	SunsetDate *time.Time
}

// VersionManager handles API versioning
type VersionManager struct {
	versions       map[string]*Version
	routes         map[string]map[string]string // version -> route -> handler
	defaultVersion string
	latestVersion  string
}

// NewVersionManager creates a new version manager
func NewVersionManager() *VersionManager {
	return &VersionManager{
		versions:       make(map[string]*Version),
		routes:         make(map[string]map[string]string),
		defaultVersion: "v1",
		latestVersion:  "v1",
	}
}

// RegisterVersion registers a new API version
func (vm *VersionManager) RegisterVersion(versionStr string, deprecated bool, sunsetDate *time.Time) error {
	version, err := ParseVersion(versionStr)
	if err != nil {
		return err
	}

	vm.versions[versionStr] = &Version{
		Major:      version.Major,
		Minor:      version.Minor,
		Patch:      version.Patch,
		Deprecated: deprecated,
		SunsetDate: sunsetDate,
	}

	if vm.routes[versionStr] == nil {
		vm.routes[versionStr] = make(map[string]string)
	}

	// Update latest version
	if !deprecated {
		if vm.isNewerVersion(versionStr, vm.latestVersion) {
			vm.latestVersion = versionStr
		}
	}

	return nil
}

// ParseVersion parses a version string
func ParseVersion(versionStr string) (*Version, error) {
	// Remove 'v' prefix if present
	versionStr = strings.TrimPrefix(versionStr, "v")

	parts := strings.Split(versionStr, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return nil, fmt.Errorf("invalid version format: %s", versionStr)
	}

	version := &Version{}

	// Parse major version
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", parts[0])
	}
	version.Major = major

	// Parse minor version if present
	if len(parts) > 1 {
		minor, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid minor version: %s", parts[1])
		}
		version.Minor = minor
	}

	// Parse patch version if present
	if len(parts) > 2 {
		patch, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid patch version: %s", parts[2])
		}
		version.Patch = patch
	}

	return version, nil
}

// ExtractVersion extracts version from request
func (vm *VersionManager) ExtractVersion(r *http.Request) string {
	// Check URL path for version
	pathVersion := vm.extractVersionFromPath(r.URL.Path)
	if pathVersion != "" {
		return pathVersion
	}

	// Check Accept header for version
	acceptVersion := vm.extractVersionFromHeader(r.Header.Get("Accept"))
	if acceptVersion != "" {
		return acceptVersion
	}

	// Check custom version header
	if version := r.Header.Get("X-API-Version"); version != "" {
		return version
	}

	// Return default version
	return vm.defaultVersion
}

// extractVersionFromPath extracts version from URL path
func (vm *VersionManager) extractVersionFromPath(path string) string {
	// Pattern: /api/v1/... or /api/v2.1/...
	re := regexp.MustCompile(`/api/(v\d+(?:\.\d+)?(?:\.\d+)?)/`)
	matches := re.FindStringSubmatch(path)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractVersionFromHeader extracts version from Accept header
func (vm *VersionManager) extractVersionFromHeader(accept string) string {
	// Pattern: application/vnd.api+json;version=1.0
	re := regexp.MustCompile(`version=(\d+(?:\.\d+)?(?:\.\d+)?)`)
	matches := re.FindStringSubmatch(accept)
	if len(matches) > 1 {
		return "v" + matches[1]
	}
	return ""
}

// GetVersionInfo returns version information
func (vm *VersionManager) GetVersionInfo(versionStr string) *Version {
	return vm.versions[versionStr]
}

// IsDeprecated checks if a version is deprecated
func (vm *VersionManager) IsDeprecated(versionStr string) bool {
	if version, ok := vm.versions[versionStr]; ok {
		return version.Deprecated
	}
	return false
}

// GetSunsetDate returns the sunset date for a version
func (vm *VersionManager) GetSunsetDate(versionStr string) *time.Time {
	if version, ok := vm.versions[versionStr]; ok {
		return version.SunsetDate
	}
	return nil
}

// MapRoute maps a route to a specific version
func (vm *VersionManager) MapRoute(version, route, handler string) {
	if vm.routes[version] == nil {
		vm.routes[version] = make(map[string]string)
	}
	vm.routes[version][route] = handler
}

// GetHandler returns the handler for a specific version and route
func (vm *VersionManager) GetHandler(version, route string) (string, bool) {
	if routes, ok := vm.routes[version]; ok {
		if handler, ok := routes[route]; ok {
			return handler, true
		}
	}
	// Fallback to previous version if route not found
	return vm.getFallbackHandler(version, route)
}

// getFallbackHandler tries to find handler in previous versions
func (vm *VersionManager) getFallbackHandler(version, route string) (string, bool) {
	v, err := ParseVersion(version)
	if err != nil {
		return "", false
	}

	// Try previous minor versions
	for minor := v.Minor - 1; minor >= 0; minor-- {
		prevVersion := fmt.Sprintf("v%d.%d", v.Major, minor)
		if routes, ok := vm.routes[prevVersion]; ok {
			if handler, ok := routes[route]; ok {
				return handler, true
			}
		}
	}

	// Try previous major versions
	for major := v.Major - 1; major >= 1; major-- {
		prevVersion := fmt.Sprintf("v%d", major)
		if routes, ok := vm.routes[prevVersion]; ok {
			if handler, ok := routes[route]; ok {
				return handler, true
			}
		}
	}

	return "", false
}

// isNewerVersion checks if version1 is newer than version2
func (vm *VersionManager) isNewerVersion(version1, version2 string) bool {
	v1, err1 := ParseVersion(version1)
	v2, err2 := ParseVersion(version2)

	if err1 != nil || err2 != nil {
		return false
	}

	if v1.Major > v2.Major {
		return true
	}
	if v1.Major == v2.Major && v1.Minor > v2.Minor {
		return true
	}
	if v1.Major == v2.Major && v1.Minor == v2.Minor && v1.Patch > v2.Patch {
		return true
	}

	return false
}

// GetAvailableVersions returns all available versions
func (vm *VersionManager) GetAvailableVersions() []string {
	var versions []string
	for version := range vm.versions {
		versions = append(versions, version)
	}
	return versions
}

// SetVersionHeaders sets version-related headers
func (vm *VersionManager) SetVersionHeaders(w http.ResponseWriter, version string) {
	w.Header().Set("X-API-Version", version)

	if vm.IsDeprecated(version) {
		w.Header().Set("X-API-Deprecated", "true")

		if sunsetDate := vm.GetSunsetDate(version); sunsetDate != nil {
			w.Header().Set("X-API-Sunset", sunsetDate.Format(time.RFC3339))
			w.Header().Set("Sunset", sunsetDate.Format(time.RFC1123))
		}

		// Add warning header
		warning := fmt.Sprintf("299 - \"This API version is deprecated and will be sunset on %s\"",
			vm.GetSunsetDate(version).Format("2006-01-02"))
		w.Header().Set("Warning", warning)
	}

	// Add link to latest version
	if version != vm.latestVersion {
		w.Header().Set("X-API-Latest-Version", vm.latestVersion)
	}
}

// VersionMiddleware creates a middleware for handling API versioning
func (vm *VersionManager) VersionMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		version := vm.ExtractVersion(r)

		// Check if version exists
		if _, ok := vm.versions[version]; !ok {
			http.Error(w, fmt.Sprintf("API version %s not found", version), http.StatusNotFound)
			return
		}

		// Set version headers
		vm.SetVersionHeaders(w, version)

		// Continue with request
		next(w, r)
	}
}

// VersionInfo returns information about all versions
func (vm *VersionManager) VersionInfo() map[string]interface{} {
	info := make(map[string]interface{})

	versions := make([]map[string]interface{}, 0)
	for versionStr, version := range vm.versions {
		vInfo := map[string]interface{}{
			"version":    versionStr,
			"deprecated": version.Deprecated,
			"major":      version.Major,
			"minor":      version.Minor,
			"patch":      version.Patch,
		}

		if version.SunsetDate != nil {
			vInfo["sunset_date"] = version.SunsetDate.Format(time.RFC3339)
		}

		versions = append(versions, vInfo)
	}

	info["versions"] = versions
	info["latest"] = vm.latestVersion
	info["default"] = vm.defaultVersion

	return info
}
