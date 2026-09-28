package conversation

import "context"

// MessageAttachmentScope binds bounded message IDs to the persisted work participants.
type MessageAttachmentScope struct {
	ConversationID int
	ConsumerID     int
	ProviderID     int
	MessageIDs     []int
}
type MessageImageReference struct {
	FileID      string
	Description string
}
type MessageAttachmentReferences struct {
	Images      []MessageImageReference
	AudioFileID string
	VideoFileID string
}

// MessageAttachmentReader returns only owned file references, never access URLs.
type MessageAttachmentReader interface {
	FindByMessagePage(ctx context.Context, scope MessageAttachmentScope) (map[int]MessageAttachmentReferences, error)
}
