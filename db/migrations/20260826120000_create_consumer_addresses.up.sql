ALTER TABLE consumers
    ADD CONSTRAINT consumers_user_id_unique UNIQUE (user_id);

CREATE TABLE consumer_addresses (
    consumer_id INTEGER PRIMARY KEY REFERENCES consumers(user_id) ON DELETE CASCADE,
    street VARCHAR(200) NOT NULL,
    street_number VARCHAR(50) NOT NULL,
    floor VARCHAR(20),
    unit VARCHAR(50),
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    coverage_zone_id INTEGER NOT NULL REFERENCES coverage_zones(id) ON DELETE RESTRICT,
    created_on TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_on TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT consumer_addresses_street_not_blank CHECK (BTRIM(street) <> ''),
    CONSTRAINT consumer_addresses_street_number_not_blank CHECK (BTRIM(street_number) <> ''),
    CONSTRAINT consumer_addresses_latitude_range CHECK (latitude >= -90 AND latitude <= 90),
    CONSTRAINT consumer_addresses_longitude_range CHECK (longitude >= -180 AND longitude <= 180)
);

CREATE INDEX consumer_addresses_coverage_zone_id_idx
    ON consumer_addresses (coverage_zone_id);

