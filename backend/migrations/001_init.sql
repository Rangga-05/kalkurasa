-- Kalkurasa 001_init.sql
-- Skema awal database

create extension if not exists pgcrypto;

create table users (
  id uuid primary key default gen_random_uuid(),
  email text unique not null,
  name text not null,
  role text not null check (role in ('owner','manager','staff','admin')),
  password_hash text not null,
  is_active boolean not null default true,
  created_at timestamptz not null default now()
);

create table resources (
  id uuid primary key default gen_random_uuid(),
  code text unique not null,
  name text not null,
  unit text not null,
  count int not null check (count > 0),
  amount_per_unit_day numeric(12,4) not null check (amount_per_unit_day > 0),
  efficiency numeric(5,4) not null check (efficiency > 0 and efficiency <= 1)
);

create table products (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  category text not null default 'umum',
  price numeric(16,4) not null check (price >= 0),
  packaging_cost numeric(16,4) not null default 0,
  overhead_cost numeric(16,4) not null default 0,
  is_active boolean not null default true
);

create table product_loads (
  product_id uuid references products(id) on delete cascade,
  resource_id uuid references resources(id) on delete cascade,
  load numeric(12,4) not null check (load >= 0),
  primary key (product_id, resource_id)
);

create table ingredients (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  use_unit text not null,
  buy_unit text not null,
  buy_unit_size numeric(12,4) not null check (buy_unit_size > 0),
  buy_price numeric(16,4) not null check (buy_price >= 0),
  stock numeric(16,4) not null default 0,
  min_stock numeric(16,4) not null default 0,
  rounding text not null default 'loose' check (rounding in ('loose','pack')),
  is_active boolean not null default true
);

create table recipe_items (
  product_id uuid references products(id) on delete cascade,
  ingredient_id uuid references ingredients(id),
  qty_per_unit numeric(16,6) not null check (qty_per_unit > 0),
  primary key (product_id, ingredient_id)
);

create table settings (
  key text primary key,
  value jsonb not null
);

create table orders (
  id uuid primary key default gen_random_uuid(),
  code text unique not null,
  source text not null default 'public' check (source in ('public','manual')),
  customer_name text not null,
  customer_contact text not null,
  delivery_type text not null check (delivery_type in ('delivery','pickup')),
  address text,
  notes text,
  event_date date not null,
  production_date date not null,
  status text not null check (status in ('submitted','pending_decision','countered','accepted','confirmed',
    'in_production','ready','delivering','completed','rejected','cancelled','expired')),
  total numeric(16,4) not null,
  dp_required numeric(16,4) not null default 0,
  dp_deadline timestamptz,
  paid numeric(16,4) not null default 0,
  recommendation text,
  recommendation_reason text,
  counter_offer jsonb,
  decided_by uuid references users(id),
  decided_at timestamptz,
  override_reason text,
  snapshot jsonb,
  created_by uuid references users(id),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index on orders (production_date, status);
create index on orders (status);

create table order_items (
  id uuid primary key default gen_random_uuid(),
  order_id uuid not null references orders(id) on delete cascade,
  product_id uuid not null references products(id),
  qty int not null check (qty > 0),
  unit_price numeric(16,4) not null,
  unit_cost numeric(16,4) not null
);

create table order_evaluations (
  id uuid primary key default gen_random_uuid(),
  order_id uuid not null references orders(id) on delete cascade,
  gate text not null check (gate in ('G1','G2','G3','G4')),
  value numeric(18,6),
  threshold numeric(18,6),
  passed boolean not null,
  detail jsonb,
  created_at timestamptz not null default now()
);

create table payments (
  id uuid primary key default gen_random_uuid(),
  order_id uuid not null references orders(id) on delete cascade,
  amount numeric(16,4) not null check (amount > 0),
  method text not null default 'transfer',
  paid_at timestamptz not null default now(),
  note text,
  recorded_by uuid references users(id)
);

create table tasks (
  id uuid primary key default gen_random_uuid(),
  order_id uuid references orders(id) on delete cascade,
  type text not null check (type in ('produce','shop','deliver')),
  title text not null,
  due_date date not null,
  assignee_id uuid references users(id),
  status text not null default 'todo' check (status in ('todo','doing','done')),
  actual_minutes int,
  created_at timestamptz not null default now()
);

create table purchases (
  id uuid primary key default gen_random_uuid(),
  ingredient_id uuid not null references ingredients(id),
  qty_buy_units numeric(16,4) not null check (qty_buy_units > 0),
  unit_price numeric(16,4) not null,
  bought_at timestamptz not null default now(),
  bought_by uuid references users(id)
);

create table stock_movements (
  id uuid primary key default gen_random_uuid(),
  ingredient_id uuid not null references ingredients(id),
  delta numeric(16,4) not null,
  reason text not null,
  ref_type text, ref_id uuid,
  created_at timestamptz not null default now()
);

create table cash_entries (
  id uuid primary key default gen_random_uuid(),
  entry_date date not null,
  amount numeric(16,4) not null,
  note text,
  created_by uuid references users(id)
);

create table audit_logs (
  id uuid primary key default gen_random_uuid(),
  actor_id uuid references users(id),
  action text not null,
  entity text not null, entity_id uuid,
  detail jsonb,
  created_at timestamptz not null default now()
);

create table day_locks (day date primary key);

create table rate_limits (
  key text not null,
  window_start timestamptz not null,
  count int not null default 1,
  primary key (key, window_start)
);

-- Tabel pencatatan migrasi
create table if not exists schema_migrations (
  version text primary key,
  applied_at timestamptz not null default now()
);

-- RLS aktif tanpa policy pada semua tabel
do $$
declare t text;
begin
  for t in select tablename from pg_tables where schemaname = 'public' loop
    execute format('alter table public.%I enable row level security', t);
  end loop;
end $$;

