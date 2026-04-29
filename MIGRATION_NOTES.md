# Migration Notes

This project now uses a clean game database model for fresh databases. Local data does not need preservation; reset with:

```sh
docker compose down -v
docker compose up -d --build
```

Architecture:

- `auth.users` remains the authentication account table and is not modified by the game schema.
- `game.player_profiles` replaces the old mixed game profile table. It stores only account-level game state: user id, cached display fields, balance, inventory, selected character id, and timestamps.
- `game.player_characters` stores every concrete owned character. Character class, image, level, EXP, max EXP, stats, ranged flags, and source all live here.
- `game.match_players` stores the immutable character snapshot used for a match. After match creation, later character switches or stat changes do not mutate the snapshot.

Removed model:

- The old `players` table is no longer created.
- `player_heroes` is no longer created or read.
- The previous starter class named `adventurer` is removed; default/fallback class is `guardian`.
- Purchase history is not stored on `player_characters`. Add a separate transaction/event table later if that history becomes gameplay-relevant.

Flow notes:

- New profile creation inserts `player_profiles`, creates the initial `player_characters` row from the requested class or `guardian`, then stores that character id in `player_profiles.selected_character_id`.
- Hiring validates Tavern, balance, catalog class, and duplicate ownership, then deducts shared profile balance and creates a full `player_characters` row.
- Active hero selection accepts `heroClassId`, finds the owned character for that class, and updates `player_profiles.selected_character_id`.
- Profile reads join `player_profiles` to the selected character and expose existing response fields from `player_characters`.
- Match creation copies selected character data into `match_players`, including `character_id`, class, image, progression, stats, health/energy caps, and profile inventory.
- Match finalization awards EXP to `player_characters` by `match_players.character_id`; profile rows do not carry character EXP.
