package operation

import (
	"errors"
	"fmt"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
)

var ErrInvalidChatQuery = errors.New("invalid conversation message query")

const MaxChatPageSize = 100

// ChatQuery binds a continuation position to its persisted conversation.
// HTTP cursor integrity and operation identity are validated by the adapter.
type ChatQuery struct {
	Limit          int
	ConversationID int
	After          *conversation.MessagePosition
}

func (query ChatQuery) Validate() error {
	if query.Limit < 0 || query.Limit > MaxChatPageSize {
		return fmt.Errorf("%w: limit must be between 1 and 100", ErrInvalidChatQuery)
	}
	if query.After == nil {
		if query.ConversationID != 0 {
			return fmt.Errorf("%w: conversation requires a position", ErrInvalidChatQuery)
		}
		return nil
	}
	if query.ConversationID <= 0 || query.After.ID <= 0 || query.After.CreatedOn.IsZero() {
		return fmt.Errorf("%w: invalid continuation position", ErrInvalidChatQuery)
	}
	return nil
}

func (query ChatQuery) EffectiveLimit() int {
	if query.Limit == 0 {
		return DefaultChatPageSize
	}
	return query.Limit
}
