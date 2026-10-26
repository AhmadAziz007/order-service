CREATE TYPE order_shipping_type AS ENUM ('DELIVERY', 'PICKUP');

ALTER TABLE orders
    ALTER COLUMN shipping_type DROP DEFAULT,
    ALTER COLUMN shipping_type TYPE order_shipping_type
        USING UPPER(shipping_type)::order_shipping_type,
    ALTER COLUMN shipping_type SET DEFAULT 'PICKUP';
