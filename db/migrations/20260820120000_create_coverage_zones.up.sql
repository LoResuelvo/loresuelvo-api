CREATE TABLE coverage_markets (
    id SERIAL PRIMARY KEY,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    enabled BOOLEAN NOT NULL,
    created_on TIMESTAMP NOT NULL,
    updated_on TIMESTAMP NOT NULL,
    CONSTRAINT coverage_markets_code_unique UNIQUE (code)
);

CREATE TABLE coverage_zones (
    id SERIAL PRIMARY KEY,
    market_id INTEGER NOT NULL REFERENCES coverage_markets(id) ON DELETE RESTRICT,
    code VARCHAR(100) NOT NULL,
    name VARCHAR(100) NOT NULL,
    normalized_name VARCHAR(100) NOT NULL,
    kind VARCHAR(50) NOT NULL,
    parent_zone_id INTEGER REFERENCES coverage_zones(id) ON DELETE RESTRICT,
    enabled BOOLEAN NOT NULL,
    created_on TIMESTAMP NOT NULL,
    updated_on TIMESTAMP NOT NULL,
    CONSTRAINT coverage_zones_code_unique UNIQUE (code),
    CONSTRAINT coverage_zones_market_normalized_name_unique UNIQUE (market_id, normalized_name),
    CONSTRAINT coverage_zones_kind_check CHECK (
        kind IN ('COMMUNE', 'PARTY', 'DEPARTMENT', 'NEIGHBORHOOD', 'OPERATIONAL_ZONE')
    )
);

CREATE INDEX coverage_zones_parent_zone_id_idx
    ON coverage_zones (parent_zone_id);

CREATE TABLE coverage_zone_external_references (
    coverage_zone_id INTEGER NOT NULL REFERENCES coverage_zones(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    external_id VARCHAR(255) NOT NULL,
    source_version VARCHAR(100),
    created_on TIMESTAMP NOT NULL,
    updated_on TIMESTAMP NOT NULL,
    PRIMARY KEY (coverage_zone_id, provider),
    CONSTRAINT coverage_zone_external_reference_unique UNIQUE (provider, external_id),
    CONSTRAINT coverage_zone_external_provider_not_blank CHECK (BTRIM(provider) <> ''),
    CONSTRAINT coverage_zone_external_id_not_blank CHECK (BTRIM(external_id) <> '')
);

CREATE TABLE provider_coverage_zones (
    provider_id INTEGER NOT NULL REFERENCES providers(user_id) ON DELETE CASCADE,
    coverage_zone_id INTEGER NOT NULL REFERENCES coverage_zones(id) ON DELETE RESTRICT,
    PRIMARY KEY (provider_id, coverage_zone_id)
);

CREATE INDEX provider_coverage_zones_coverage_zone_id_idx
    ON provider_coverage_zones (coverage_zone_id);
