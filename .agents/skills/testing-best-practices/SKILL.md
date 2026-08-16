---
name: testing-best-practices
description: Use when writing or running tests in this project. Covers BDD (godog), unit tests, and integration tests with PostgreSQL. Ensures consistency with existing test patterns and Makefile conventions.
---

# Testing Best Practices

Project has three test layers: BDD acceptance (godog), unit (domain), integration (repository with real DB).

## Running Tests

```bash
make test              # Run all tests with migrations; serializes packages with -p 1
make test-all-once    # Tests + lint
```

- Prefer `make test` over local `go test ./...` for integration/BDD tests because it runs inside Docker with the correct `TEST_DATABASE_URL`.
- Tests share a PostgreSQL test database; package-level parallelism is intentionally disabled with `go test -p 1`.

## BDD (godog)

- Feature files in `features/consumer/` and `features/provider/`.
- Step definitions in `features/steps/`.
- Test database via `TEST_DATABASE_URL` env var (separate from dev/prod).
- Auth0 replaced with test helper (`testhelper/fakevalidator.go`) during tests.
- Use `@wip` tag for scenarios not yet ready to run.
- Never write SQL in `features/steps`; steps must use repositories or HTTP calls.
- Keep prerequisites explicit in Gherkin; do not hide unrelated setup inside shared steps.
- Spanish is acceptable in scenario text and step regex strings only; Go code remains English.
- If Gherkin uses user-facing identifiers such as emails or names, resolve them in steps through repositories/helpers and assert domain/API IDs or roles; do not force domain entities or API responses to expose emails only for test convenience.
- Reuse existing setup/assertion steps where possible, but keep each step honest: a `When` should exercise the API path under test and a `Then` should validate the observable contract.

```gherkin
Scenario: ...
  Given ...
  When ...
  Then ...
```

## Unit Tests (domain)

- Files: `internal/domain/*/service_test.go`, `*_test.go`.
- Domain has NO external dependencies — easy to test.
- Mock the smallest domain interface/capability the service consumes, not a full repository when unnecessary.
- Name test doubles after the port (`consumerIDFinderMock`, `categoryFinderMock`) so tests document decoupling.

```go
func TestCategoryService_Create(t *testing.T) {
    // arrange
    svc := NewService(mockRepo{})
    // act
    result, err := svc.Create(ctx, "Plomeria")
    // assert
    require.NoError(t, err)
}
```

## Integration Tests (repository)

- Files: `internal/adapters/repositories/*_test.go`.
- Uses real PostgreSQL via `TEST_DATABASE_URL`.
- Migration up/down per test suite.
- Repository tests may use SQL for fixture cleanup; BDD steps must not.
- Add integration coverage for each new repository method, especially helpers exposed for BDD setup/assertions like `FindIDByEmail`, `ExistsByID`, `DeleteAll`, `FindBetween`.
- When a repository returns an aggregate entity, assert that owned nested data is hydrated consistently (for example `Conversation.Messages` after `SaveWithMessage` and `FindByID`).

```bash
make migrate-test-up      # Setup test DB
make migrate-test-down    # Teardown test DB
```

## Mocking

- Use `testify/require` for assertions.
- Use `DATA-DOG/go-sqlmock` for SQL mocking where appropriate.
- Auth test helper in `testhelper/fakevalidator.go` — do not mock real Auth0.
- Keep every mock/double shared by a package's unit tests in that package's
  `mock_test.go`; individual test files should contain scenarios and
  assertions, not private mock implementations. Move reusable setup helpers
  there as well when they construct those mocks.
- Name mocks after the smallest port they implement (for example,
  `fileServiceMock` or `unitOfWorkMock`) and embed `testify/mock.Mock`.
  Implement port methods with `m.Called(...)`; configure behavior in each test
  with `On(...).Return(...).Once()` and verify it with
  `AssertExpectations`/`AssertNotCalled`.
- Use `mock.Anything` for request contexts and `mock.MatchedBy` for meaningful
  domain predicates. Use `Run` callbacks when a persistence mock must mutate a
  passed aggregate (for example, assigning a generated ID), rather than
  adding bespoke mutable fields and handwritten behavior to the mock.
- When an interface dependency is intentionally absent, pass a nil interface
  to the constructor; avoid passing a typed nil mock because it is non-nil
  after interface conversion.

## Coverage

- Aim for meaningful coverage, not vanity metrics.
- Happy path + error paths for each domain service method.
