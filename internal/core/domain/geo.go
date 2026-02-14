package domain

// GeoPoint represents a geographic coordinate (WGS 84).
type GeoPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// GeoLineString represents an ordered sequence of geographic coordinates.
type GeoLineString struct {
	Coordinates []GeoPoint `json:"coordinates"`
}

// Bounds represents a geographic bounding box.
type Bounds struct {
	MinLat float64 `json:"min_lat"`
	MinLon float64 `json:"min_lon"`
	MaxLat float64 `json:"max_lat"`
	MaxLon float64 `json:"max_lon"`
}
