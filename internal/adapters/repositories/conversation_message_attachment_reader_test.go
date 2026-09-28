package repositories_test

import (
	"context"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestConversationMessageAttachmentReaderScopesAllMediaToPageConversationAndSender(t *testing.T) {
	fixture := newOperationInboxFixture(t)
	consumerID, providerID := savedJobRequestParticipants(t, fixture.testContext)
	now := time.Now().UTC().Truncate(time.Microsecond)
	request := fixture.jobRequest(t, consumerID, providerID, now, jobrequest.StatusAccepted)
	otherConsumer := savedConsumerIDWithData(t, fixture.testContext, "auth0|media-other", "media.other@example.com", "Other", "Consumer")
	foreign := fixture.jobRequest(t, otherConsumer, providerID, now, jobrequest.StatusAccepted)
	database := fixture.testContext.database
	var createdMessageIDs []int
	var createdFileIDs []string
	t.Cleanup(func() {
		if len(createdMessageIDs) > 0 {
			_, err := database.Exec(`DELETE FROM messages WHERE id=ANY($1)`, createdMessageIDs)
			require.NoError(t, err)
		}
		if len(createdFileIDs) > 0 {
			_, err := database.Exec(`DELETE FROM files WHERE id=ANY($1::uuid[])`, createdFileIDs)
			require.NoError(t, err)
		}
	})
	var senderAuth, foreignAuth string
	require.NoError(t, database.QueryRow(`SELECT auth_id FROM users WHERE id=$1`, consumerID).Scan(&senderAuth))
	require.NoError(t, database.QueryRow(`SELECT auth_id FROM users WHERE id=$1`, otherConsumer).Scan(&foreignAuth))
	insertMessage := func(conversationID int, role string) int {
		var id int
		require.NoError(t, database.QueryRow(`INSERT INTO messages(conversation_id,sender_role,content,created_on) VALUES($1,$2,'Media',$3) RETURNING id`, conversationID, role, now).Scan(&id))
		createdMessageIDs = append(createdMessageIDs, id)
		return id
	}
	selected := insertMessage(request.ConversationID, "consumer")
	outside := insertMessage(request.ConversationID, "consumer")
	foreignMessage := insertMessage(foreign.ConversationID, "consumer")
	wrongSender := insertMessage(request.ConversationID, "provider")
	attach := func(messageID int, owner string) []string {
		result := make([]string, 0, 3)
		for _, kind := range []string{"image", "audio", "video"} {
			id := uuid.NewString()
			result = append(result, id)
			mime := map[string]string{"image": "image/jpeg", "audio": "audio/ogg", "video": "video/mp4"}[kind]
			_, err := database.Exec(`INSERT INTO files(id,key,bucket,original_name,mime_type,size_bytes,status,visibility,purpose,uploaded_by_auth_id,created_on,updated_on) VALUES($1,$6,'private-bucket','private-file',$2,100,'confirmed','private',$3,$4,$5,$5)`, id, mime, "conversation_message_"+kind, owner, now, "private-chat-test/"+id)
			require.NoError(t, err)
			createdFileIDs = append(createdFileIDs, id)
			// Audio/video message associations reference their persisted subtype metadata.
			switch kind {
			case "audio":
				_, err = database.Exec(`INSERT INTO file_audios(file_id,codec,duration_seconds) VALUES($1,'opus',4)`, id)
				require.NoError(t, err)
			case "video":
				_, err = database.Exec(`INSERT INTO file_videos(file_id,video_codec,audio_codec,duration_seconds,width,height) VALUES($1,'h264','aac',5,640,480)`, id)
				require.NoError(t, err)
			}
			table := map[string]string{"image": "message_images", "audio": "message_audios", "video": "message_videos"}[kind]
			query := `INSERT INTO ` + table + `(message_id,file_id) VALUES($1,$2)`
			if kind == "image" {
				query = `INSERT INTO message_images(message_id,file_id,position,description) VALUES($1,$2,0,'Evidence')`
			}
			_, err = database.Exec(query, messageID, id)
			require.NoError(t, err)
		}
		return result
	}
	selectedIDs := attach(selected, senderAuth)
	attach(outside, senderAuth)
	attach(foreignMessage, foreignAuth)
	attach(wrongSender, senderAuth)
	reader := repositories.NewConversationMessageAttachmentReader(repositories.NewMessageImageRepository(database), repositories.NewMessageAudioRepository(database), repositories.NewMessageVideoRepository(database))
	scope := conversation.MessageAttachmentScope{ConversationID: request.ConversationID, ConsumerID: consumerID, ProviderID: providerID, MessageIDs: []int{selected, foreignMessage, wrongSender}}
	refs, err := reader.FindByMessagePage(context.Background(), scope)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, selectedIDs[0], refs[selected].Images[0].FileID)
	require.Equal(t, "Evidence", refs[selected].Images[0].Description)
	require.Equal(t, selectedIDs[1], refs[selected].AudioFileID)
	require.Equal(t, selectedIDs[2], refs[selected].VideoFileID)
	require.NotContains(t, refs, outside)
	require.NotContains(t, refs, foreignMessage)
	require.NotContains(t, refs, wrongSender)
	scope.ConsumerID = otherConsumer
	refs, err = reader.FindByMessagePage(context.Background(), scope)
	require.NoError(t, err)
	require.Empty(t, refs)
}
