CREATE TABLE work_order_reviews (
    work_order_id INTEGER PRIMARY KEY REFERENCES work_orders(id) ON DELETE CASCADE,
    rating SMALLINT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    CONSTRAINT work_order_reviews_rating_check
        CHECK (rating BETWEEN 1 AND 5),
    CONSTRAINT work_order_reviews_description_length_check
        CHECK (char_length(description) <= 500)
);
