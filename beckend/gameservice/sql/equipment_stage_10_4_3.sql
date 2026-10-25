-- Metadata repair for existing Stage 10.3/10.4 template codes only.
-- Alternative to the idempotent startup reconciliation; never deletes instances.
-- All 50 existing active templates must already have been registered.
BEGIN;
LOCK TABLE item_sets,item_templates IN SHARE ROW EXCLUSIVE MODE;
INSERT INTO item_sets(code,name,class_restriction,rarity,description) VALUES
('aegiswarden_set_uncommon','Aegiswarden','guardian','green','monster'),
('stonewarden_set','Stonewarden','guardian','blue','craft'),
('bloodroot_set_uncommon','Bloodroot','berserker','green','monster'),
('emberfang_set','Emberfang','berserker','blue','craft'),
('greenwisp_set_uncommon','Greenwisp','ranger','green','monster'),
('amberveil_set','Amberveil','ranger','blue','craft'),
('sagecloth_set_uncommon','Elderbloom','mystic','green','monster'),
('mooncrystal_set','Mooncrystal','mystic','blue','craft')
ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name,class_restriction=EXCLUDED.class_restriction,rarity=EXCLUDED.rarity,description=EXCLUDED.description;

WITH mapping(code,name,set_code,hero_class,rarity,slot,item_type,handedness,image_url) AS (VALUES
('aegiswarden_sword_uncommon','Aegiswarden Sword','aegiswarden_set_uncommon','guardian','green','main_hand','sword','one_hand','/equipment/guardian/Aegiswarden/Aegiswarden Sword.png'),
('aegiswarden_shield_uncommon','Aegiswarden Shield','aegiswarden_set_uncommon','guardian','green','off_hand','shield','one_hand','/equipment/guardian/Aegiswarden/Aegiswarden Shield.png'),
('aegiswarden_helm_uncommon','Aegiswarden Helm','aegiswarden_set_uncommon','guardian','green','helmet','helmet','none','/equipment/guardian/Aegiswarden/Aegiswarden Helm.png'),
('aegiswarden_vest_uncommon','Aegiswarden Vest','aegiswarden_set_uncommon','guardian','green','chest','chest','none','/equipment/guardian/Aegiswarden/Aegiswarden Vest.png'),
('aegiswarden_pants_uncommon','Aegiswarden Pants','aegiswarden_set_uncommon','guardian','green','pants','pants','none','/equipment/guardian/Aegiswarden/Aegiswarden Pants.png'),
('aegiswarden_gloves_uncommon','Aegiswarden Gloves','aegiswarden_set_uncommon','guardian','green','gloves','gloves','none','/equipment/guardian/Aegiswarden/Aegiswarden Gloves.png'),
('aegiswarden_boots_uncommon','Aegiswarden Boots','aegiswarden_set_uncommon','guardian','green','boots','boots','none','/equipment/guardian/Aegiswarden/boots.png'),
('crafted_guardian_main_hand','Stonewarden Sword','stonewarden_set','guardian','blue','main_hand','sword','one_hand','/equipment/guardian/Stonewarden/stonewarden_sword.png'),
('crafted_guardian_off_hand','Stonewarden Shield','stonewarden_set','guardian','blue','off_hand','shield','one_hand','/equipment/guardian/Stonewarden/stonewarden_shield.png'),
('crafted_guardian_helmet','Stonewarden Helm','stonewarden_set','guardian','blue','helmet','helmet','none','/equipment/guardian/Stonewarden/stonewarden_helm.png'),
('crafted_guardian_chest','Stonewarden Cuirass','stonewarden_set','guardian','blue','chest','chest','none','/equipment/guardian/Stonewarden/stonewarden_cuirass.png'),
('crafted_guardian_pants','Stonewarden Legguards','stonewarden_set','guardian','blue','pants','pants','none','/equipment/guardian/Stonewarden/stonewarden_legguards.png'),
('crafted_guardian_gloves','Stonewarden Gauntlets','stonewarden_set','guardian','blue','gloves','gloves','none','/equipment/guardian/Stonewarden/stonewarden_gauntlets.png'),
('crafted_guardian_boots','Stonewarden Sabatons','stonewarden_set','guardian','blue','boots','boots','none','/equipment/guardian/Stonewarden/stonewarden_sabatons.png'),
('bloodroot_axe_uncommon','Bloodroot Axe','bloodroot_set_uncommon','berserker','green','main_hand','axe','two_hand','/equipment/berserker/Bloodroot/Bloodroot Axe.png'),
('bloodroot_horned_helm_uncommon','Bloodroot Horned Helm','bloodroot_set_uncommon','berserker','green','helmet','helmet','none','/equipment/berserker/Bloodroot/Bloodroot Horned Helm.png'),
('bloodroot_war_vest_uncommon','Bloodroot War Vest','bloodroot_set_uncommon','berserker','green','chest','chest','none','/equipment/berserker/Bloodroot/Bloodroot War Vest.png'),
('bloodroot_raider_pants_uncommon','Bloodroot Raider Pants','bloodroot_set_uncommon','berserker','green','pants','pants','none','/equipment/berserker/Bloodroot/Bloodroot Raider Pants.png'),
('bloodroot_grips_uncommon','Bloodroot Grips','bloodroot_set_uncommon','berserker','green','gloves','gloves','none','/equipment/berserker/Bloodroot/Bloodroot Grips.png'),
('bloodroot_stompers_uncommon','Bloodroot Stompers','bloodroot_set_uncommon','berserker','green','boots','boots','none','/equipment/berserker/Bloodroot/Bloodroot Stompers.png'),
('crafted_berserker_main_hand','Emberfang Ravager Axe','emberfang_set','berserker','blue','main_hand','axe','two_hand','/equipment/berserker/Emberfang/Emberfang_Ravager_Axe.png'),
('crafted_berserker_helmet','Emberfang Ravager Helm','emberfang_set','berserker','blue','helmet','helmet','none','/equipment/berserker/Emberfang/Emberfang_Ravager_Helm.png'),
('crafted_berserker_chest','Emberfang Ravager Chest','emberfang_set','berserker','blue','chest','chest','none','/equipment/berserker/Emberfang/Emberfang_Ravager_Chest.png'),
('crafted_berserker_pants','Emberfang Ravager Pants','emberfang_set','berserker','blue','pants','pants','none','/equipment/berserker/Emberfang/Emberfang_Ravager_Pants.png'),
('crafted_berserker_gloves','Emberfang Ravager Gloves','emberfang_set','berserker','blue','gloves','gloves','none','/equipment/berserker/Emberfang/Emberfang_Ravager_Gloves.png'),
('crafted_berserker_boots','Emberfang Ravager Boots','emberfang_set','berserker','blue','boots','boots','none','/equipment/berserker/Emberfang/Emberfang_Ravager_Boots.png'),
('greenwisp_bow_uncommon','Greenwisp Bow','greenwisp_set_uncommon','ranger','green','main_hand','bow','two_hand','/equipment/ranger/Greenwisp/Greenwisp Bow.png'),
('greenwisp_hood_uncommon','Greenwisp Hood','greenwisp_set_uncommon','ranger','green','helmet','helmet','none','/equipment/ranger/Greenwisp/Greenwisp Hood.png'),
('greenwisp_vest_uncommon','Greenwisp Vest','greenwisp_set_uncommon','ranger','green','chest','chest','none','/equipment/ranger/Greenwisp/Greenwisp Vest.png'),
('greenwisp_ranger_pants_uncommon','Greenwisp Ranger Pants','greenwisp_set_uncommon','ranger','green','pants','pants','none','/equipment/ranger/Greenwisp/Greenwisp Ranger Pants.png'),
('greenwisp_gloves_uncommon','Greenwisp Gloves','greenwisp_set_uncommon','ranger','green','gloves','gloves','none','/equipment/ranger/Greenwisp/Greenwisp Gloves.png'),
('greenwisp_boots_uncommon','Greenwisp Boots','greenwisp_set_uncommon','ranger','green','boots','boots','none','/equipment/ranger/Greenwisp/Greenwisp Boots.png'),
('crafted_ranger_main_hand','Amberveil Bow','amberveil_set','ranger','blue','main_hand','bow','two_hand','/equipment/ranger/Amberveil/06_Amberveil_Bow.png'),
('crafted_ranger_helmet','Amberveil Helm','amberveil_set','ranger','blue','helmet','helmet','none','/equipment/ranger/Amberveil/01_Amberveil_Helm.png'),
('crafted_ranger_chest','Amberveil Chest','amberveil_set','ranger','blue','chest','chest','none','/equipment/ranger/Amberveil/02_Amberveil_Chest.png'),
('crafted_ranger_pants','Amberveil Pants','amberveil_set','ranger','blue','pants','pants','none','/equipment/ranger/Amberveil/04_Amberveil_Pants.png'),
('crafted_ranger_gloves','Amberveil Gloves','amberveil_set','ranger','blue','gloves','gloves','none','/equipment/ranger/Amberveil/03_Amberveil_Gloves.png'),
('crafted_ranger_boots','Amberveil Boots','amberveil_set','ranger','blue','boots','boots','none','/equipment/ranger/Amberveil/05_Amberveil_Boots.png'),
('sagecloth_staff_uncommon','Elderbloom Staff','sagecloth_set_uncommon','mystic','green','main_hand','staff','two_hand','/equipment/mystic/Elderbloom/Elderbloom Staff.png'),
('sagecloth_jacket_uncommon','Elderbloom Vest','sagecloth_set_uncommon','mystic','green','chest','chest','none','/equipment/mystic/Elderbloom/Elderbloom Vest.png'),
('sagecloth_pants_uncommon','Elderbloom Pants','sagecloth_set_uncommon','mystic','green','pants','pants','none','/equipment/mystic/Elderbloom/Elderbloom Pants.png'),
('sagecloth_boots_uncommon','Elderbloom Boots','sagecloth_set_uncommon','mystic','green','boots','boots','none','/equipment/mystic/Elderbloom/Elderbloom Boots.png'),
('sagecloth_gloves_uncommon','Elderbloom Gloves','sagecloth_set_uncommon','mystic','green','gloves','gloves','none','/equipment/mystic/Elderbloom/Elderbloom Gloves.png'),
('sagecloth_hood_uncommon','Elderbloom Hood','sagecloth_set_uncommon','mystic','green','helmet','helmet','none','/equipment/mystic/Elderbloom/Elderbloom Hood.png'),
('crafted_mystic_main_hand','Mooncrystal Staff','mooncrystal_set','mystic','blue','main_hand','staff','two_hand','/equipment/mystic/Mooncrystal/07_mooncrystal_staff.png'),
('crafted_mystic_chest','Mooncrystal Chest','mooncrystal_set','mystic','blue','chest','chest','none','/equipment/mystic/Mooncrystal/01_mooncrystal_chest.png'),
('crafted_mystic_pants','Mooncrystal Pants','mooncrystal_set','mystic','blue','pants','pants','none','/equipment/mystic/Mooncrystal/05_mooncrystal_pants.png'),
('crafted_mystic_boots','Mooncrystal Boots','mooncrystal_set','mystic','blue','boots','boots','none','/equipment/mystic/Mooncrystal/06_mooncrystal_boots.png'),
('crafted_mystic_gloves','Mooncrystal Gloves','mooncrystal_set','mystic','blue','gloves','gloves','none','/equipment/mystic/Mooncrystal/04_mooncrystal_gloves.png'),
('crafted_mystic_helmet','Mooncrystal Helmet','mooncrystal_set','mystic','blue','helmet','helmet','none','/equipment/mystic/Mooncrystal/02_mooncrystal_helmet.png')
) UPDATE item_templates it SET name=m.name,set_id=s.id,class_restriction=m.hero_class,rarity=m.rarity,slot=m.slot,item_type=m.item_type,handedness=m.handedness,image_url=m.image_url
 FROM mapping m JOIN item_sets s ON s.code=m.set_code WHERE it.code=m.code;
-- Fail atomically if this database has not registered the existing 50 codes.
DO $$
BEGIN
 IF (SELECT count(*) FROM item_templates WHERE code IN (
'aegiswarden_sword_uncommon','aegiswarden_shield_uncommon','aegiswarden_helm_uncommon','aegiswarden_vest_uncommon','aegiswarden_pants_uncommon','aegiswarden_gloves_uncommon','aegiswarden_boots_uncommon','crafted_guardian_main_hand','crafted_guardian_off_hand','crafted_guardian_helmet','crafted_guardian_chest','crafted_guardian_pants','crafted_guardian_gloves','crafted_guardian_boots','bloodroot_axe_uncommon','bloodroot_horned_helm_uncommon','bloodroot_war_vest_uncommon','bloodroot_raider_pants_uncommon','bloodroot_grips_uncommon','bloodroot_stompers_uncommon','crafted_berserker_main_hand','crafted_berserker_helmet','crafted_berserker_chest','crafted_berserker_pants','crafted_berserker_gloves','crafted_berserker_boots','greenwisp_bow_uncommon','greenwisp_hood_uncommon','greenwisp_vest_uncommon','greenwisp_ranger_pants_uncommon','greenwisp_gloves_uncommon','greenwisp_boots_uncommon','crafted_ranger_main_hand','crafted_ranger_helmet','crafted_ranger_chest','crafted_ranger_pants','crafted_ranger_gloves','crafted_ranger_boots','sagecloth_staff_uncommon','sagecloth_jacket_uncommon','sagecloth_pants_uncommon','sagecloth_boots_uncommon','sagecloth_gloves_uncommon','sagecloth_hood_uncommon','crafted_mystic_main_hand','crafted_mystic_chest','crafted_mystic_pants','crafted_mystic_boots','crafted_mystic_gloves','crafted_mystic_helmet'
 ))<>50 THEN RAISE EXCEPTION 'Register Stage 10.3/10.4 templates before applying equipment metadata repair'; END IF;
END;
$$;
UPDATE item_set_bonuses b SET description=replace(b.description,'Aegiswarden Set','Aegiswarden') FROM item_sets s WHERE b.set_id=s.id AND s.code='aegiswarden_set_uncommon';
UPDATE item_set_bonuses b SET description=replace(b.description,'Bloodroot Set','Bloodroot') FROM item_sets s WHERE b.set_id=s.id AND s.code='bloodroot_set_uncommon';
UPDATE item_set_bonuses b SET description=replace(b.description,'Greenwisp Set','Greenwisp') FROM item_sets s WHERE b.set_id=s.id AND s.code='greenwisp_set_uncommon';
UPDATE item_set_bonuses b SET description=replace(b.description,'Sagecloth Set','Elderbloom') FROM item_sets s WHERE b.set_id=s.id AND s.code='sagecloth_set_uncommon';
COMMIT;
