DROP INDEX providers_category_id_idx;

ALTER TABLE providers
    DROP COLUMN category_id;
