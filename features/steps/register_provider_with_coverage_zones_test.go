package steps_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/cucumber/godog"
)

func registerProviderWithCoverageZonesSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que están habilitadas las zonas de cobertura "([^"]*)", "([^"]*)" y "([^"]*)"$`, suite.thereAreEnabledCoverageZones)
	sc.Step(`^que no existe la zona de cobertura "([^"]*)"$`, suite.coverageZoneDoesNotExist)
	sc.Step(`^que la zona de cobertura "([^"]*)" está deshabilitada$`, suite.coverageZoneIsDisabled)
	sc.Step(`^me registro como prestador con correo "([^"]*)", nombre "([^"]*)", apellido "([^"]*)", rubro "([^"]*)" y zona de cobertura "([^"]*)"$`, suite.requestProviderRegistrationWithCoverageZone)
	sc.Step(`^me registro como prestador con correo "([^"]*)", nombre "([^"]*)", apellido "([^"]*)", rubro "([^"]*)" y sin zonas de cobertura$`, suite.requestProviderRegistrationWithoutCoverageZones)
	sc.Step(`^el prestador "([^"]*)" queda registrado con la zona de cobertura "([^"]*)"$`, suite.providerIsRegisteredWithCoverageZone)
	sc.Step(`^el sistema me indica que debo seleccionar al menos una zona de cobertura$`, suite.systemReportsCoverageZoneRequired)
	sc.Step(`^el sistema me indica que la zona de cobertura seleccionada no está disponible$`, suite.systemReportsCoverageZoneUnavailable)
	sc.Step(`^el prestador "([^"]*)" no queda registrado$`, suite.providerIsNotRegistered)
}

const defaultProviderCoverageZoneName = "Comuna 6"

const nonExistingProviderCoverageZoneID = 999999999

func (suite *testSuite) ensureDefaultProviderCoverageZone() (int, error) {
	zone, err := suite.coverageZoneRepository.FindByName(context.Background(), defaultProviderCoverageZoneName)
	if err == nil {
		return zone.ID, nil
	}
	if !errors.Is(err, coveragezone.ErrDoesNotExist) {
		return 0, fmt.Errorf("could not find default provider coverage zone: %w", err)
	}

	newZone, err := coveragezone.New(defaultProviderCoverageZoneName)
	if err != nil {
		return 0, err
	}

	savedZone, err := suite.coverageZoneRepository.Save(context.Background(), *newZone)
	if err != nil {
		return 0, fmt.Errorf("could not create default provider coverage zone: %w", err)
	}

	return savedZone.ID, nil
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

func (suite *testSuite) coverageZoneDoesNotExist(name string) error {
	_, err := suite.coverageZoneRepository.FindByName(context.Background(), name)
	if errors.Is(err, coveragezone.ErrDoesNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not verify that coverage zone %q does not exist: %w", name, err)
	}

	return fmt.Errorf("coverage zone %q unexpectedly exists", name)
}

func (suite *testSuite) coverageZoneIsDisabled(name string) error {
	ctx := context.Background()
	zone, err := suite.coverageZoneRepository.FindByName(ctx, name)
	if err != nil {
		return fmt.Errorf("could not find coverage zone %q to disable: %w", name, err)
	}
	if zone == nil {
		return fmt.Errorf("coverage zone repository returned no zone for %q", name)
	}
	if zone.Enabled {
		zone.Disable()
		if err := suite.coverageZoneRepository.Update(ctx, *zone); err != nil {
			return fmt.Errorf("could not disable coverage zone %q: %w", name, err)
		}
	}

	disabledZone, err := suite.coverageZoneRepository.FindByID(ctx, zone.ID)
	if err != nil {
		return fmt.Errorf("could not verify that coverage zone %q is disabled: %w", name, err)
	}
	if disabledZone == nil || disabledZone.Enabled {
		return fmt.Errorf("coverage zone %q is still enabled", name)
	}

	return nil
}

func (suite *testSuite) requestProviderRegistrationWithCoverageZone(email, name, surname, categoryName, coverageZoneName string) error {
	categoryID, err := suite.categoryIDFor(categoryName)
	if err != nil {
		return err
	}

	coverageZoneID := nonExistingProviderCoverageZoneID
	zone, err := suite.coverageZoneRepository.FindByName(context.Background(), coverageZoneName)
	if err == nil {
		if zone == nil {
			return fmt.Errorf("coverage zone repository returned no zone for %q", coverageZoneName)
		}
		coverageZoneID = zone.ID
		if !zone.Enabled {
			suite.expectedCoverageZoneRegistrationError = coveragezone.ErrNotAvailable.Error()
		} else {
			suite.expectedCoverageZoneRegistrationError = ""
		}
	} else if !errors.Is(err, coveragezone.ErrDoesNotExist) {
		return fmt.Errorf("could not find coverage zone %q for the scenario: %w", coverageZoneName, err)
	} else {
		suite.expectedCoverageZoneRegistrationError = coveragezone.ErrDoesNotExist.Error()
	}

	return suite.requestProviderRegistration(providerRegistrationRequest{
		Email:                  email,
		Name:                   name,
		Surname:                surname,
		CategoryID:             categoryID,
		CoverageZoneIDs:        []int{coverageZoneID},
		CriminalRecordFile:     "criminal-record.pdf",
		CUITCertificateFile:    "cuit-certificate.pdf",
		BiometricValidationID:  "biometric-validation-approved",
		ProfessionalCredential: "professional-license-or-certificate.pdf",
		ProfilePhotoFileID:     suite.providerProfilePhotoFileID,
	})
}

func (suite *testSuite) requestProviderRegistrationWithoutCoverageZones(email, name, surname, categoryName string) error {
	suite.expectedCoverageZoneRegistrationError = ""

	categoryID, err := suite.categoryIDFor(categoryName)
	if err != nil {
		return err
	}

	return suite.requestProviderRegistration(providerRegistrationRequest{
		Email:                  email,
		Name:                   name,
		Surname:                surname,
		CategoryID:             categoryID,
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

func (suite *testSuite) systemReportsCoverageZoneRequired() error {
	if err := suite.registrationResponseShouldHaveStatusCode(http.StatusBadRequest); err != nil {
		return err
	}

	return suite.registrationResponseShouldSay(coveragezone.ErrAtLeastOneRequired.Error())
}

func (suite *testSuite) systemReportsCoverageZoneUnavailable() error {
	if err := suite.registrationResponseShouldHaveStatusCode(http.StatusBadRequest); err != nil {
		return err
	}
	if suite.expectedCoverageZoneRegistrationError == "" {
		return fmt.Errorf("expected a coverage-zone availability error for the registration scenario")
	}

	return suite.registrationResponseShouldSay(suite.expectedCoverageZoneRegistrationError)
}

func (suite *testSuite) providerIsNotRegistered(email string) error {
	if suite.userRepository.FindByEmail(email) {
		return fmt.Errorf("expected provider %q not to be registered", email)
	}

	return nil
}
