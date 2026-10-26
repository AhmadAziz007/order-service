-- Buat enum type dulu
CREATE TYPE order_status AS ENUM ('PENDING', 'DONE');

-- Hapus default lama dan CHECK constraint (kalau ada) sebelum ubah tipe
ALTER TABLE orders ALTER COLUMN status DROP DEFAULT;

-- Ubah tipe kolom dari varchar ke enum
ALTER TABLE orders
    ALTER COLUMN status TYPE order_status
    USING status::order_status;

-- Set ulang default dengan tipe enum
ALTER TABLE orders ALTER COLUMN status SET DEFAULT 'PENDING';