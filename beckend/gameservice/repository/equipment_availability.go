package repository

import "strings"

// Rarity is authoritative; names and image colours are never used. Scope this
// predicate to equipment templates so Common resources/consumables stay intact.
func commonEquipmentSQL(alias string) string {
	if alias != "" {
		alias += "."
	}
	return "(lower(btrim(" + alias + "rarity)) IN ('gray','grey','common') AND " + alias + "slot IN ('main_hand','off_hand','helmet','chest','pants','boots','gloves','ring','amulet') AND " + alias + "item_type IN ('sword','shield','staff','bow','axe','helmet','chest','pants','boots','gloves','ring','amulet'))"
}

func isCommonEquipment(rarity, slot string) bool {
	switch strings.ToLower(strings.TrimSpace(rarity)) {
	case "gray", "grey", "common":
		return isValidEquipmentSlot(slot)
	}
	return false
}
