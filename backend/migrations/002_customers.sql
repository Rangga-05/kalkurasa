-- 002_customers.sql: Akun pelanggan (role customer), kontak pengguna, dan relasi orders.customer_id (Tahap 4A+).

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;

ALTER TABLE users
  ADD CONSTRAINT users_role_check
  CHECK (role IN ('owner', 'manager', 'staff', 'admin', 'customer'));

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS contact text NOT NULL DEFAULT '';

ALTER TABLE orders
  ADD COLUMN IF NOT EXISTS customer_id uuid REFERENCES users(id);

CREATE INDEX IF NOT EXISTS orders_customer_id_idx ON orders (customer_id);

