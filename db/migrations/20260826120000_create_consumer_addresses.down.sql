DROP TABLE consumer_addresses;

ALTER TABLE consumers
    DROP CONSTRAINT consumers_user_id_unique;
