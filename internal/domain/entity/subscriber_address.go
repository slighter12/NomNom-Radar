package entity

// SubscriberAddress represents a user address bundled with the subscription's
// notification radius. This is used for geospatial queries that need both the
// address location and the user's chosen radius in one result to avoid N+1
// lookups.
type SubscriberAddress struct {
	Address
	NotificationRadius float64 `json:"notification_radius"`
}
