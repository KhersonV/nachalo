-- MANUAL ONLY. Stop writers and take a full backup first; see the stage report.
-- Historical templates/instances are retained as inaccessible tombstones to
-- preserve item_instance_events, sales, bids and all existing foreign keys.
BEGIN;
SET LOCAL lock_timeout = '15s';
SELECT pg_advisory_xact_lock(711009, 1);
LOCK TABLE item_templates,item_instances,character_equipment,market_auctions,
 market_orders,market_deliveries,player_profiles,match_players,inventory_items
 IN SHARE ROW EXCLUSIVE MODE;

CREATE OR REPLACE FUNCTION nachalo_common_equipment(r text,s text,t text)
RETURNS boolean LANGUAGE sql IMMUTABLE AS $$
 SELECT lower(btrim(r)) IN ('gray','grey','common')
 AND s IN ('main_hand','off_hand','helmet','chest','pants','boots','gloves','ring','amulet')
 AND t IN ('sword','shield','staff','bow','axe','helmet','chest','pants','boots','gloves','ring','amulet')
$$;
CREATE TEMP TABLE common_templates ON COMMIT DROP AS
 SELECT * FROM item_templates WHERE nachalo_common_equipment(rarity,slot,item_type);
CREATE UNIQUE INDEX ON common_templates(id);
CREATE TEMP TABLE common_instances ON COMMIT DROP AS
 SELECT ii.* FROM item_instances ii JOIN common_templates t ON t.id=ii.template_id;
CREATE UNIQUE INDEX ON common_instances(id);
-- Exact manifest appears before mutations, and is also durably archived below.
SELECT id,code,rarity,slot,item_type FROM common_templates ORDER BY id;
SELECT id,template_id,owner_user_id,current_character_id,status FROM common_instances ORDER BY id;

CREATE TABLE IF NOT EXISTS common_equipment_retirement_audit (
 entity text NOT NULL, entity_key text NOT NULL, original_row jsonb NOT NULL,
 retired_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(entity,entity_key)
);
CREATE TEMP TABLE retirement_report(entity text, affected bigint) ON COMMIT DROP;
INSERT INTO common_equipment_retirement_audit
 SELECT 'item_templates',id::text,to_jsonb(t),now() FROM common_templates t ON CONFLICT DO NOTHING;
INSERT INTO common_equipment_retirement_audit
 SELECT 'item_instances',id::text,to_jsonb(i),now() FROM common_instances i ON CONFLICT DO NOTHING;
INSERT INTO common_equipment_retirement_audit
 SELECT 'character_equipment',ce.id::text,to_jsonb(ce),now() FROM character_equipment ce
 JOIN common_instances i ON i.id=ce.item_instance_id ON CONFLICT DO NOTHING;
INSERT INTO common_equipment_retirement_audit
 SELECT 'market_auctions',a.id::text,to_jsonb(a),now() FROM market_auctions a
 JOIN common_instances i ON i.id=a.item_instance_id ON CONFLICT DO NOTHING;
INSERT INTO common_equipment_retirement_audit
 SELECT 'market_orders',o.id::text,to_jsonb(o),now() FROM market_orders o
 JOIN common_templates t ON t.id=o.template_id WHERE o.item_kind='equipment' ON CONFLICT DO NOTHING;
INSERT INTO common_equipment_retirement_audit
 SELECT 'market_deliveries',d.id::text,to_jsonb(d),now() FROM market_deliveries d
 JOIN common_instances i ON i.id=d.item_instance_id ON CONFLICT DO NOTHING;

-- Return reserved funds as claims, without changing any account's gold balance.
-- Only active escrow is refundable; cancellation in this same transaction makes
-- reruns safe. Retain all bids/transactions and the original escrow audit row.
WITH refunds AS (
 INSERT INTO market_wallet_claims(recipient_user_id,amount)
 SELECT a.highest_bidder_user_id,a.highest_bid FROM market_auctions a
 JOIN common_instances i ON i.id=a.item_instance_id
 WHERE a.status='active' AND a.highest_bidder_user_id IS NOT NULL AND a.highest_bid>0 RETURNING id
) INSERT INTO retirement_report SELECT 'market_wallet_claims:auction_refunds',count(*) FROM refunds;
WITH refunds AS (
 INSERT INTO market_wallet_claims(recipient_user_id,amount)
 SELECT o.buyer_user_id,o.reserved_remaining FROM market_orders o
 JOIN common_templates t ON t.id=o.template_id
 WHERE o.item_kind='equipment' AND o.status IN ('open','partially_filled') AND o.reserved_remaining>0 RETURNING id
) INSERT INTO retirement_report SELECT 'market_wallet_claims:order_refunds',count(*) FROM refunds;
WITH changed AS (
 UPDATE market_auctions a SET status='cancelled',settled_at=now()
 FROM common_instances i WHERE i.id=a.item_instance_id AND a.status='active' RETURNING a.id
) INSERT INTO retirement_report SELECT 'market_auctions:closed',count(*) FROM changed;
WITH changed AS (
 UPDATE market_orders o SET status='cancelled',reserved_remaining=0,settled_at=now()
 FROM common_templates t WHERE t.id=o.template_id AND o.item_kind='equipment'
 AND o.status IN ('open','partially_filled') RETURNING o.id
) INSERT INTO retirement_report SELECT 'market_orders:closed',count(*) FROM changed;
WITH changed AS (
 UPDATE market_deliveries d SET claimed_at=now() FROM common_instances i
 WHERE i.id=d.item_instance_id AND d.claimed_at IS NULL RETURNING d.id
) INSERT INTO retirement_report SELECT 'market_deliveries:retired',count(*) FROM changed;
WITH changed AS (
 DELETE FROM character_equipment ce USING common_instances i WHERE i.id=ce.item_instance_id RETURNING ce.id
) INSERT INTO retirement_report SELECT 'character_equipment:unequipped',count(*) FROM changed;
WITH changed AS (
INSERT INTO item_instance_events(item_instance_id,event_type,from_user_id,metadata)
 SELECT i.id,'deleted',i.owner_user_id,jsonb_build_object('reason','remove_common_equipment')
 FROM common_instances i WHERE i.status<>'deleted' RETURNING id
) INSERT INTO retirement_report SELECT 'item_instance_events:deleted_event',count(*) FROM changed;
WITH changed AS (
 UPDATE item_instances ii SET status='deleted',current_character_id=NULL,
 match_instance_id=NULL,is_locked=false,version=ii.version+1,updated_at=now()
 FROM common_instances i WHERE i.id=ii.id AND (ii.status<>'deleted' OR ii.current_character_id IS NOT NULL OR ii.match_instance_id IS NOT NULL OR ii.is_locked)
 RETURNING ii.id
) INSERT INTO retirement_report SELECT 'item_instances:retired',count(*) FROM changed;
-- Tombstones remain readable by history, but are never available to the market.
WITH changed AS (
 UPDATE item_templates it SET tradable=false,updated_at=now() FROM common_templates t
 WHERE it.id=t.id AND it.tradable RETURNING it.id
) INSERT INTO retirement_report SELECT 'item_templates:retired',count(*) FROM changed;

-- Canonical equipment lives in item_instances. Clean positively identified
-- legacy JSON references as well; never infer equipment from a numeric resource
-- ID, a name, an image or a Common rarity alone.
CREATE OR REPLACE FUNCTION pg_temp.clean_common_gear(v jsonb) RETURNS jsonb
LANGUAGE plpgsql AS $$
DECLARE result jsonb; pair record; cleaned jsonb;
BEGIN
 IF jsonb_typeof(v)='object' THEN
  IF COALESCE(v->>'item_type',v->>'itemType','') IN ('resource','artifact','scroll','consumable','material','blueprint') THEN RETURN v; END IF;
  IF EXISTS(SELECT 1 FROM common_instances i WHERE i.id::text IN (v->>'instanceId',v->>'itemInstanceId',v->>'item_instance_id'))
   OR EXISTS(SELECT 1 FROM common_templates t WHERE t.id::text IN (v->>'templateId',v->>'template_id'))
   OR (COALESCE(v->>'item_type',v->>'itemType','') IN ('equipment','sword','shield','staff','bow','axe','helmet','chest','pants','boots','gloves','ring','amulet')
       AND EXISTS(SELECT 1 FROM common_templates t WHERE t.id::text=COALESCE(v->>'item_id',v->>'id') OR t.code=COALESCE(v->>'code',v->>'templateCode')))
   THEN RETURN NULL; END IF;
  result:='{}'::jsonb;
  FOR pair IN SELECT * FROM jsonb_each(v) LOOP
   IF EXISTS(SELECT 1 FROM common_instances i WHERE pair.key IN (i.id::text,'equipment_'||i.id::text,'item_'||i.id::text))
    OR EXISTS(SELECT 1 FROM common_templates t WHERE pair.key IN ('equipment_'||t.id::text,'equipment_'||t.code)) THEN CONTINUE; END IF;
   cleaned:=pg_temp.clean_common_gear(pair.value);
   IF cleaned IS NOT NULL THEN result:=result||jsonb_build_object(pair.key,cleaned); END IF;
  END LOOP;
  RETURN result;
 ELSIF jsonb_typeof(v)='array' THEN
  result:='[]'::jsonb;
  FOR pair IN SELECT value FROM jsonb_array_elements(v) LOOP
   cleaned:=pg_temp.clean_common_gear(pair.value);
   IF cleaned IS NOT NULL THEN result:=result||jsonb_build_array(cleaned); END IF;
  END LOOP;
  RETURN result;
 END IF;
 RETURN v;
END $$;
INSERT INTO common_equipment_retirement_audit
 SELECT 'player_profiles.inventory',p.user_id::text,jsonb_build_object('inventory',p.inventory),now()
 FROM player_profiles p WHERE p.inventory IS DISTINCT FROM COALESCE(pg_temp.clean_common_gear(p.inventory),'{}'::jsonb) ON CONFLICT DO NOTHING;
WITH changed AS (
 UPDATE player_profiles SET inventory=COALESCE(pg_temp.clean_common_gear(inventory),'{}'::jsonb)
 WHERE inventory IS DISTINCT FROM COALESCE(pg_temp.clean_common_gear(inventory),'{}'::jsonb) RETURNING user_id
) INSERT INTO retirement_report SELECT 'player_profiles.inventory:cleaned',count(*) FROM changed;
INSERT INTO common_equipment_retirement_audit
 SELECT 'match_players.inventory',p.instance_id||':'||p.user_id::text,jsonb_build_object('inventory',p.inventory),now()
 FROM match_players p WHERE p.inventory IS DISTINCT FROM COALESCE(pg_temp.clean_common_gear(p.inventory),'{}'::jsonb) ON CONFLICT DO NOTHING;
WITH changed AS (
 UPDATE match_players SET inventory=COALESCE(pg_temp.clean_common_gear(inventory),'{}'::jsonb)
 WHERE inventory IS DISTINCT FROM COALESCE(pg_temp.clean_common_gear(inventory),'{}'::jsonb) RETURNING user_id
) INSERT INTO retirement_report SELECT 'match_players.inventory:cleaned',count(*) FROM changed;
INSERT INTO common_equipment_retirement_audit
 SELECT 'inventory_items',i.id::text,to_jsonb(i),now() FROM inventory_items i
 JOIN common_templates t ON t.id=i.item_id WHERE i.item_type IN ('equipment','sword','shield','staff','bow','axe','helmet','chest','pants','boots','gloves','ring','amulet') ON CONFLICT DO NOTHING;
WITH changed AS (
 DELETE FROM inventory_items i USING common_templates t WHERE t.id=i.item_id
 AND i.item_type IN ('equipment','sword','shield','staff','bow','axe','helmet','chest','pants','boots','gloves','ring','amulet') RETURNING i.id
) INSERT INTO retirement_report SELECT 'inventory_items:deleted',count(*) FROM changed;

-- Enforce retirement even for old clients, arbitrary grants and direct inserts.
CREATE OR REPLACE FUNCTION nachalo_reject_common_instance() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.status<>'deleted' AND EXISTS(SELECT 1 FROM item_templates t WHERE t.id=NEW.template_id AND nachalo_common_equipment(t.rarity,t.slot,t.item_type))
 THEN RAISE EXCEPTION 'Common equipment is retired'; END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS reject_common_instance ON item_instances;
CREATE TRIGGER reject_common_instance BEFORE INSERT OR UPDATE ON item_instances FOR EACH ROW EXECUTE FUNCTION nachalo_reject_common_instance();
CREATE OR REPLACE FUNCTION nachalo_reject_common_template() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF nachalo_common_equipment(NEW.rarity,NEW.slot,NEW.item_type) THEN
  IF TG_OP='INSERT' THEN RAISE EXCEPTION 'Common equipment is retired'; END IF;
  IF NOT nachalo_common_equipment(OLD.rarity,OLD.slot,OLD.item_type) THEN RAISE EXCEPTION 'Common equipment is retired'; END IF;
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS reject_common_template ON item_templates;
CREATE TRIGGER reject_common_template BEFORE INSERT OR UPDATE ON item_templates FOR EACH ROW EXECUTE FUNCTION nachalo_reject_common_template();
CREATE OR REPLACE FUNCTION nachalo_reject_common_equip() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM item_instances ii JOIN item_templates t ON t.id=ii.template_id WHERE ii.id=NEW.item_instance_id AND nachalo_common_equipment(t.rarity,t.slot,t.item_type))
 THEN RAISE EXCEPTION 'Common equipment is retired'; END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS reject_common_equip ON character_equipment;
CREATE TRIGGER reject_common_equip BEFORE INSERT OR UPDATE ON character_equipment FOR EACH ROW EXECUTE FUNCTION nachalo_reject_common_equip();

SELECT * FROM retirement_report ORDER BY entity;
SELECT entity,count(*) AS archived_rows FROM common_equipment_retirement_audit GROUP BY entity ORDER BY entity;
COMMIT;
