package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gameservice/internal/hexgrid"
)

type HexItemResult struct {
	Budget        *HexBudgets      `json:"budget,omitempty"`
	ActionEffects []map[string]any `json:"actionEffects,omitempty"`
	ItemType      string           `json:"itemType"`
	ItemID        int              `json:"itemId"`
	TurnNumber    int              `json:"turnNumber"`
	Health        int              `json:"health"`
	AP            int              `json:"ap"`
	Inventory     json.RawMessage  `json:"inventory"`
	Axis          string           `json:"axis,omitempty"`
	Value         int              `json:"value,omitempty"`
	StateRevision int64            `json:"stateRevision"`
	MapRevision   int64            `json:"mapRevision"`
	Replayed      bool             `json:"-"`
}

func (r *HexItemResult) SetCommandRevisions(state, mapRevision int64) {
	r.StateRevision, r.MapRevision = state, mapRevision
}

// Use only the match inventory. Selected items are atomically transferred from
// Storage at match creation; using them must not debit the profile again.
func consumeHexInventoryTx(ctx context.Context, tx *sql.Tx, instanceID string, userID int, itemType string, itemID int) (json.RawMessage, error) {
	var raw []byte
	if err := tx.QueryRowContext(ctx, `SELECT inventory FROM match_players WHERE instance_id=$1 AND user_id=$2 FOR UPDATE`, instanceID, userID).Scan(&raw); err != nil {
		return nil, err
	}
	var inv map[string]map[string]any
	if err := json.Unmarshal(raw, &inv); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%s_%d", itemType, itemID)
	entry, ok := inv[key]
	if !ok {
		return nil, ErrCommandRejected
	}
	count, ok := entry["item_count"].(float64)
	if !ok || count < 1 {
		return nil, ErrCommandRejected
	}
	if count == 1 {
		delete(inv, key)
	} else {
		entry["item_count"] = int(count) - 1
	}
	updated, err := json.Marshal(inv)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE match_players SET inventory=$3 WHERE instance_id=$1 AND user_id=$2`, instanceID, userID, updated); err != nil {
		return nil, err
	}
	// Starting loadout and collected items both have normalized match rows.
	if _, err := tx.ExecContext(ctx, `UPDATE inventory_items SET item_count=item_count-1 WHERE instance_id=$1 AND user_id=$2 AND item_type=$3 AND item_id=$4 AND item_count>0`, instanceID, userID, itemType, itemID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM inventory_items WHERE instance_id=$1 AND user_id=$2 AND item_type=$3 AND item_id=$4 AND item_count<=0`, instanceID, userID, itemType, itemID); err != nil {
		return nil, err
	}
	return updated, nil
}

func ApplyHexItem(ctx context.Context, instanceID string, userID int, commandID, itemType string, itemID int, publish func(HexItemResult), targetUsers ...int) (HexItemResult, error) {
	var out HexItemResult
	if itemID <= 0 || (itemType != "resource" && itemType != "scroll") {
		return out, ErrCommandRejected
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	targetUser := 0
	if len(targetUsers) > 0 {
		targetUser = targetUsers[0]
	}
	canonical := fmt.Sprintf("hex-item-v1:%d:%s:%d", userID, itemType, itemID)
	if targetUser != 0 {
		canonical += fmt.Sprintf(":target:%d", targetUser)
	}
	c, err := BeginMatchCommand(ctx, instanceID, userID, commandID, canonical)
	if err != nil {
		return out, err
	}
	defer c.Rollback()
	if c.Replay != nil {
		if err := json.Unmarshal(c.Replay, &out); err != nil {
			return out, err
		}
		out.Replayed = true
		return out, nil
	}
	if c.ActiveUser != userID {
		return out, ErrCommandRejected
	}
	if err := validateMatchItemPocketsTx(ctx, c.Tx, instanceID, userID); err != nil {
		return out, err
	}
	var mode string
	var version, turn, health, maxHealth, ap, maxAP int
	var pos hexgrid.Coord
	err = c.Tx.QueryRowContext(ctx, `SELECT m.geometry->>'kind',m.schema_version,m.turn_number,p.health,p.max_health,p.ap,p.max_ap,p.position_q,p.position_r FROM matches m JOIN match_players p ON p.instance_id=m.instance_id AND p.user_id=$2 WHERE m.instance_id=$1`, instanceID, userID).Scan(&mode, &version, &turn, &health, &maxHealth, &ap, &maxAP, &pos.Q, &pos.R)
	if err != nil {
		return out, err
	}
	if mode != "hex" || version != 2 || health <= 0 {
		return out, ErrCommandRejected
	}
	out.ItemType, out.ItemID, out.TurnNumber, out.Health, out.AP = itemType, itemID, turn, health, ap
	if itemType == "resource" {
		var kind string
		var effectRaw []byte
		if err := c.Tx.QueryRowContext(ctx, `SELECT type,effect FROM resources WHERE id=$1`, itemID).Scan(&kind, &effectRaw); err != nil {
			return out, err
		}
		if recipe, ok := craftRecipe(kind); ok && recipe.Effect != "" {
			if err := c.applyCraftConsumableTx(ctx, userID, targetUser, recipe); err != nil {
				return out, err
			}
			out.Inventory, err = consumeHexInventoryTx(ctx, c.Tx, instanceID, userID, itemType, itemID)
			if err != nil {
				return out, err
			}
			out.StateRevision, out.MapRevision, err = c.Commit(ctx, &out, c.MapChanged)
			if err != nil {
				return HexItemResult{}, err
			}
			if publish != nil {
				publish(out)
			}
			return out, nil
		}
		if targetUser != 0 && targetUser != userID {
			return out, ErrCommandRejected
		}
		if kind != "food" && kind != "water" {
			return out, ErrCommandRejected
		}
		var effect map[string]int
		if err := json.Unmarshal(effectRaw, &effect); err != nil {
			return out, err
		}
		// Square: one food per two turns; at most five water uses in two turns.
		var used int
		if kind == "food" {
			err = c.Tx.QueryRowContext(ctx, `SELECT count(*) FROM match_commands mc JOIN resources r ON r.id=(mc.response->>'itemId')::int WHERE mc.instance_id=$1 AND mc.user_id=$2 AND mc.response->>'itemType'='resource' AND r.type='food' AND (mc.response->>'turnNumber')::int>=$3`, instanceID, userID, turn-1).Scan(&used)
			if err != nil {
				return out, err
			}
			if used > 0 {
				return out, ErrHexFoodCooldown
			}
		} else {
			b, err := ReadHexBudgetsTx(ctx, c.Tx, instanceID, userID)
			if err != nil {
				return out, err
			}
			if b.AP >= b.MaxAP || b.PersonalTurn < b.WaterAvailableTurn {
				return out, ErrHexWaterLimit
			}
			if _, err = c.Tx.ExecContext(ctx, `UPDATE match_players SET water_available_turn=personal_turn_counter+2 WHERE instance_id=$1 AND user_id=$2`, instanceID, userID); err != nil {
				return out, err
			}
		}

		out.Health = health + effect["health"]
		if out.Health > maxHealth {
			out.Health = maxHealth
		}
		out.AP = ap
		if kind == "water" {
			out.AP += 2
		}
		if out.AP > maxAP {
			out.AP = maxAP
		}
		if _, err := c.Tx.ExecContext(ctx, `UPDATE match_players SET health=$3,ap=$4 WHERE instance_id=$1 AND user_id=$2`, instanceID, userID, out.Health, out.AP); err != nil {
			return out, err
		}
	} else {
		if err := c.SpendHexTx(ctx, userID, 2, 0, 0); err != nil {
			return out, err
		}
		if itemID < 2001 || itemID > 2006 {
			return out, ErrCommandRejected
		}
		out.Axis = "q"
		if itemID%2 == 0 {
			out.Axis = "r"
		}
		switch itemID {
		case 2001, 2002:
			var raw []byte
			if err := c.Tx.QueryRowContext(ctx, `SELECT portal_position_v2 FROM matches WHERE instance_id=$1`, instanceID).Scan(&raw); err != nil {
				return out, err
			}
			var target hexgrid.Coord
			if err := json.Unmarshal(raw, &target); err != nil {
				return out, err
			}
			out.Value = target.Q
			if out.Axis == "r" {
				out.Value = target.R
			}
		case 2003, 2004:
			rows, err := c.Tx.QueryContext(ctx, `SELECT position_q,position_r FROM match_players WHERE instance_id=$1 AND user_id<>$2 AND health>0`, instanceID, userID)
			if err != nil {
				return out, err
			}
			nearest := int(^uint(0) >> 1)
			found := false
			for rows.Next() {
				var target hexgrid.Coord
				if err := rows.Scan(&target.Q, &target.R); err != nil {
					rows.Close()
					return out, err
				}
				d, err := hexgrid.Distance(pos, target)
				if err != nil {
					rows.Close()
					return out, err
				}
				if d < nearest {
					nearest = d
					out.Value = target.Q
					if out.Axis == "r" {
						out.Value = target.R
					}
					found = true
				}
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return out, err
			}
			if !found {
				return out, ErrHexNoTarget
			}
		case 2005, 2006:
			rows, err := c.Tx.QueryContext(ctx, `SELECT q,r FROM match_map_cells WHERE instance_id=$1 AND barbel IS NOT NULL`, instanceID)
			if err != nil {
				return out, err
			}
			nearest := int(^uint(0) >> 1)
			found := false
			for rows.Next() {
				var target hexgrid.Coord
				if err := rows.Scan(&target.Q, &target.R); err != nil {
					rows.Close()
					return out, err
				}
				d, err := hexgrid.Distance(pos, target)
				if err != nil {
					rows.Close()
					return out, err
				}
				if d < nearest {
					nearest = d
					out.Value = target.Q
					if out.Axis == "r" {
						out.Value = target.R
					}
					found = true
				}
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return out, err
			}
			if !found {
				return out, ErrHexNoTarget
			}
		}
	}
	out.Inventory, err = consumeHexInventoryTx(ctx, c.Tx, instanceID, userID, itemType, itemID)
	if err != nil {
		return out, err
	}
	out.StateRevision, out.MapRevision, err = c.Commit(ctx, &out, false)
	if err != nil {
		return HexItemResult{}, err
	}
	if publish != nil {
		publish(out)
	}
	return out, nil
}

var ErrHexFoodCooldown = fmt.Errorf("food can be used once every two turns")
var ErrHexWaterLimit = fmt.Errorf("water is unavailable until the next eligible personal turn, or AP is full")
var ErrHexNoTarget = fmt.Errorf("no target for this scroll")
