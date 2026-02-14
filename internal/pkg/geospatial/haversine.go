package geospatial

import "math"

const earthRadiusKm = 6371.0

// Haversine calculates the great-circle distance in meters between two points.
func Haversine(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKm * c * 1000 // meters
}

// BoundingBox returns a bounding box around a point with the given radius in meters.
func BoundingBox(lat, lon, radiusMeters float64) (minLat, minLon, maxLat, maxLon float64) {
	latDelta := radiusMeters / 111320.0
	lonDelta := radiusMeters / (111320.0 * math.Cos(toRad(lat)))

	return lat - latDelta, lon - lonDelta, lat + latDelta, lon + lonDelta
}

func toRad(deg float64) float64 {
	return deg * math.Pi / 180
}
