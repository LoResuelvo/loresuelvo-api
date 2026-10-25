ALTER TABLE job_requests
    DROP CONSTRAINT job_requests_service_address_check,
    DROP COLUMN service_address_street,
    DROP COLUMN service_address_street_number,
    DROP COLUMN service_address_floor,
    DROP COLUMN service_address_unit;
