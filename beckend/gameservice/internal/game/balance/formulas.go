package balance

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
