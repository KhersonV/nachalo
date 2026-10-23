package repository

import (
	"math/rand"
	"os"
	"strconv"
	"strings"
)

const DefaultEquipmentDropChancePercent = 10

func EquipmentDropChancePercent() int {
	raw := strings.TrimSpace(os.Getenv("EQUIPMENT_DROP_CHANCE_OVERRIDE"))
	if raw == "" {
		return DefaultEquipmentDropChancePercent
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 || value > 100 {
		return DefaultEquipmentDropChancePercent
	}
	return value
}

func RollEquipmentDropChancePercent(chancePercent int) bool {
	return chancePercent > 0 && rand.Intn(100) < chancePercent
}

func RollEquipmentDropChance() bool {
	return RollEquipmentDropChancePercent(EquipmentDropChancePercent())
}
