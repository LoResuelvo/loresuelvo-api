package workorder

import "context"

type ReviewReportRepository interface {
	FindByWorkOrderID(context.Context, int) (*ReviewReport, error)
	Save(context.Context, *ReviewReport) error
}
type ReviewReportActorFinder interface {
	FindByAuthID(context.Context, string) (int, string, error)
}
type ReviewReportOrderFinder interface {
	FindByID(context.Context, int) (*WorkOrder, error)
}
