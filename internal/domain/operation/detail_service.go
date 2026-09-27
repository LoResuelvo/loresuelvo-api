package operation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/google/uuid"
)

var ErrInvalidOperationID = errors.New("invalid operation ID")
var ErrOperationNotFound = errors.New("operation not found")

type DetailReader interface {
	FindByID(ctx context.Context, id readmodel.ID) (*readmodel.OperationDetail, error)
}

type DetailService struct {
	reader      DetailReader
	operatorIDs audit.OperatorIDFinder
	auditWriter audit.Writer
	clock       clock.Clock
}

func NewDetailService(reader DetailReader, operatorIDs audit.OperatorIDFinder, auditWriter audit.Writer, clock clock.Clock) *DetailService {
	return &DetailService{reader: reader, operatorIDs: operatorIDs, auditWriter: auditWriter, clock: clock}
}

func ParseOperationID(raw string) (readmodel.ID, error) {
	kind, number, found := strings.Cut(raw, "-")
	if !found || (kind != string(readmodel.KindJobRequest) && kind != string(readmodel.KindServiceProposal)) || number == "" || number[0] < '1' || number[0] > '9' {
		return readmodel.ID{}, ErrInvalidOperationID
	}
	for _, digit := range number {
		if digit < '0' || digit > '9' {
			return readmodel.ID{}, ErrInvalidOperationID
		}
	}
	parsed, err := strconv.ParseInt(number, 10, 32)
	if err != nil || parsed <= 0 || parsed > math.MaxInt32 {
		return readmodel.ID{}, ErrInvalidOperationID
	}
	return readmodel.ID{Kind: readmodel.Kind(kind), ResourceID: int(parsed)}, nil
}

func (service *DetailService) Query(ctx context.Context, rawID, authSubject, correlationID string) (*readmodel.OperationDetail, error) {
	id, err := ParseOperationID(rawID)
	if err != nil {
		return nil, err
	}
	detail, err := service.reader.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("reading operation detail: %w", err)
	}
	if detail == nil {
		return nil, ErrOperationNotFound
	}
	resourceType := "job_request"
	if id.Kind == readmodel.KindServiceProposal {
		resourceType = "service_proposal"
	}
	actorID, err := service.operatorIDs.FindOperatorIDByAuthID(ctx, authSubject)
	if err != nil {
		return nil, fmt.Errorf("finding operation detail reader: %w", err)
	}
	event, err := audit.NewEvent(audit.EventParams{
		ID: uuid.New(), OperatorID: actorID, Action: audit.ActionAccess,
		ResourceType: resourceType, ResourceID: strconv.Itoa(id.ResourceID), OccurredOn: service.clock.Now(),
		Result: audit.ResultPrepared, CorrelationID: correlationID,
	})
	if err != nil {
		return nil, fmt.Errorf("creating operation detail access event: %w", err)
	}
	if err := service.auditWriter.Save(ctx, event); err != nil {
		return nil, fmt.Errorf("saving operation detail access event: %w", err)
	}
	return detail, nil
}
