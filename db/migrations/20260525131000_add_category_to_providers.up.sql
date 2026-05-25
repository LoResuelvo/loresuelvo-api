ALTER TABLE providers
    ADD COLUMN category_id INTEGER REFERENCES categories(id);

CREATE INDEX providers_category_id_idx
    ON providers (category_id);
