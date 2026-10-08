package repositories_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestClaimAdministrationUnitOfWorkRollsBackEveryComponentWithPostgres(t *testing.T) {
	for _, tc := range []struct{ stage, context, code, constraint string }{
		{"claim", "saving claim lifecycle", "23514", "claims_lifecycle_check"},
		{"action", "saving claim action", "23503", "claim_actions_actor_id_fkey"},
		{"resolution", "saving claim resolution", "23514", "claim_resolutions_type_check"},
		{"audit", "saving audit event", "23505", "audit_events_pkey"},
		{"record", "saving claim administration record", "23503", "claim_administration_records_action_id_fkey"},
	} {
		stage := tc.stage
		t.Run(stage, func(t *testing.T) {
			fixture := newOperationInboxFixture(t)
			consumer, provider := savedJobRequestParticipants(t, fixture.testContext)
			ctx := context.Background()
			now := time.Now().UTC().Truncate(time.Microsecond)
			request := fixture.jobRequest(t, consumer, provider, now.Add(-time.Hour), "pending")
			operator, err := admin.NewAdmin("auth0|claim-operator", "operator@example.com", "Operator", "Local", nil)
			require.NoError(t, err)
			_, err = fixture.testContext.userRepository.Save(ctx, operator)
			require.NoError(t, err)
			db := fixture.testContext.database
			repo := repositories.NewClaimRepository(db)
			events := repositories.NewAuditEventRepository(db)
			unit := repositories.NewClaimAdministrationUnitOfWork(db, repo, events)
			ref, err := claim.NewReference(claim.ReferenceKindJobRequest, fmt.Sprint(request.ID))
			require.NoError(t, err)
			found, err := claim.New(claim.Claimant{ID: consumer, Party: claim.PartyConsumer}, operationmodel.ID{Kind: operationmodel.KindJobRequest, ResourceID: request.ID}, uuid.NewString(), claim.Submission{Reference: ref, Reason: claim.ReasonDamage, Description: "original"}, now.Add(-time.Minute))
			require.NoError(t, err)
			_, err = found.StartReview(operator.ID(), now)
			require.NoError(t, err)
			require.NoError(t, repo.Save(ctx, found))
			eventID := uuid.New()
			correlation := "rollback-" + uuid.NewString()
			event, err := audit.NewEvent(audit.EventParams{ID: eventID, OperatorID: operator.ID(), Action: audit.ActionExecute, ResourceType: "claim", ResourceID: fmt.Sprint(found.ID), OccurredOn: now, Result: audit.ResultSucceeded, CorrelationID: correlation})
			require.NoError(t, err)
			if stage == "audit" {
				require.NoError(t, events.Save(ctx, event))
			}
			err = unit.Execute(ctx, func(store claim.AdministrationStore) error {
				loaded, err := store.FindClaim(ctx, found.ID)
				if err != nil {
					return err
				}
				action, err := loaded.Resolve(operator.ID(), claim.ResolutionInput{Type: claim.ResolutionTypeAgreement, Reasoning: "verified"}, now.Add(time.Minute))
				if err != nil {
					return err
				}
				// Malformed persistence inputs force actual PostgreSQL failures at each table boundary.
				switch stage {
				case "claim":
					loaded.ClosedOn = nil
				case "action":
					action.ActorID = -1
				case "resolution":
					loaded.Resolution.Type = "invalid"
				}
				if err := store.SaveClaim(ctx, loaded); err != nil {
					return err
				}
				if err := store.SaveAuditEvent(ctx, event); err != nil {
					return err
				}
				actionID := action.ID
				if stage == "record" {
					actionID = -1
				}
				return store.SaveRecord(ctx, &claim.AdministrationRecord{OperatorID: operator.ID(), Key: uuid.NewString(), Fingerprint: found.SubmissionFingerprint(), ClaimID: found.ID, ActionID: actionID})
			})
			require.ErrorContains(t, err, tc.context)
			var pgError *pgconn.PgError
			require.True(t, errors.As(err, &pgError), "the failure must retain its PostgreSQL cause")
			require.Equal(t, tc.code, pgError.Code)
			require.Equal(t, tc.constraint, pgError.ConstraintName)
			if stage == "audit" {
				require.ErrorIs(t, err, audit.ErrPersistence)
			}
			persisted, err := repo.FindAdministrativeByID(ctx, found.ID)
			require.NoError(t, err)
			require.Equal(t, claim.StatusInReview, persisted.Claim.Status)
			require.Nil(t, persisted.Claim.ClosedOn)
			require.Nil(t, persisted.Claim.Resolution)
			require.Equal(t, found.Actions, persisted.Claim.Actions)
			var count int
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM claim_administration_records WHERE operator_id=$1`, operator.ID()).Scan(&count))
			require.Zero(t, count)
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE correlation_id=$1`, correlation).Scan(&count))
			expected := 0
			if stage == "audit" {
				expected = 1
			}
			require.Equal(t, expected, count)
		})
	}
}
