package game

import (
	"os"
	"strconv"
	"strings"
)

const (
	DefaultBarrelArtifactChancePercent = 4
	BarrelDamageChancePercent          = 30
	BarrelResourceChancePercent        = 40
)

// The remaining probability is an empty barrel, not another resource or damage
// roll. The upper bound preserves the existing resource and damage categories.
func BarrelArtifactChancePercent() int {
	raw := strings.TrimSpace(os.Getenv("BARREL_ARTIFACT_CHANCE_PERCENT"))
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 || value > 100-BarrelDamageChancePercent-BarrelResourceChancePercent {
		return DefaultBarrelArtifactChancePercent
	}
	return value
}

type EmptyBarrelEvent struct{}
