package conversation_handler

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

func sentMessageResponseFromDomain(message conversation.Message) sentMessageResponse {
	return sentMessageResponse{
		ID:             message.ID,
		ConversationID: message.ConversationID,
		SenderRole:     message.SenderRole,
		Content:        message.Content,
		Images:         messageImageResponsesFromDomain(message.Images),
		Audio:          messageAudioResponseFromDomain(message.Audio),
		Video:          messageVideoResponseFromDomain(message.Video),
		CreatedOn:      message.CreatedOn,
	}
}

func chatbotConversationResponseFromDomain(result conversation.ChatbotConversationResult) chatbotConversationResponse {
	chatbotConversation, _ := result.Conversation.(*conversation.ChatBotConversation)
	messages := make([]conversationMessageResponse, 0, len(result.Conversation.Messages()))
	var chatbotResponse *conversationMessageResponse
	for _, message := range result.Conversation.Messages() {
		messageResponse := conversationMessageResponse{
			ID:         message.ID,
			SenderRole: message.SenderRole,
			Content:    message.Content,
			Images:     messageImageResponsesFromDomain(message.Images),
			Audio:      messageAudioResponseFromDomain(message.Audio),
			Video:      messageVideoResponseFromDomain(message.Video),
			CreatedOn:  message.CreatedOn,
		}
		messages = append(messages, messageResponse)
		if message.SenderRole == conversation.SenderChatbot {
			chatbotResponse = &messageResponse
		}
	}

	title := ""
	if chatbotConversation != nil {
		title = chatbotConversation.Title
	}

	var problemCategory *problemCategoryResponse
	if result.ProblemCategory != nil {
		problemCategory = &problemCategoryResponse{
			ID:   result.ProblemCategory.ID,
			Name: result.ProblemCategory.Name,
		}
	}
	return chatbotConversationResponse{
		ID:     result.Conversation.ID(),
		Status: result.Conversation.Status(),
		chatbotConversationDetail: chatbotConversationDetail{
			Title:                title,
			ResponseStatus:       string(result.ResponseStatus),
			Assessment:           assessmentResponse(result.Assessment != nil, assessmentOutcome(result.Assessment), problemCategory),
			RecommendedProviders: providerSummaryResponsesFromDomain(result.RecommendedProviders),
		},
		Messages: messages,
		Response: chatbotResponse,
	}
}

func conversationDetailResponseFromDomain(foundConversation readmodel.ConversationDetail) conversationDetailResponse {
	messages := make([]conversationMessageResponse, 0, len(foundConversation.Messages))
	for _, message := range foundConversation.Messages {
		messages = append(messages, conversationMessageResponse{
			ID:         message.ID,
			SenderRole: message.SenderRole,
			Content:    message.Content,
			Images:     messageImageResponsesFromDomain(message.Images),
			Audio:      messageAudioResponseFromDomain(message.Audio),
			Video:      messageVideoResponseFromDomain(message.Video),
			CreatedOn:  message.CreatedOn,
		})
	}

	return conversationDetailResponse{
		ID:        foundConversation.ID,
		Type:      foundConversation.Type,
		Status:    foundConversation.Status,
		Work:      workConversationDetailResponseFromDomain(foundConversation.Work),
		Chatbot:   chatbotConversationDetailResponseFromDomain(foundConversation.Chatbot),
		Messages:  messages,
		UpdatedOn: foundConversation.UpdatedOn,
	}
}

func messageImageResponsesFromDomain(images []filedomain.MessageImage) []messageImageResponse {
	response := make([]messageImageResponse, 0, len(images))
	for _, image := range images {
		response = append(response, messageImageResponse{ID: image.FileID, URL: image.URL, OriginalName: image.OriginalName})
	}
	return response
}

func messageAudioResponseFromDomain(audio *filedomain.MessageAudio) *messageAudioResponse {
	if audio == nil {
		return nil
	}

	return &messageAudioResponse{
		ID:              audio.FileID,
		URL:             audio.URL,
		OriginalName:    audio.OriginalName,
		MimeType:        audio.MimeType,
		Codec:           audio.Codec,
		DurationSeconds: audio.DurationSeconds,
	}
}

func messageVideoResponseFromDomain(video *filedomain.MessageVideo) *messageVideoResponse {
	if video == nil {
		return nil
	}

	return &messageVideoResponse{
		ID:              video.FileID,
		URL:             video.URL,
		OriginalName:    video.OriginalName,
		MimeType:        video.MimeType,
		VideoCodec:      video.VideoCodec,
		AudioCodec:      video.AudioCodec,
		DurationSeconds: video.DurationSeconds,
		Width:           video.Width,
		Height:          video.Height,
	}
}

func workConversationDetailResponseFromDomain(detail *readmodel.WorkConversationDetail) *workConversationDetail {
	if detail == nil {
		return nil
	}

	return &workConversationDetail{
		Counterpart: conversationCounterpartResponseFromDomain(detail.Counterpart),
	}
}

func chatbotConversationDetailResponseFromDomain(detail *readmodel.ChatbotConversationDetail) *chatbotConversationDetail {
	if detail == nil {
		return nil
	}

	var problemCategory *problemCategoryResponse
	if detail.Assessment != nil && detail.Assessment.ProblemCategory != nil {
		problemCategory = &problemCategoryResponse{
			ID:   detail.Assessment.ProblemCategory.ID,
			Name: detail.Assessment.ProblemCategory.Name,
		}
	}

	return &chatbotConversationDetail{
		Title:                detail.Title,
		ResponseStatus:       detail.ResponseStatus,
		Assessment:           assessmentResponse(detail.Assessment != nil, readModelAssessmentOutcome(detail.Assessment), problemCategory),
		RecommendedProviders: providerSummaryResponsesFromDomain(detail.RecommendedProviders),
	}
}

func assessmentOutcome(assessment *conversation.ProblemAssessment) string {
	if assessment == nil {
		return ""
	}
	return string(assessment.Outcome)
}

func readModelAssessmentOutcome(assessment *readmodel.ProblemAssessmentDetail) string {
	if assessment == nil {
		return ""
	}
	return assessment.Outcome
}

func assessmentResponse(present bool, outcome string, category *problemCategoryResponse) *problemAssessmentResponse {
	if !present {
		return nil
	}
	return &problemAssessmentResponse{Outcome: outcome, ProblemCategory: category}
}

func providerSummaryResponseFromDomain(provider provider.Provider) providerSummaryResponse {
	profilePhotoURL := ""
	if provider.ProfilePhoto() != nil {
		profilePhotoURL = provider.ProfilePhoto().URL
	}
	return providerSummaryResponse{
		ID:              provider.ID(),
		Name:            provider.Name(),
		Surname:         provider.Surname(),
		CategoryName:    provider.Categoryname(),
		ProfilePhotoURL: profilePhotoURL,
	}
}

func providerSummaryResponsesFromDomain(providers []provider.Provider) []providerSummaryResponse {
	response := make([]providerSummaryResponse, 0, len(providers))
	for _, foundProvider := range providers {
		response = append(response, providerSummaryResponseFromDomain(foundProvider))
	}

	return response
}

func chatbotConversationSummaryResponsesFromDomain(summaries []readmodel.ConversationSummary) []chatbotConversationSummaryResponse {
	response := make([]chatbotConversationSummaryResponse, 0, len(summaries))
	for _, summary := range summaries {
		response = append(response, chatbotConversationSummaryResponseFromDomain(summary))
	}

	return response
}

func chatbotConversationSummaryResponseFromDomain(summary readmodel.ConversationSummary) chatbotConversationSummaryResponse {
	title := ""
	if summary.Chatbot != nil {
		title = summary.Chatbot.Title
	}

	return chatbotConversationSummaryResponse{
		conversationSummaryResponse: baseConversationSummaryResponseFromDomain(summary),
		Title:                       title,
	}
}

func conversationSummaryResponsesFromDomain(summaries []readmodel.ConversationSummary) []workConversationSummaryResponse {
	response := make([]workConversationSummaryResponse, 0, len(summaries))
	for _, summary := range summaries {
		response = append(response, conversationSummaryResponseFromDomain(summary))
	}

	return response
}

func conversationSummaryResponseFromDomain(summary readmodel.ConversationSummary) workConversationSummaryResponse {
	var counterpart readmodel.ConversationParticipant
	if summary.Work != nil {
		counterpart = summary.Work.Counterpart
	}

	return workConversationSummaryResponse{
		conversationSummaryResponse: baseConversationSummaryResponseFromDomain(summary),
		Counterpart:                 conversationCounterpartResponseFromDomain(counterpart),
	}
}

func baseConversationSummaryResponseFromDomain(summary readmodel.ConversationSummary) conversationSummaryResponse {
	return conversationSummaryResponse{
		ID:          summary.ID,
		Status:      summary.Status,
		LastMessage: conversationLastMessageResponseFromDomain(summary.LastMessage),
		UpdatedOn:   summary.UpdatedOn,
	}
}

func conversationCounterpartResponseFromDomain(counterpart readmodel.ConversationParticipant) conversationCounterpartResponse {
	return conversationCounterpartResponse{
		ID:              counterpart.ID,
		Role:            counterpart.Role,
		Name:            counterpart.Name,
		Surname:         counterpart.Surname,
		CategoryName:    counterpart.CategoryName,
		ProfilePhotoURL: counterpart.ProfilePhotoURL,
	}
}

func conversationLastMessageResponseFromDomain(message *readmodel.MessageSummary) *conversationLastMessageResponse {
	if message == nil {
		return nil
	}

	return &conversationLastMessageResponse{
		ID:         message.ID,
		SenderRole: message.SenderRole,
		Content:    message.Content,
		Audio:      messageAudioResponseFromDomain(message.Audio),
		CreatedOn:  message.CreatedOn,
	}
}
