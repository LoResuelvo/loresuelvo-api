package consumer

import "math"

// GeoPoint is a WGS84 latitude/longitude pair in decimal degrees.
type GeoPoint struct {
	Latitude  float64
	Longitude float64
}

func NewGeoPoint(latitude, longitude float64) (GeoPoint, error) {
	point := GeoPoint{Latitude: latitude, Longitude: longitude}
	if err := point.Validate(); err != nil {
		return GeoPoint{}, err
	}

	return point, nil
}

func (point GeoPoint) Validate() error {
	if math.IsNaN(point.Latitude) || math.IsInf(point.Latitude, 0) || point.Latitude < -90 || point.Latitude > 90 {
		return ErrLatitudeInvalid
	}
	if math.IsNaN(point.Longitude) || math.IsInf(point.Longitude, 0) || point.Longitude < -180 || point.Longitude > 180 {
		return ErrLongitudeInvalid
	}

	return nil
}
