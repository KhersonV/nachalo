-- Read-only manifest. Save and review this output before running the migration.
BEGIN READ ONLY;
SELECT current_database() AS database, now() AS inspected_at;
SELECT id,code,rarity,slot,item_type FROM item_templates
WHERE lower(btrim(rarity)) IN ('gray','grey','common')
AND slot IN ('main_hand','off_hand','helmet','chest','pants','boots','gloves','ring','amulet')
AND item_type IN ('sword','shield','staff','bow','axe','helmet','chest','pants','boots','gloves','ring','amulet') ORDER BY id;
WITH targets AS (
 SELECT id FROM item_templates WHERE lower(btrim(rarity)) IN ('gray','grey','common')
 AND slot IN ('main_hand','off_hand','helmet','chest','pants','boots','gloves','ring','amulet')
 AND item_type IN ('sword','shield','staff','bow','axe','helmet','chest','pants','boots','gloves','ring','amulet')
)
SELECT ii.id,ii.template_id,ii.owner_user_id,ii.current_character_id,ii.status,ii.match_instance_id
FROM item_instances ii JOIN targets t ON t.id=ii.template_id ORDER BY ii.id;
WITH targets AS (
 SELECT id FROM item_templates WHERE lower(btrim(rarity)) IN ('gray','grey','common')
 AND slot IN ('main_hand','off_hand','helmet','chest','pants','boots','gloves','ring','amulet')
 AND item_type IN ('sword','shield','staff','bow','axe','helmet','chest','pants','boots','gloves','ring','amulet')
)
SELECT a.id AS auction_id,a.status,a.highest_bidder_user_id,a.highest_bid
FROM market_auctions a JOIN item_instances ii ON ii.id=a.item_instance_id JOIN targets t ON t.id=ii.template_id ORDER BY a.id;
WITH targets AS (
 SELECT id FROM item_templates WHERE lower(btrim(rarity)) IN ('gray','grey','common')
 AND slot IN ('main_hand','off_hand','helmet','chest','pants','boots','gloves','ring','amulet')
 AND item_type IN ('sword','shield','staff','bow','axe','helmet','chest','pants','boots','gloves','ring','amulet')
)
SELECT o.id AS order_id,o.buyer_user_id,o.status,o.reserved_remaining
FROM market_orders o JOIN targets t ON t.id=o.template_id WHERE o.item_kind='equipment' ORDER BY o.id;
COMMIT;
