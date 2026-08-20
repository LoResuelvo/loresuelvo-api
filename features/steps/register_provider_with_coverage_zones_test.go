package steps_test

import (
	"context"
	"fmt"
	"net/http"

	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/cucumber/godog"
)

func registerProviderWithCoverageZonesSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que están habilitadas las zonas de cobertura "([^"]*)", "([^"]*)" y "([^"]*)"$`, suite.thereAreEnabledCoverageZones)
	sc.Step(`^me registro como prestador con correo "([^"]*)", nombre "([^"]*)", apellido "([^"]*)", rubro "([^"]*)" y zona de cobertura "([^"]*)"$`, suite.requestProviderRegistrationWithCoverageZone)
	sc.Step(`^el prestador "([^"]*)" queda registrado con la zona de cobertura "([^"]*)"$`, suite.providerIsRegisteredWithCoverageZone)
}

func (suite *testSuite) thereAreEnabledCoverageZones(firstName, secondName, thirdName string) error {
	if err := suite.coverageZoneRepository.DeleteAll(); err != nil {
		return fmt.Errorf("could not reset coverage zones: %w", err)
	}

	for _, name := range []string{firstName, secondName, thirdName} {
		zone, err := coveragezone.New(name)
		if err != nil {
			return err
		}

		if _, err := suite.coverageZoneRepository.Save(context.Background(), *zone); err != nil {
			return fmt.Errorf("could not create coverage zone %q: %w", name, err)
		}
	}

	return nil
}

func (suite *testSuite) requestProviderRegistrationWithCoverageZone(email, name, surname, categoryName, coverageZoneName string) error {
	categoryID, err := suite.categoryIDFor(categoryName)
	if err != nil {
		return err
	}

	zone, err := suite.coverageZoneRepository.FindByName(context.Background(), coverageZoneName)
	if err != nil {
		return fmt.Errorf("coverage zone %q is not available for the scenario: %w", coverageZoneName, err)
	}

	return suite.requestProviderRegistration(providerRegistrationRequest{
		Email:                  email,
		Name:                   name,
		Surname:                surname,
		CategoryID:             categoryID,
		CoverageZoneIDs:        []int{zone.ID},
		CriminalRecordFile:     "criminal-record.pdf",
		CUITCertificateFile:    "cuit-certificate.pdf",
		BiometricValidationID:  "biometric-validation-approved",
		ProfessionalCredential: "professional-license-or-certificate.pdf",
		ProfilePhotoFileID:     suite.providerProfilePhotoFileID,
	})
}

func (suite *testSuite) providerIsRegisteredWithCoverageZone(email, expectedZoneName string) error {
	if err := suite.registrationResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}

	providerID, err := suite.userRepository.FindIDByEmail(email)
	if err != nil {
		return fmt.Errorf("could not find registered provider %q: %w", email, err)
	}

	zones, err := suite.coverageZoneRepository.FindByProviderID(context.Background(), providerID)
	if err != nil {
		return err
	}
	if len(zones) != 1 {
		return fmt.Errorf("expected provider %q to have exactly one coverage zone, got %d", email, len(zones))
	}
	if !sameNormalizedName(zones[0].Name, expectedZoneName) {
		return fmt.Errorf("expected provider %q to have coverage zone %q, got %q", email, expectedZoneName, zones[0].Name)
	}

	return nil
}
