package policy

// DevicePolicy defines domain rules for device token health and stale cleanup.
type DevicePolicy struct {
	HealthyWindowDays int
	StaleCleanupDays  int
}

// LoginThrottlePolicy defines domain rules for progressive lockout duration.
type LoginThrottlePolicy struct {
	InitialLockoutMinutes int
	BackoffMultiplier     int
	MaxLockoutMinutes     int
}

// RefreshTokenPolicy defines domain rules for refresh token cleanup retention.
type RefreshTokenPolicy struct {
	RevokedRetentionDays int
}

func DefaultDevicePolicy() DevicePolicy {
	return DevicePolicy{
		HealthyWindowDays: 30,
		StaleCleanupDays:  270,
	}
}

func DefaultLoginThrottlePolicy() LoginThrottlePolicy {
	return LoginThrottlePolicy{
		InitialLockoutMinutes: 15,
		BackoffMultiplier:     4,
		MaxLockoutMinutes:     1440,
	}
}

func DefaultRefreshTokenPolicy() RefreshTokenPolicy {
	return RefreshTokenPolicy{
		RevokedRetentionDays: 7,
	}
}

// LockoutMinutes computes lockout duration for the next lockout event.
// lockoutCount is the historical lockout count before increment.
func (p LoginThrottlePolicy) LockoutMinutes(lockoutCount int) int {
	if lockoutCount < 0 {
		lockoutCount = 0
	}

	minutes := p.InitialLockoutMinutes
	for i := 0; i < lockoutCount; i++ {
		minutes *= p.BackoffMultiplier
		if minutes >= p.MaxLockoutMinutes {
			return p.MaxLockoutMinutes
		}
	}
	if minutes > p.MaxLockoutMinutes {
		return p.MaxLockoutMinutes
	}

	return minutes
}
