package readmodel_test

import (
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConversationAssociationIsSharedOnlyForMultipleProposals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ids    []int
		shared bool
	}{{"none", nil, false}, {"one", []int{1}, false}, {"multiple", []int{1, 2}, true}} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.shared, (readmodel.ConversationAssociation{RelatedServiceProposalIDs: tc.ids}).IsShared())
		})
	}
}
