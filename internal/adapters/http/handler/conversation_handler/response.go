package conversation_handler

import "time"

type sentMessageResponse struct {
	ID             int                    `json:"id"`
	ConversationID int                    `json:"conversation_id"`
	SenderRole     string                 `json:"sender_role"`
	Content        string                 `json:"content"`
	Images         []messageImageResponse `json:"images"`
	Audio          *messageAudioResponse  `json:"audio,omitempty"`
	Video          *messageVideoResponse  `json:"video,omitempty"`
	CreatedOn      time.Time              `json:"created_on"`
}

type conversationDetailResponse struct {
	ID        int                           `json:"id"`
	Type      string                        `json:"type"`
	Status    string                        `json:"status"`
	Work      *workConversationDetail       `json:"work,omitempty"`
	Chatbot   *chatbotConversationDetail    `json:"chatbot,omitempty"`
	Messages  []conversationMessageResponse `json:"messages"`
	UpdatedOn time.Time                     `json:"updated_on"`
}

type workConversationDetail struct {
	Counterpart conversationCounterpartResponse `json:"counterpart"`
}

type chatbotConversationDetail struct {
	Title                string                     `json:"title"`
	ResponseStatus       string                     `json:"response_status"`
	Assessment           *problemAssessmentResponse `json:"assessment,omitempty"`
	RecommendedProviders []providerSummaryResponse  `json:"recommended_providers"`
}

type problemAssessmentResponse struct {
	Outcome         string                   `json:"outcome"`
	ProblemCategory *problemCategoryResponse `json:"problem_category,omitempty"`
}

type problemCategoryResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type conversationMessageResponse struct {
	ID         int                    `json:"id"`
	SenderRole string                 `json:"sender_role"`
	Content    string                 `json:"content"`
	Images     []messageImageResponse `json:"images"`
	Audio      *messageAudioResponse  `json:"audio,omitempty"`
	Video      *messageVideoResponse  `json:"video,omitempty"`
	CreatedOn  time.Time              `json:"created_on"`
}

type messageImageResponse struct {
	ID           string `json:"id"`
	URL          string `json:"url"`
	OriginalName string `json:"original_name"`
}

type messageAudioResponse struct {
	ID              string `json:"id"`
	URL             string `json:"url"`
	OriginalName    string `json:"original_name"`
	MimeType        string `json:"mime_type"`
	Codec           string `json:"codec"`
	DurationSeconds int    `json:"duration_seconds"`
}

type messageVideoResponse struct {
	ID              string `json:"id"`
	URL             string `json:"url"`
	OriginalName    string `json:"original_name"`
	MimeType        string `json:"mime_type"`
	VideoCodec      string `json:"video_codec"`
	AudioCodec      string `json:"audio_codec,omitempty"`
	DurationSeconds int    `json:"duration_seconds"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
}

type conversationSummaryResponse struct {
	ID          int                              `json:"id"`
	Status      string                           `json:"status"`
	LastMessage *conversationLastMessageResponse `json:"last_message"`
	UpdatedOn   time.Time                        `json:"updated_on"`
}

type workConversationSummaryResponse struct {
	conversationSummaryResponse
	Counterpart conversationCounterpartResponse `json:"counterpart"`
}

type chatbotConversationSummaryResponse struct {
	conversationSummaryResponse
	Title string `json:"title"`
}

type conversationCounterpartResponse struct {
	ID              int    `json:"id"`
	Role            string `json:"role"`
	Name            string `json:"name"`
	Surname         string `json:"surname"`
	CategoryName    string `json:"category_name,omitempty"`
	ProfilePhotoURL string `json:"profile_photo_url,omitempty"`
}

type conversationLastMessageResponse struct {
	ID         int                   `json:"id"`
	SenderRole string                `json:"sender_role"`
	Content    string                `json:"content"`
	Audio      *messageAudioResponse `json:"audio,omitempty"`
	Video      *messageVideoResponse `json:"video,omitempty"`
	CreatedOn  time.Time             `json:"created_on"`
}

type providerSummaryResponse struct {
	ID                   int    `json:"id"`
	Name                 string `json:"name"`
	Surname              string `json:"surname"`
	CategoryName         string `json:"category_name"`
	ProfilePhotoURL      string `json:"profile_photo_url"`
	RecommendationReason string `json:"recommendation_reason,omitempty"`
}

type chatbotConversationResponse struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
	chatbotConversationDetail
	Messages []conversationMessageResponse `json:"messages"`
	Response *conversationMessageResponse  `json:"response,omitempty"`
}
