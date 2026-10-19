package balance

// MoveEnergyCost is shared by square movement and the HEX test route.
func MoveEnergyCost(mobility int) int {
	switch {
	case mobility <= 2:
		return 4
	case mobility <= 5:
		return 3
	case mobility <= 8:
		return 2
	default:
		return 1
	}
}

func CalculateReflexProcChance(reflex int) int {
	if reflex <= 0 {
		return 0
	}

	chance := (30 * reflex) / (reflex + 10)

	if chance > 25 {
		return 25
	}

	return chance
}
