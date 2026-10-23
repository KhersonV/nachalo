package repository

import "fmt"

// EnsureMarketSchema is safe to rerun at startup. Escrow and delivery records are
// durable so a process restart cannot lose an item or a reserved balance.
func EnsureMarketSchema() error {
	if DB == nil {
		return fmt.Errorf("market schema: database is not initialized")
	}
	_, err := DB.Exec(`
 ALTER TABLE resources ADD COLUMN IF NOT EXISTS tradable BOOLEAN NOT NULL DEFAULT true;
 ALTER TABLE resources ADD COLUMN IF NOT EXISTS market_category TEXT;
 ALTER TABLE resources ADD COLUMN IF NOT EXISTS market_subcategory TEXT;
 ALTER TABLE item_templates DROP CONSTRAINT IF EXISTS item_templates_rarity_check;
 ALTER TABLE item_templates ADD CONSTRAINT item_templates_rarity_check CHECK (rarity IN ('gray','green','blue','purple','orange'));
 ALTER TABLE item_sets DROP CONSTRAINT IF EXISTS item_sets_rarity_check;
 ALTER TABLE item_sets ADD CONSTRAINT item_sets_rarity_check CHECK (rarity IN ('gray','green','blue','purple','orange'));
 CREATE TABLE IF NOT EXISTS market_auctions (
 id BIGSERIAL PRIMARY KEY,
 seller_user_id INTEGER NOT NULL REFERENCES player_profiles(user_id),
 item_kind TEXT NOT NULL CHECK (item_kind IN ('equipment','resource')),
 item_instance_id UUID REFERENCES item_instances(id),
 resource_id INTEGER REFERENCES resources(id),
 resource_data JSONB,
 quantity INTEGER NOT NULL CHECK (quantity > 0),
 start_price INTEGER NOT NULL CHECK (start_price > 0),
 buyout_price INTEGER CHECK (buyout_price IS NULL OR buyout_price >= start_price),
 highest_bid INTEGER,
 highest_bidder_user_id INTEGER REFERENCES player_profiles(user_id),
 bid_count INTEGER NOT NULL DEFAULT 0 CHECK (bid_count >= 0),
 status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','sold','expired','cancelled')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 expires_at TIMESTAMPTZ NOT NULL,
 settled_at TIMESTAMPTZ,
 CHECK ((item_kind='equipment' AND item_instance_id IS NOT NULL AND resource_id IS NULL AND quantity=1) OR (item_kind='resource' AND resource_id IS NOT NULL AND item_instance_id IS NULL AND resource_data IS NOT NULL)),
 CHECK ((highest_bid IS NULL AND highest_bidder_user_id IS NULL) OR (highest_bid IS NOT NULL AND highest_bidder_user_id IS NOT NULL))
 );
 CREATE UNIQUE INDEX IF NOT EXISTS market_active_equipment_auction ON market_auctions(item_instance_id) WHERE status='active';
 CREATE INDEX IF NOT EXISTS market_auctions_active_expiry ON market_auctions(expires_at,id) WHERE status='active';
 CREATE INDEX IF NOT EXISTS market_auctions_seller ON market_auctions(seller_user_id,id DESC);
 CREATE TABLE IF NOT EXISTS market_bids (
 id BIGSERIAL PRIMARY KEY,
 auction_id BIGINT NOT NULL REFERENCES market_auctions(id),
 bidder_user_id INTEGER NOT NULL REFERENCES player_profiles(user_id),
 amount INTEGER NOT NULL CHECK (amount > 0),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
 );
 CREATE INDEX IF NOT EXISTS market_bids_user ON market_bids(bidder_user_id,id DESC);
 CREATE TABLE IF NOT EXISTS market_orders (
 id BIGSERIAL PRIMARY KEY,
 buyer_user_id INTEGER NOT NULL REFERENCES player_profiles(user_id),
 resource_id INTEGER NOT NULL REFERENCES resources(id),
 quantity INTEGER NOT NULL CHECK (quantity > 0),
 filled_quantity INTEGER NOT NULL DEFAULT 0 CHECK (filled_quantity >= 0 AND filled_quantity <= quantity),
 price_per_unit INTEGER NOT NULL CHECK (price_per_unit > 0),
 reserved_remaining BIGINT NOT NULL CHECK (reserved_remaining >= 0),
 status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','partially_filled','filled','expired','cancelled')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 expires_at TIMESTAMPTZ NOT NULL,
 settled_at TIMESTAMPTZ,
 CHECK (reserved_remaining = (quantity-filled_quantity)::bigint * price_per_unit OR status IN ('expired','cancelled'))
 );
 CREATE INDEX IF NOT EXISTS market_orders_active_expiry ON market_orders(expires_at,id) WHERE status IN ('open','partially_filled');
 CREATE INDEX IF NOT EXISTS market_orders_buyer ON market_orders(buyer_user_id,id DESC);
 CREATE TABLE IF NOT EXISTS market_fills (
 id BIGSERIAL PRIMARY KEY,
 order_id BIGINT NOT NULL REFERENCES market_orders(id),
 seller_user_id INTEGER NOT NULL REFERENCES player_profiles(user_id),
 quantity INTEGER NOT NULL CHECK (quantity > 0),
 gross_price BIGINT NOT NULL CHECK (gross_price > 0),
 fee BIGINT NOT NULL CHECK (fee >= 0),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
 );
 CREATE TABLE IF NOT EXISTS market_deliveries (
 id BIGSERIAL PRIMARY KEY,
 recipient_user_id INTEGER NOT NULL REFERENCES player_profiles(user_id),
 item_kind TEXT NOT NULL CHECK (item_kind IN ('equipment','resource')),
 item_instance_id UUID REFERENCES item_instances(id),
 resource_id INTEGER REFERENCES resources(id),
 resource_data JSONB,
 quantity INTEGER NOT NULL CHECK (quantity > 0),
 claimed_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK ((item_kind='equipment' AND item_instance_id IS NOT NULL AND resource_id IS NULL AND quantity=1) OR (item_kind='resource' AND resource_id IS NOT NULL AND item_instance_id IS NULL AND resource_data IS NOT NULL))
 );
 CREATE UNIQUE INDEX IF NOT EXISTS market_unclaimed_equipment_delivery_once ON market_deliveries(item_instance_id) WHERE claimed_at IS NULL;
 CREATE INDEX IF NOT EXISTS market_delivery_inbox ON market_deliveries(recipient_user_id,id DESC) WHERE claimed_at IS NULL;
 CREATE TABLE IF NOT EXISTS market_wallet_claims (
 id BIGSERIAL PRIMARY KEY,
 recipient_user_id INTEGER NOT NULL REFERENCES player_profiles(user_id),
 amount BIGINT NOT NULL CHECK (amount > 0),
 claimed_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
 );
 CREATE INDEX IF NOT EXISTS market_wallet_claim_inbox ON market_wallet_claims(recipient_user_id,id DESC) WHERE claimed_at IS NULL;
 CREATE TABLE IF NOT EXISTS market_transactions (
 id BIGSERIAL PRIMARY KEY,
 kind TEXT NOT NULL CHECK (kind IN ('auction_sale','exchange_fill')),
 auction_id BIGINT REFERENCES market_auctions(id),
 fill_id BIGINT REFERENCES market_fills(id),
 buyer_user_id INTEGER NOT NULL REFERENCES player_profiles(user_id),
 seller_user_id INTEGER NOT NULL REFERENCES player_profiles(user_id),
 gross_price BIGINT NOT NULL CHECK (gross_price > 0),
 fee BIGINT NOT NULL CHECK (fee >= 0),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK ((kind='auction_sale' AND auction_id IS NOT NULL AND fill_id IS NULL) OR (kind='exchange_fill' AND fill_id IS NOT NULL AND auction_id IS NULL))
 );
 CREATE UNIQUE INDEX IF NOT EXISTS market_transaction_auction_once ON market_transactions(auction_id) WHERE auction_id IS NOT NULL;
 CREATE UNIQUE INDEX IF NOT EXISTS market_transaction_fill_once ON market_transactions(fill_id) WHERE fill_id IS NOT NULL;
 CREATE TABLE IF NOT EXISTS market_stack_catalog (
 inventory_key TEXT PRIMARY KEY,
 catalog_id INTEGER NOT NULL DEFAULT 0,
 code TEXT NOT NULL,
 name TEXT NOT NULL,
 category TEXT NOT NULL,
 subcategory TEXT NOT NULL,
 rarity TEXT NOT NULL DEFAULT 'gray',
 image_url TEXT NOT NULL DEFAULT '',
 tradable BOOLEAN NOT NULL DEFAULT true,
 stackable BOOLEAN NOT NULL DEFAULT true
 );
 ALTER TABLE market_stack_catalog ADD COLUMN IF NOT EXISTS catalog_id INTEGER NOT NULL DEFAULT 0;
 ALTER TABLE market_stack_catalog ADD COLUMN IF NOT EXISTS required_forge_level INTEGER NOT NULL DEFAULT 0;
 ALTER TABLE market_auctions ADD COLUMN IF NOT EXISTS stack_key TEXT REFERENCES market_stack_catalog(inventory_key);
 ALTER TABLE market_deliveries ADD COLUMN IF NOT EXISTS stack_key TEXT REFERENCES market_stack_catalog(inventory_key);
 ALTER TABLE market_orders ADD COLUMN IF NOT EXISTS item_kind TEXT NOT NULL DEFAULT 'resource';
 ALTER TABLE market_orders ADD COLUMN IF NOT EXISTS template_id BIGINT REFERENCES item_templates(id);
 ALTER TABLE market_orders ADD COLUMN IF NOT EXISTS stack_key TEXT REFERENCES market_stack_catalog(inventory_key);
 ALTER TABLE market_orders ALTER COLUMN resource_id DROP NOT NULL;
 DO $$ DECLARE c RECORD; BEGIN
   FOR c IN SELECT conname FROM pg_constraint WHERE conrelid='market_auctions'::regclass AND contype='c' AND conname NOT IN ('market_auctions_shape_v2','market_auctions_kind_v2') AND pg_get_constraintdef(oid) LIKE '%item_kind%' LOOP
     EXECUTE format('ALTER TABLE market_auctions DROP CONSTRAINT %I',c.conname);
   END LOOP;
   FOR c IN SELECT conname FROM pg_constraint WHERE conrelid='market_deliveries'::regclass AND contype='c' AND conname NOT IN ('market_deliveries_shape_v2','market_deliveries_kind_v2') AND pg_get_constraintdef(oid) LIKE '%item_kind%' LOOP
     EXECUTE format('ALTER TABLE market_deliveries DROP CONSTRAINT %I',c.conname);
   END LOOP;
 END $$;
 DO $$ BEGIN
   IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='market_auctions_kind_v2') THEN
     ALTER TABLE market_auctions ADD CONSTRAINT market_auctions_kind_v2 CHECK(item_kind IN ('equipment','resource','stack'));
   END IF;
   IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='market_auctions_shape_v2') THEN
     ALTER TABLE market_auctions ADD CONSTRAINT market_auctions_shape_v2 CHECK(
       (item_kind='equipment' AND item_instance_id IS NOT NULL AND resource_id IS NULL AND stack_key IS NULL AND quantity=1)
       OR (item_kind='resource' AND resource_id IS NOT NULL AND item_instance_id IS NULL AND stack_key IS NULL AND resource_data IS NOT NULL)
       OR (item_kind='stack' AND stack_key IS NOT NULL AND item_instance_id IS NULL AND resource_id IS NULL AND resource_data IS NOT NULL));
   END IF;
   IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='market_deliveries_kind_v2') THEN
     ALTER TABLE market_deliveries ADD CONSTRAINT market_deliveries_kind_v2 CHECK(item_kind IN ('equipment','resource','stack'));
   END IF;
   IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='market_deliveries_shape_v2') THEN
     ALTER TABLE market_deliveries ADD CONSTRAINT market_deliveries_shape_v2 CHECK(
       (item_kind='equipment' AND item_instance_id IS NOT NULL AND resource_id IS NULL AND stack_key IS NULL AND quantity=1)
       OR (item_kind='resource' AND resource_id IS NOT NULL AND item_instance_id IS NULL AND stack_key IS NULL AND resource_data IS NOT NULL)
       OR (item_kind='stack' AND stack_key IS NOT NULL AND item_instance_id IS NULL AND resource_id IS NULL AND resource_data IS NOT NULL));
   END IF;
   IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='market_orders_shape_v2') THEN
     ALTER TABLE market_orders ADD CONSTRAINT market_orders_shape_v2 CHECK(
       (item_kind='resource' AND resource_id IS NOT NULL AND template_id IS NULL AND stack_key IS NULL)
       OR (item_kind='equipment' AND template_id IS NOT NULL AND resource_id IS NULL AND stack_key IS NULL AND quantity=1)
       OR (item_kind='stack' AND stack_key IS NOT NULL AND resource_id IS NULL AND template_id IS NULL));
   END IF;
 END $$;
 CREATE INDEX IF NOT EXISTS market_orders_template ON market_orders(template_id) WHERE item_kind='equipment';
 CREATE INDEX IF NOT EXISTS market_orders_stack ON market_orders(stack_key) WHERE item_kind='stack';
 ALTER TABLE market_transactions ADD COLUMN IF NOT EXISTS variant_durability INTEGER;
 ALTER TABLE market_transactions ADD COLUMN IF NOT EXISTS variant_max_durability INTEGER;
 ALTER TABLE market_transactions ADD COLUMN IF NOT EXISTS variant_recorded BOOLEAN NOT NULL DEFAULT false;
 CREATE INDEX IF NOT EXISTS market_transactions_recent ON market_transactions(created_at DESC,id DESC);
 CREATE INDEX IF NOT EXISTS market_auctions_resource_sales ON market_auctions(resource_id,id) WHERE item_kind='resource';
 CREATE INDEX IF NOT EXISTS market_auctions_stack_sales ON market_auctions(stack_key,id) WHERE item_kind='stack';
 CREATE INDEX IF NOT EXISTS market_fills_order ON market_fills(order_id,id);
 CREATE TABLE IF NOT EXISTS market_notifications (
 id BIGSERIAL PRIMARY KEY,user_id INTEGER NOT NULL REFERENCES player_profiles(user_id),event_key TEXT NOT NULL UNIQUE,
 message TEXT NOT NULL,target_kind TEXT NOT NULL,target_id BIGINT NOT NULL,read_at TIMESTAMPTZ,created_at TIMESTAMPTZ NOT NULL DEFAULT now());
 CREATE INDEX IF NOT EXISTS market_notifications_inbox ON market_notifications(user_id,id DESC);
 CREATE TABLE IF NOT EXISTS market_rate_events (
 id BIGSERIAL PRIMARY KEY,user_id INTEGER NOT NULL,category TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now());
 CREATE INDEX IF NOT EXISTS market_rate_recent ON market_rate_events(user_id,category,created_at DESC);
 CREATE TABLE IF NOT EXISTS market_audit (
 id BIGSERIAL PRIMARY KEY,actor_user_id INTEGER NOT NULL,event TEXT NOT NULL,subject_kind TEXT,subject_id BIGINT,details JSONB NOT NULL DEFAULT '{}'::jsonb,created_at TIMESTAMPTZ NOT NULL DEFAULT now());
 CREATE INDEX IF NOT EXISTS market_audit_recent ON market_audit(created_at DESC,id DESC);
 CREATE TABLE IF NOT EXISTS market_favorites (
 user_id INTEGER NOT NULL REFERENCES player_profiles(user_id),item_kind TEXT NOT NULL CHECK(item_kind IN ('equipment','resource','stack')),
 item_key TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(user_id,item_kind,item_key));
 CREATE TABLE IF NOT EXISTS market_saved_searches (
 id BIGSERIAL PRIMARY KEY,user_id INTEGER NOT NULL REFERENCES player_profiles(user_id),name TEXT NOT NULL,filters JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),UNIQUE(user_id,name));
 CREATE INDEX IF NOT EXISTS market_saved_searches_user ON market_saved_searches(user_id,id DESC);
 CREATE TABLE IF NOT EXISTS market_idempotency (
 user_id INTEGER NOT NULL REFERENCES player_profiles(user_id),
 request_key UUID NOT NULL,
 action TEXT NOT NULL,
 result_id BIGINT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY (user_id,request_key)
 );
 `)
	return err
}
