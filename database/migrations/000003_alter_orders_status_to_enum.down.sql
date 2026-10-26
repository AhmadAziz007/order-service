-- Kembalikan ke varchar
ALTER TABLE orders ALTER COLUMN status DROP DEFAULT;

ALTER TABLE orders
    ALTER COLUMN status TYPE VARCHAR(20)
    USING status::VARCHAR(20);

ALTER TABLE orders ALTER COLUMN status SET NOT NULL;

DROP TYPE order_status;