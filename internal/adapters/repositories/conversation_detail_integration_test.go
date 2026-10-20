package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestConversationReaderDetailHydratesPersistedAudioAndVideoMetadata(t *testing.T) {
	fixture := newSavedConversationReaderFixture(t)
	audioMetadata, err := filedomain.NewAudioFileMetadata("pump.webm", "audio/webm", 1024, 18, "opus")
	require.NoError(t, err)
	videoMetadata, err := filedomain.NewVideoFileMetadata("leak.mp4", "video/mp4", 2048, filedomain.VideoMetadata{DurationSeconds: 23, VideoCodec: "h264", AudioCodec: "aac", Width: 1280, Height: 720})
	require.NoError(t, err)
	audioFile, err := filedomain.NewFile("00000000-0000-0000-0000-000000000011", "conversation/audio/pump.webm", "private", *audioMetadata, filedomain.StatusConfirmed, filedomain.VisibilityPrivate, filedomain.PurposeConversationMessageAudio, "auth0|conversation-consumer", time.Now().UTC(), time.Now().UTC())
	require.NoError(t, err)
	videoFile, err := filedomain.NewFile("00000000-0000-0000-0000-000000000012", "conversation/video/leak.mp4", "private", *videoMetadata, filedomain.StatusConfirmed, filedomain.VisibilityPrivate, filedomain.PurposeConversationMessageVideo, "auth0|conversation-consumer", time.Now().UTC(), time.Now().UTC())
	require.NoError(t, err)
	files := repositories.NewFileRepository(fixture.database)
	require.NoError(t, files.Save(context.Background(), *audioFile))
	require.NoError(t, files.Save(context.Background(), *videoFile))
	expectedAudio := filedomain.MessageAudio{FileID: audioFile.ID, OriginalName: audioFile.OriginalName(), MimeType: audioFile.MimeType(), Codec: audioFile.Codec(), DurationSeconds: audioFile.DurationSeconds()}
	expectedVideo := filedomain.MessageVideo{FileID: videoFile.ID, OriginalName: videoFile.OriginalName(), MimeType: videoFile.MimeType(), VideoCodec: videoFile.VideoCodec(), AudioCodec: videoFile.AudioCodec(), DurationSeconds: videoFile.DurationSeconds(), Width: videoFile.Width(), Height: videoFile.Height()}
	audio, err := conversation.NewConsumerAudioMessage(expectedAudio)
	require.NoError(t, err)
	video, err := conversation.NewConsumerVideoMessage("Leak", expectedVideo)
	require.NoError(t, err)
	fixture.savedConversation.AddMessage(*audio)
	fixture.savedConversation.AddMessage(*video)
	_, err = fixture.conversationRepository.SaveConversation(context.Background(), fixture.savedConversation)
	require.NoError(t, err)
	for _, role := range []string{conversation.SenderConsumer, conversation.SenderProvider} {
		detail, err := fixture.reader.FindDetailByIDRoleAndType(context.Background(), fixture.savedConversation.ID(), role, conversation.TypeWork)
		require.NoError(t, err)
		require.Len(t, detail.Messages, 3)
		require.Equal(t, &expectedAudio, detail.Messages[1].Audio)
		require.Equal(t, &expectedVideo, detail.Messages[2].Video)
	}
}

func TestGetByIDSeesMessageCommittedBetweenAccessAndFinalRead(t *testing.T) {
	fixture := newSavedConversationReaderFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	projected := make(chan struct{})
	resume := make(chan struct{})
	reader := &conversationAccessReadBarrier{Reader: fixture.reader}
	reader.On("FindAccessByID", mock.Anything, fixture.savedConversation.ID()).Run(func(mock.Arguments) {
		close(projected)
		select {
		case <-resume:
		case <-ctx.Done():
		}
	}).Return().Once()
	files := &conversationEmptyFileServiceMock{}
	files.On("ResolvePublicURL", mock.Anything, mock.Anything).Return("https://files.test/profile.jpg", nil).Twice()
	files.On("PrepareMessageImages", mock.Anything, "auth0|conversation-consumer", mock.Anything).Return([]filedomain.MessageImage{}, nil).Once()
	files.On("ResolveMessageImages", mock.Anything, mock.Anything).Return(map[string]filedomain.MessageImage{}, nil).Twice()
	files.On("ResolveMessageAudios", mock.Anything, mock.Anything).Return(map[string]filedomain.MessageAudio{}, nil).Twice()
	files.On("ResolveMessageVideos", mock.Anything, mock.Anything).Return(map[string]filedomain.MessageVideo{}, nil).Twice()
	publisher := &conversationMessagePublisherMock{}
	publisher.On("PublishMessage", mock.Anything, mock.Anything, "auth0|conversation-consumer", mock.Anything).Return().Once()
	users := repositories.NewUserRepository(fixture.database)
	getter := conversation.NewService(nil, users, reader, nil, nil, nil, files, nil, conversation.DefaultProviderRecommendationConfig(), nil)
	sender := conversation.NewService(fixture.conversationRepository, users, fixture.reader, publisher, nil, nil, files, nil, conversation.DefaultProviderRecommendationConfig(), nil)
	result := make(chan struct {
		detail *readmodel.ConversationDetail
		err    error
	}, 1)
	go func() {
		detail, err := getter.GetByID(ctx, "auth0|conversation-consumer", fixture.savedConversation.ID())
		result <- struct {
			detail *readmodel.ConversationDetail
			err    error
		}{detail, err}
	}()
	select {
	case <-projected:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	sent, err := sender.SendMessage(ctx, "auth0|conversation-consumer", fixture.savedConversation.ID(), "Confirmed while GET is in progress", nil, "")
	close(resume)
	require.NoError(t, err)
	var found *readmodel.ConversationDetail
	select {
	case readResult := <-result:
		require.NoError(t, readResult.err)
		found = readResult.detail
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.Len(t, found.Messages, 2)
	require.Equal(t, sent.ID, found.Messages[1].ID)
	require.Equal(t, sent.Content, found.Messages[1].Content)
	require.False(t, found.UpdatedOn.Before(sent.CreatedOn))
	// A subsequent GET also observes the already committed send.
	subsequent := conversation.NewService(nil, users, fixture.reader, nil, nil, nil, files, nil, conversation.DefaultProviderRecommendationConfig(), nil)
	detail, err := subsequent.GetByID(ctx, "auth0|conversation-consumer", fixture.savedConversation.ID())
	require.NoError(t, err)
	require.Equal(t, found, detail)
	reader.AssertExpectations(t)
	files.AssertExpectations(t)
	publisher.AssertExpectations(t)
}
