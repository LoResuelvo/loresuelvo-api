ALTER TABLE job_requests
    ADD COLUMN service_address_street TEXT,
    ADD COLUMN service_address_street_number TEXT,
    ADD COLUMN service_address_floor TEXT,
    ADD COLUMN service_address_unit TEXT,
    ADD CONSTRAINT job_requests_service_address_check CHECK (
        (
            service_address_street IS NULL
            AND service_address_street_number IS NULL
            AND service_address_floor IS NULL
            AND service_address_unit IS NULL
        )
        OR (
            service_address_street IS NOT NULL
            AND length(btrim(service_address_street)) > 0
            AND service_address_street_number IS NOT NULL
            AND length(btrim(service_address_street_number)) > 0
        )
    );
