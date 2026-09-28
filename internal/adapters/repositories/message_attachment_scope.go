package repositories

import (
	"fmt"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
)

// Every attachment query joins its owning message and its work participants.
// File uploader must match the persisted sender; file availability remains file policy.
const messageAttachmentOwnershipSQL = `
 JOIN messages m ON m.id = attachment.message_id
 JOIN conversations c ON c.id = m.conversation_id AND c.type = 'work'
 JOIN work_conversations wc ON wc.conversation_id = c.id
 JOIN users sender ON sender.id = CASE m.sender_role WHEN 'consumer' THEN wc.consumer_id WHEN 'provider' THEN wc.provider_id END
 JOIN files f ON f.id = attachment.file_id AND f.uploaded_by_auth_id = sender.auth_id
 WHERE m.conversation_id = $1 AND wc.consumer_id = $2 AND wc.provider_id = $3 AND m.id = ANY($4)`

func validateMessageAttachmentScope(scope conversation.MessageAttachmentScope) error {
	if scope.ConversationID <= 0 || scope.ConsumerID <= 0 || scope.ProviderID <= 0 || len(scope.MessageIDs) > conversation.MaxMessagePageSize {
		return fmt.Errorf("reading message attachments: %w", conversation.ErrInvalidMessagePage)
	}
	for _, id := range scope.MessageIDs {
		if id <= 0 {
			return fmt.Errorf("reading message attachments: %w", conversation.ErrInvalidMessagePage)
		}
	}
	return nil
}
