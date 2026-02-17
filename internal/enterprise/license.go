package enterprise

import (
	"errors"
	"slices"
	"time"
)

//nolint:gochecknoglobals // sentinel error
var ErrLicenseExpired = errors.New("enterprise: license expired")

//nolint:gochecknoglobals // sentinel error
var ErrLicenseInvalid = errors.New("enterprise: license invalid")

//nolint:gochecknoglobals // sentinel error
var ErrNoLicense = errors.New("enterprise: no license configured")

// License represents an enterprise license.
type License struct {
	ID        string
	Org       string
	MaxUsers  int
	MaxAgents int
	Features  []string // enabled feature flags
	ExpiresAt time.Time
	IssuedAt  time.Time
}

// Validator checks enterprise licenses.
type Validator struct {
	license *License
}

// NewValidator creates a Validator. If license is nil, all enterprise checks fail with ErrNoLicense.
func NewValidator(license *License) *Validator {
	return &Validator{license: license}
}

// Validate checks if the license is valid and not expired.
func (v *Validator) Validate() error {
	if v.license == nil {
		return ErrNoLicense
	}

	if time.Now().After(v.license.ExpiresAt) {
		return ErrLicenseExpired
	}

	return nil
}

// HasFeature checks if a specific feature is enabled.
func (v *Validator) HasFeature(feature string) bool {
	if v.license == nil {
		return false
	}

	return slices.Contains(v.license.Features, feature)
}

// MaxUsers returns the maximum allowed users (0 = unlimited when no license).
func (v *Validator) MaxUsers() int {
	if v.license == nil {
		return 0
	}

	return v.license.MaxUsers
}

// MaxAgents returns the maximum allowed concurrent agents.
func (v *Validator) MaxAgents() int {
	if v.license == nil {
		return 0
	}

	return v.license.MaxAgents
}
