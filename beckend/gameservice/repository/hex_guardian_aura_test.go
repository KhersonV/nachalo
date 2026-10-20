package repository

import (
	"testing"

	"gameservice/internal/hexgrid"
)

func TestHexGuardianAuraExitAccumulatesAcrossMoves(t *testing.T) {
	guardians := []hexGuardian{{userID: 8, teamID: 2, position: hexgrid.Coord{Q: 0, R: 0}}}
	extra, pressure, hits := planHexGuardianAura(hexgrid.Coord{Q: 1, R: 0}, []hexgrid.Coord{{Q: 2, R: 0}}, 3, 1, guardians, hexAuraPressure{})
	if extra != 1 || pressure.amount != 1 || len(hits) != 0 {
		t.Fatalf("inside aura: extra=%d pressure=%+v hits=%+v", extra, pressure, hits)
	}
	extra, pressure, hits = planHexGuardianAura(hexgrid.Coord{Q: 2, R: 0}, []hexgrid.Coord{{Q: 3, R: 0}}, 3, 1, guardians, pressure)
	if extra != 1 || pressure.amount != 0 || len(hits) != 1 || hits[0].damage != 2 || hits[0].sourceUserID != 8 {
		t.Fatalf("exit aura: extra=%d pressure=%+v hits=%+v", extra, pressure, hits)
	}
	_, pressure, hits = planHexGuardianAura(hexgrid.Coord{Q: 3, R: 0}, []hexgrid.Coord{{Q: 2, R: 0}, {Q: 3, R: 0}}, 3, 1, guardians, pressure)
	if pressure.amount != 0 || len(hits) != 1 || hits[0].damage != 1 {
		t.Fatalf("reentry: pressure=%+v hits=%+v", pressure, hits)
	}
}

func TestHexGuardianAuraCapAlliesAndOverlaps(t *testing.T) {
	guardians := []hexGuardian{
		{userID: 9, teamID: 2, position: hexgrid.Coord{Q: 0, R: 0}},
		{userID: 4, teamID: 2, position: hexgrid.Coord{Q: 1, R: 0}},
		{userID: 2, teamID: 1, position: hexgrid.Coord{Q: 0, R: 1}},
	}
	if source := hexAuraSourceAt(hexgrid.Coord{Q: 1, R: 0}, 3, 1, guardians); source != 4 {
		t.Fatalf("lowest enemy source = %d", source)
	}
	if source := hexAuraSourceAt(hexgrid.Coord{Q: 1, R: 0}, 3, 2, guardians); source != 2 {
		t.Fatalf("friendly guardians should be skipped, source = %d", source)
	}
	_, pressure, hits := planHexGuardianAura(hexgrid.Coord{Q: 2, R: 0}, []hexgrid.Coord{{Q: 3, R: 0}}, 3, 1, guardians, hexAuraPressure{amount: 8, sourceUserID: 9})
	if pressure.amount != 9 || len(hits) != 0 {
		t.Fatalf("overlap should remain active: pressure=%+v hits=%+v", pressure, hits)
	}
	_, pressure, hits = planHexGuardianAura(hexgrid.Coord{Q: 3, R: 0}, []hexgrid.Coord{{Q: 4, R: 0}}, 3, 1, guardians, pressure)
	if len(hits) != 1 || hits[0].damage != 5 || hits[0].sourceUserID != 4 || pressure.amount != 0 {
		t.Fatalf("capped exit = %+v pressure=%+v", hits, pressure)
	}
}

func TestHexGuardianAuraMultipleExitsOnOneRoute(t *testing.T) {
	guardians := []hexGuardian{
		{userID: 8, teamID: 2, position: hexgrid.Coord{Q: 0, R: 0}},
		{userID: 9, teamID: 2, position: hexgrid.Coord{Q: 7, R: 0}},
	}
	path := []hexgrid.Coord{
		{Q: 3, R: 0}, {Q: 4, R: 0}, {Q: 5, R: 0},
		{Q: 6, R: 0}, {Q: 7, R: 0}, {Q: 8, R: 0},
		{Q: 9, R: 0}, {Q: 10, R: 0},
	}
	extra, pressure, hits := planHexGuardianAura(hexgrid.Coord{Q: 2, R: 0}, path, 3, 1, guardians, hexAuraPressure{})
	if extra != 6 || pressure.amount != 0 || len(hits) != 2 || hits[0].damage != 1 || hits[0].sourceUserID != 8 || hits[1].damage != 5 || hits[1].sourceUserID != 9 {
		t.Fatalf("each voluntary exit should discharge once: extra=%d pressure=%+v hits=%+v", extra, pressure, hits)
	}
}
