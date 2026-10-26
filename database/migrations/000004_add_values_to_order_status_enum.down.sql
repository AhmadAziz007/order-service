-- Rollback enum value tidak didukung langsung oleh Postgres.
-- Kalau perlu rollback penuh, perlu rebuild type dari awal:

ALTER TABLE orders ALTER COLUMN status DROP DEFAULT;
ALTER TABLE orders ALTER COLUMN status TYPE VARCHAR(20) USING status::text;
DROP TYPE order_status;

CREATE TYPE order_status AS ENUM ('PENDING', 'DONE', 'CONFIRMED', 'CANCELLED', 'PROCESS', 'SENDING');

ALTER TABLE orders ALTER COLUMN status TYPE order_status USING status::order_status;
ALTER TABLE orders ALTER COLUMN status SET DEFAULT 'PENDING';
ALTER TABLE orders ALTER COLUMN status SET NOT NULL;