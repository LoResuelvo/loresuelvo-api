CREATE TABLE coverage_zones (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    normalized_name VARCHAR(100) NOT NULL,
    enabled BOOLEAN NOT NULL,
    created_on TIMESTAMP NOT NULL,
    updated_on TIMESTAMP NOT NULL
);

CREATE UNIQUE INDEX coverage_zones_normalized_name_unique
    ON coverage_zones (normalized_name);

CREATE TABLE provider_coverage_zones (
    provider_id INTEGER NOT NULL REFERENCES providers(user_id) ON DELETE CASCADE,
    coverage_zone_id INTEGER NOT NULL REFERENCES coverage_zones(id) ON DELETE RESTRICT,
    PRIMARY KEY (provider_id, coverage_zone_id)
);

CREATE INDEX provider_coverage_zones_coverage_zone_id_idx
    ON provider_coverage_zones (coverage_zone_id);
