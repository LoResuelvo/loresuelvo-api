package readmodel

import filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"

type Profile struct {
	ID            int
	Name          string
	Surname       string
	ProfilePhoto  *filedomain.Image
	CategoryID    int
	CategoryName  string
	RatingAverage float64
	RatingCount   int
	WorkOrders    []WorkOrder
}
