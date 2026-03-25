-- Schema + sample data for ask-postgres demo.
-- Runs automatically on first container init (fresh volume).

\set ON_ERROR_STOP on

create schema if not exists app;

create table if not exists app.customers (
  customer_id bigserial primary key,
  email text not null unique,
  full_name text not null,
  created_at timestamptz not null default now()
);

create table if not exists app.products (
  product_id bigserial primary key,
  sku text not null unique,
  name text not null,
  category text not null,
  price_cents int not null check (price_cents > 0),
  created_at timestamptz not null default now()
);

create table if not exists app.orders (
  order_id bigserial primary key,
  customer_id bigint not null references app.customers(customer_id),
  status text not null check (status in ('created','paid','shipped','cancelled','refunded')),
  created_at timestamptz not null default now(),
  paid_at timestamptz,
  shipped_at timestamptz
);

create table if not exists app.order_items (
  order_item_id bigserial primary key,
  order_id bigint not null references app.orders(order_id) on delete cascade,
  product_id bigint not null references app.products(product_id),
  quantity int not null check (quantity > 0),
  unit_price_cents int not null check (unit_price_cents > 0)
);

create table if not exists app.events (
  event_id bigserial primary key,
  customer_id bigint references app.customers(customer_id),
  event_type text not null,
  occurred_at timestamptz not null default now(),
  props jsonb not null default '{}'::jsonb
);

create index if not exists idx_orders_customer_created on app.orders(customer_id, created_at desc);
create index if not exists idx_events_type_time on app.events(event_type, occurred_at desc);
create index if not exists idx_events_props_gin on app.events using gin (props);

-- Seed data
insert into app.customers (email, full_name, created_at) values
  ('ava@example.com', 'Ava Singh', now() - interval '40 days'),
  ('liam@example.com', 'Liam Chen', now() - interval '25 days'),
  ('noah@example.com', 'Noah Patel', now() - interval '20 days'),
  ('mia@example.com', 'Mia Garcia', now() - interval '10 days'),
  ('zoe@example.com', 'Zoe Kim', now() - interval '5 days')
on conflict (email) do nothing;

insert into app.products (sku, name, category, price_cents, created_at) values
  ('SKU-CHAIR-01', 'Ergo Chair', 'furniture', 19999, now() - interval '60 days'),
  ('SKU-DESK-01', 'Standing Desk', 'furniture', 49999, now() - interval '55 days'),
  ('SKU-MUG-01', 'Ceramic Mug', 'kitchen', 1599, now() - interval '30 days'),
  ('SKU-LAMP-01', 'Desk Lamp', 'lighting', 3499, now() - interval '28 days'),
  ('SKU-CABLE-01', 'USB-C Cable', 'electronics', 1299, now() - interval '15 days')
on conflict (sku) do nothing;

-- A few orders across time/statuses
insert into app.orders (customer_id, status, created_at, paid_at, shipped_at)
select c.customer_id,
       o.status,
       o.created_at,
       o.paid_at,
       o.shipped_at
from (
  values
    ('ava@example.com',  'paid',      now() - interval '12 days', now() - interval '12 days' + interval '20 minutes', null),
    ('ava@example.com',  'shipped',   now() - interval '9 days',  now() - interval '9 days'  + interval '10 minutes', now() - interval '8 days'),
    ('liam@example.com', 'cancelled', now() - interval '7 days',  null, null),
    ('noah@example.com', 'paid',      now() - interval '6 days',  now() - interval '6 days'  + interval '5 minutes', null),
    ('mia@example.com',  'shipped',   now() - interval '3 days',  now() - interval '3 days'  + interval '2 minutes', now() - interval '2 days'),
    ('zoe@example.com',  'created',   now() - interval '1 day',   null, null)
) as o(email, status, created_at, paid_at, shipped_at)
join app.customers c on c.email = o.email
on conflict do nothing;

-- Order items (attach items to latest orders)
with latest_orders as (
  select order_id, customer_id, created_at
  from app.orders
  order by order_id desc
  limit 6
),
prod as (
  select product_id, sku, price_cents from app.products
)
insert into app.order_items (order_id, product_id, quantity, unit_price_cents)
select lo.order_id,
       p.product_id,
       (case when p.sku = 'SKU-CABLE-01' then 2 else 1 end) as quantity,
       p.price_cents
from latest_orders lo
join prod p on p.sku in ('SKU-CHAIR-01','SKU-MUG-01','SKU-LAMP-01','SKU-CABLE-01')
where (lo.order_id % 2 = 0 and p.sku in ('SKU-MUG-01','SKU-CABLE-01'))
   or (lo.order_id % 2 = 1 and p.sku in ('SKU-CHAIR-01','SKU-LAMP-01'))
on conflict do nothing;

-- Events
insert into app.events (customer_id, event_type, occurred_at, props)
select c.customer_id, e.event_type, e.occurred_at, e.props
from (
  values
    ('ava@example.com',  'page_view',   now() - interval '2 days',  '{"path":"/products/SKU-CHAIR-01","ref":"ad"}'::jsonb),
    ('ava@example.com',  'add_to_cart', now() - interval '2 days' + interval '2 minutes', '{"sku":"SKU-CHAIR-01","qty":1}'::jsonb),
    ('liam@example.com', 'page_view',   now() - interval '1 day',   '{"path":"/products/SKU-DESK-01"}'::jsonb),
    ('noah@example.com', 'search',      now() - interval '6 days',  '{"q":"desk lamp"}'::jsonb),
    ('mia@example.com',  'purchase',    now() - interval '3 days',  '{"order_hint":"recent"}'::jsonb),
    ('zoe@example.com',  'page_view',   now() - interval '5 hours', '{"path":"/"}'::jsonb)
) as e(email, event_type, occurred_at, props)
join app.customers c on c.email = e.email
on conflict do nothing;

