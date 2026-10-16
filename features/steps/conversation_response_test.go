package steps_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/storage"
)

type conversationCounterpartResponse struct {
	ID              int    `json:"id"`
	Role            string `json:"role"`
	Name            string `json:"name"`
	Surname         string `json:"surname"`
	ProfilePhotoURL string `json:"profile_photo_url,omitempty"`
	CategoryName    string `json:"category_name,omitempty"`
}

func (suite *testSuite) onlyConversationCounterpart() (map[string]json.RawMessage, error) {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return nil, err
	}
	var conversation map[string]json.RawMessage
	if strings.HasPrefix(strings.TrimSpace(string(suite.lastBody)), "[") {
		var conversations []map[string]json.RawMessage
		if err := json.Unmarshal(suite.lastBody, &conversations); err != nil {
			return nil, err
		}
		if len(conversations) != 1 {
			return nil, fmt.Errorf("expected exactly one conversation, got %d", len(conversations))
		}
		conversation = conversations[0]
	} else {
		var detail struct {
			Work map[string]json.RawMessage `json:"work"`
		}
		if err := json.Unmarshal(suite.lastBody, &detail); err != nil {
			return nil, err
		}
		conversation = detail.Work
	}
	var counterpart map[string]json.RawMessage
	if err := json.Unmarshal(conversation["counterpart"], &counterpart); err != nil {
		return nil, err
	}
	if counterpart == nil {
		return nil, fmt.Errorf("expected conversation counterpart")
	}
	return counterpart, nil
}

func (suite *testSuite) conversationCounterpartIsConsumerWithPhoto(fullName string) error {
	raw, err := suite.onlyConversationCounterpart()
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	var counterpart conversationCounterpartResponse
	if err := json.Unmarshal(encoded, &counterpart); err != nil {
		return err
	}
	if !conversationCounterpartMatches(counterpart, participantRoleConsumer, fullName) {
		return fmt.Errorf("expected consumer counterpart %q, got %s", fullName, encoded)
	}
	consumer, err := suite.userRepository.FindByID(suite.scenarioContext, counterpart.ID)
	if err != nil {
		return err
	}
	if consumer.Role() != participantRoleConsumer || strings.TrimSpace(consumer.Name()+" "+consumer.Surname()) != fullName {
		return fmt.Errorf("counterpart ID %d does not identify consumer %q", counterpart.ID, fullName)
	}
	if consumer.ProfilePhoto() == nil {
		return fmt.Errorf("expected consumer %q fixture to have a profile photo", fullName)
	}
	photo, err := suite.fileRepository.FindByID(suite.scenarioContext, consumer.ProfilePhoto().FileID)
	if err != nil {
		return err
	}
	components, err := storage.NewComponentsFromEnv()
	if err != nil {
		return err
	}
	expectedURL := components.Storage.PublicURL(photo.Bucket, photo.Key)
	if strings.TrimSpace(counterpart.ProfilePhotoURL) == "" || counterpart.ProfilePhotoURL != expectedURL {
		return fmt.Errorf("expected consumer counterpart profile_photo_url %q, got %q", expectedURL, counterpart.ProfilePhotoURL)
	}
	return nil
}

func (suite *testSuite) conversationCounterpartDoesNotIncludeCategory() error {
	counterpart, err := suite.onlyConversationCounterpart()
	if err != nil {
		return err
	}
	if _, exists := counterpart["category_name"]; exists {
		return fmt.Errorf("expected consumer counterpart without category_name, got %s", counterpart["category_name"])
	}
	return nil
}
