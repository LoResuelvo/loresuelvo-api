package conversation

import "errors"

var ErrProviderRequired = errors.New("Provider id is required")

var ErrMessageRequired = errors.New("Message is required")

var ErrConsumerRequired = errors.New("Consumer id is required")

var ErrAlreadyExists = errors.New("Conversation already exists")

var ErrConversationDoesNotExist = errors.New("Conversation does not exist")

var ErrConversationAccessDenied = errors.New("Cannot access conversation")

var ErrPendingConversationMessageLimitReached = errors.New("Pending conversation message limit reached")

var ErrPendingConversationRequiresAcceptance = errors.New("Pending conversation must be accepted before provider can send messages")

var ErrOnlyPendingConversationCanBeActivated = errors.New("Only pending conversations can be activated")

var ErrOnlyConsumerCanMessageChatbot = errors.New("Only consumers can send messages to the AI chatbot")

var ErrChatbotResponseRequired = errors.New("Chatbot response is required")

var ErrChatbotUnavailable = errors.New("Chatbot is unavailable")

var ErrChatbotConversationAlreadyProcessing = errors.New("Chatbot conversation is already processing another message")

var ErrChatbotContextInvalid = errors.New("Chatbot context is invalid")

var ErrInvalidChatbotTurn = errors.New("Chatbot turn is invalid")

var ErrOnlyConsumerCanListChatbotConversations = errors.New("Only consumers can list AI chatbot conversations")
