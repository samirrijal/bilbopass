package domain

import (
	"time"
)

// Agency represents a transit agency (e.g. Bilbobus, EuskoTren).
type Agency struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	URL       string    `json:"url,omitempty"`
	Timezone  string    `json:"timezone"`
	CreatedAt time.Time `json:"created_at"`
}

// Stop represents a transit stop or station.
type Stop struct {
	ID                   string         `json:"id"`
	StopID               string         `json:"stop_id"`
	AgencyID             string         `json:"agency_id"`
	Name                 string         `json:"name"`
	Location             GeoPoint       `json:"location"`
	PlatformCode         string         `json:"platform_code,omitempty"`
	WheelchairAccessible bool           `json:"wheelchair_accessible"`
	Metadata             map[string]any `json:"metadata,omitempty"`
	Distance             *float64       `json:"distance,omitempty"` // computed field
	CreatedAt            time.Time      `json:"created_at"`
}

// Route represents a transit route.
type Route struct {
	ID        string         `json:"id"`
	RouteID   string         `json:"route_id"`
	AgencyID  string         `json:"agency_id"`
	ShortName string         `json:"short_name,omitempty"`
	LongName  string         `json:"long_name"`
	RouteType int            `json:"route_type"`
	Color     string         `json:"color"`
	TextColor string         `json:"text_color"`
	Shape     *GeoLineString `json:"shape,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// Trip represents a single trip on a route.
type Trip struct {
	ID                   string    `json:"id"`
	TripID               string    `json:"trip_id"`
	RouteID              string    `json:"route_id"`
	ServiceID            string    `json:"service_id"`
	Headsign             string    `json:"headsign,omitempty"`
	DirectionID          int       `json:"direction_id"`
	ShapeID              string    `json:"shape_id,omitempty"`
	WheelchairAccessible bool      `json:"wheelchair_accessible"`
	BikesAllowed         bool      `json:"bikes_allowed"`
	CreatedAt            time.Time `json:"created_at"`
}

// StopTime represents a scheduled stop on a trip.
type StopTime struct {
	ID            string        `json:"id"`
	TripID        string        `json:"trip_id"`
	StopID        string        `json:"stop_id"`
	ArrivalTime   time.Duration `json:"arrival_time"`
	DepartureTime time.Duration `json:"departure_time"`
	StopSequence  int           `json:"stop_sequence"`
	PickupType    int           `json:"pickup_type"`
	DropOffType   int           `json:"drop_off_type"`
	CreatedAt     time.Time     `json:"created_at"`
}

// VehiclePosition is a real-time vehicle location reading.
type VehiclePosition struct {
	Time            time.Time      `json:"time"`
	VehicleID       string         `json:"vehicle_id"`
	TripID          string         `json:"trip_id,omitempty"`
	RouteID         string         `json:"route_id,omitempty"`
	Location        GeoPoint       `json:"location"`
	Bearing         float64        `json:"bearing"`
	Speed           float64        `json:"speed"` // m/s
	CongestionLevel int            `json:"congestion_level"`
	OccupancyStatus int            `json:"occupancy_status"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// DelayEvent records a detected delay at a stop.
type DelayEvent struct {
	ID                 string         `json:"id"`
	Time               time.Time      `json:"time"`
	TripID             string         `json:"trip_id"`
	StopID             string         `json:"stop_id"`
	ScheduledArrival   time.Time      `json:"scheduled_arrival"`
	ActualArrival      time.Time      `json:"actual_arrival"`
	DelaySeconds       int            `json:"delay_seconds"`
	IsCompensated      bool           `json:"is_compensated"`
	CompensationSentAt *time.Time     `json:"compensation_sent_at,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

// Affiliate is a partner shop that offers compensations.
type Affiliate struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Category   string    `json:"category"`
	Location   GeoPoint  `json:"location"`
	Address    string    `json:"address,omitempty"`
	OfferText  string    `json:"offer_text"`
	OfferValue float64   `json:"offer_value"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
}

// Compensation is a coupon issued to a user after a delay.
type Compensation struct {
	ID           string         `json:"id"`
	UserID       string         `json:"user_id"`
	DelayEventID string         `json:"delay_event_id"`
	AffiliateID  string         `json:"affiliate_id"`
	Code         string         `json:"code"`
	IssuedAt     time.Time      `json:"issued_at"`
	ExpiresAt    time.Time      `json:"expires_at"`
	RedeemedAt   *time.Time     `json:"redeemed_at,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// Departure is a computed next-departure at a stop.
type Departure struct {
	Trip          *Trip      `json:"trip"`
	ScheduledTime time.Time  `json:"scheduled_time"`
	EstimatedTime *time.Time `json:"estimated_time,omitempty"`
	Delay         *int       `json:"delay,omitempty"` // seconds
	Platform      string     `json:"platform,omitempty"`
}

// Journey represents a possible route between two stops.
type Journey struct {
	Legs          []JourneyLeg  `json:"legs"`
	Duration      time.Duration `json:"duration"`
	DepartureTime time.Time     `json:"departure_time"`
	ArrivalTime   time.Time     `json:"arrival_time"`
	Transfers     int           `json:"transfers"`
}

// JourneyLeg is a single segment inside a journey.
type JourneyLeg struct {
	Route       *Route    `json:"route"`
	FromStop    *Stop     `json:"from_stop"`
	ToStop      *Stop     `json:"to_stop"`
	Departure   Departure `json:"departure"`
	ArrivalTime time.Time `json:"arrival_time"`
}
