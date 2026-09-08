package evals

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/stretchr/testify/require"
)

func TestMapPDDoesNotExposeEvaluatorMetadata(t *testing.T) {
	var c PDCase
	require.NoError(t, json.Unmarshal([]byte(`{"id":"PD-001","title":"SECRET_TITLE","expected":{"facts":["SECRET_EXPECTATION"]},"input":{"user_message":"help","available_categories":["Plumbing"],"recent_messages":[{"sender_role":"assistant","content":"question","images":[{"file_id":"file","description":"historical"}]}]}}`), &c))
	question, categories, err := (&Dataset{}).MapPD(c.Input)
	require.NoError(t, err)
	data, err := json.Marshal(question)
	require.NoError(t, err)
	require.NotContains(t, string(data), "SECRET")
	require.Equal(t, conversation.SenderChatbot, question.RecentMessages[0].SenderRole)
	require.Equal(t, "historical", question.RecentMessages[0].Images[0].Description)
	require.Equal(t, "Plumbing", categories[0].Name)
}
func TestMapPDVerifiesImageBytesAtUseTime(t *testing.T) {
	root := t.TempDir()
	original := []byte("image bytes")
	img := ImageInput{AssetID: "IMG-01", Path: "image.png", SHA256: digest(original), FileID: "file", OriginalName: "image.png", MimeType: "image/png"}
	d := &Dataset{Root: root, assets: map[string]ImageInput{img.AssetID: img}}
	require.NoError(t, os.WriteFile(filepath.Join(root, img.Path), original, 0600))
	question, _, err := d.MapPD(PDInput{Images: []ImageInput{img}})
	require.NoError(t, err)
	require.Equal(t, original, question.Images[0].Data)
	require.Empty(t, question.Images[0].Description)
	require.NoError(t, os.WriteFile(filepath.Join(root, img.Path), []byte("changed"), 0600))
	_, _, err = d.MapPD(PDInput{Images: []ImageInput{img}})
	require.ErrorIs(t, err, ErrInvalidDataset)
}
func TestRankingMappingCopiesEvidenceWithoutIdentity(t *testing.T) {
	input := RKInput{MaxResults: 3, Candidates: []CandidateInput{{Reference: "candidate", Evidence: EvidenceInput{RatingDistribution: []int{0, 0, 0, 0, 1}, WorkHistory: []WorkInput{{ID: 10, Review: &ReviewInput{Rating: 5, Description: "review"}}}}}}}
	request, err := input.DomainRequest()
	require.NoError(t, err)
	require.Zero(t, request.Candidates[0].ProviderID)
	require.True(t, request.Candidates[0].Evidence.MostRecentPaidWork.IsZero())
	input.Candidates[0].Evidence.WorkHistory[0].Review.Description = "changed"
	require.Equal(t, "review", request.Candidates[0].Evidence.WorkHistory[0].Review.Description)
}
func TestRankingMappingRejectsWrongDistributionLength(t *testing.T) {
	_, err := (RKInput{MaxResults: 3, Candidates: []CandidateInput{{Reference: "candidate", Evidence: EvidenceInput{RatingDistribution: []int{1}}}}}).DomainRequest()
	require.ErrorIs(t, err, ErrInvalidDataset)
}

func TestMapPDRejectsUnknownImageWithoutFallback(t *testing.T) {
	_, _, err := (&Dataset{Root: t.TempDir()}).MapPD(PDInput{Images: []ImageInput{{AssetID: "unknown"}}})
	require.ErrorIs(t, err, ErrInvalidDataset)
}
func TestMapPDRejectsUnknownRole(t *testing.T) {
	_, _, err := (&Dataset{}).MapPD(PDInput{RecentMessages: []MessageInput{{SenderRole: "system"}}})
	require.ErrorIs(t, err, ErrInvalidDataset)
}

func TestRankingMappingRejectsNonpositiveMaximum(t *testing.T) {
	_, err := (RKInput{}).DomainRequest()
	require.ErrorContains(t, err, "max results must be positive")
}
