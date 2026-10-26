ALTER TABLE orders
    ADD COLUMN stock_deducted BOOLEAN NOT NULL DEFAULT FALSE;

-- Orders that already existed were sent to the stock queue at creation time.
UPDATE orders
SET stock_deducted = TRUE;
