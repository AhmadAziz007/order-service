ALTER TABLE orders
    ALTER COLUMN shipping_type DROP DEFAULT,
    ALTER COLUMN shipping_type TYPE VARCHAR(20)
        USING shipping_type::text,
    ALTER COLUMN shipping_type SET DEFAULT 'PICKUP';

DROP TYPE order_shipping_type;
