package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	httpadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/lifecycle"
)

// PaymentsDemoModeVariable is the env var that opts the application into the
// in-process payment demo simulator. The simulator synthesises an approved
// payment through the standard pipeline so the post-checkout flow can be
// exercised end-to-end without an external webhook from the payment gateway.
// The simulator is a development-only tool — see ValidatePaymentsDemoMode for
// the environment restrictions.
const PaymentsDemoModeVariable = "PAYMENTS_DEMO_MODE"

// ErrPaymentsDemoModeMisconfigured is returned when PAYMENTS_DEMO_MODE=true is
// combined with an environment that does not permit the demo simulator.
var ErrPaymentsDemoModeMisconfigured = errors.New("PAYMENTS_DEMO_MODE is only allowed in development or test")

type Application struct {
	coordinator *lifecycle.Coordinator
}

func NewApplication(ctx context.Context, database *sql.DB, logger *slog.Logger) (*Application, error) {
	if err := SeedDefaultDataFromEnv(ctx, database); err != nil {
		return nil, fmt.Errorf("seeding default data: %w", err)
	}

	environment := routerEnvironmentFromEnv()
	paymentsDemoMode, err := ParsePaymentsDemoMode(os.Getenv(PaymentsDemoModeVariable))
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", PaymentsDemoModeVariable, err)
	}
	if err := ValidatePaymentsDemoMode(paymentsDemoMode, environment); err != nil {
		return nil, err
	}
	if paymentsDemoMode {
		logger.Warn("⚠️ PAYMENT DEMO MODE ENABLED - payments are simulated")
	}

	dependencies, err := NewDependencies(database, paymentsDemoMode)
	if err != nil {
		return nil, fmt.Errorf("building application dependencies: %w", err)
	}
	auth0Validator, err := auth0.NewValidatorFromEnv()
	if err != nil {
		return nil, fmt.Errorf("configuring Auth0 validator: %w", err)
	}

	router := httpadapter.NewRouter(dependencies.RouterConfig(auth0Validator, logger, environment))
	engine, err := router.SetUp()
	if err != nil {
		return nil, fmt.Errorf("setting up HTTP router: %w", err)
	}

	server := &http.Server{Addr: ":8080", Handler: engine}
	coordinator, err := lifecycle.NewCoordinator(dependencies.Runtime.lifecycleConfig(server, database, logger))
	if err != nil {
		return nil, fmt.Errorf("configuring application lifecycle: %w", err)
	}
	return &Application{coordinator: coordinator}, nil
}

func (application *Application) Run(ctx context.Context) error {
	return application.coordinator.Run(ctx)
}

// ParsePaymentsDemoMode parses the raw env var. Only "true"/"1" (case-
// insensitive, trimmed) enables the simulator; anything else, including the
// empty string, is treated as disabled.
func ParsePaymentsDemoMode(raw string) (bool, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "", "false", "0":
		return false, nil
	case "true", "1":
		return true, nil
	default:
		return false, fmt.Errorf(
			"%s must be one of: true, false, 1, 0 (got %q)",
			PaymentsDemoModeVariable,
			raw,
		)
	}
}

// ValidatePaymentsDemoMode enforces that the demo simulator can only be
// enabled in environments where it cannot leak into production. Test and
// development are both accepted so the feature can be exercised by the BDD
// suite and local docker-compose stack.
func ValidatePaymentsDemoMode(enabled bool, environment httpadapter.Environment) error {
	if !enabled {
		return nil
	}
	switch environment {
	case httpadapter.DevelopmentEnvironment, httpadapter.TestEnvironment:
		return nil
	default:
		return fmt.Errorf("%w (current environment: %q)", ErrPaymentsDemoModeMisconfigured, environment)
	}
}

func routerEnvironmentFromEnv() httpadapter.Environment {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT"))) {
	case "dev", "development":
		return httpadapter.DevelopmentEnvironment
	case "test":
		return httpadapter.TestEnvironment
	case "staging":
		return httpadapter.StagingEnvironment
	case "production":
		return httpadapter.ProductionEnvironment
	default:
		return ""
	}
}
