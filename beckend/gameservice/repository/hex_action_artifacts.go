package repository

import "context"

type hexArtifactDelta struct{ attack, defense, health, reflex, mp int }

func hexArtifactStats(code string) hexArtifactDelta {
	switch code {
	case ArtifactBootsOfStealth:
		return hexArtifactDelta{}
	case ArtifactCrownEnlightenment:
		return hexArtifactDelta{attack: 3, health: -15}
	case ArtifactGuardianShield:
		return hexArtifactDelta{defense: 2, mp: -1}
	case ArtifactRingOfWisdom:
		return hexArtifactDelta{reflex: 1, mp: -1}
	case ArtifactTitanBreastplate:
		return hexArtifactDelta{attack: -3, defense: 2}

	}
	return hexArtifactDelta{}
}

// One selected quest artifact, one reversible stat projection. Never apply at read time.
func (c *MatchCommand) SyncHexArtifactsTx(ctx context.Context) error {
	rows, err := c.Tx.QueryContext(ctx, `SELECT p.user_id,p.hex_applied_artifact,COALESCE(a.name,'') FROM match_players p LEFT JOIN matches m ON m.instance_id=p.instance_id LEFT JOIN inventory_items ii ON ii.instance_id=p.instance_id AND ii.user_id=p.user_id AND ii.item_type='artifact' AND ii.item_id=m.quest_artifact_id AND ii.item_count>0 AND p.health>0 LEFT JOIN artifacts a ON a.id=ii.item_id WHERE p.instance_id=$1 ORDER BY p.user_id`, c.InstanceID)
	if err != nil {
		return err
	}
	type change struct {
		user      int
		old, next string
	}
	all := []change{}
	for rows.Next() {
		var v change
		if err := rows.Scan(&v.user, &v.old, &v.next); err != nil {
			rows.Close()
			return err
		}
		all = append(all, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, v := range all {
		if v.old == v.next {
			continue
		}
		old, next := hexArtifactStats(v.old), hexArtifactStats(v.next)
		_, err = c.Tx.ExecContext(ctx, `UPDATE match_players SET attack=attack+$3,defense=defense+$4,max_health=max_health+$5,health=LEAST(health,max_health+$5),agility=agility+$6,max_mp=GREATEST(0,LEAST(16,max_mp+$7)),mp=GREATEST(0,LEAST(16,mp+LEAST(0,$7))),hex_applied_artifact=$8 WHERE instance_id=$1 AND user_id=$2`, c.InstanceID, v.user, next.attack-old.attack, next.defense-old.defense, next.health-old.health, next.reflex-old.reflex, next.mp-old.mp, v.next)
		if err != nil {
			return err
		}
	}
	return nil
}
