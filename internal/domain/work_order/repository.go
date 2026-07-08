package workorder

import (
	"context"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order/read_model"
)

type Reader interface {
	FindByUserID(ctx context.Context, userID int, viewerRole string) ([]readmodel.WorkOrderSummary, error)
}

type UserRepository interface {
	FindByAuthID(auth0ID string) (user.User, error)
}

type FileURLResolver interface {
	ResolvePublicURLs(ctx context.Context, fileIDs []string) (map[string]string, error)
}
