# Game Database Checks

The auth database remains separate from game state:

- `auth.users` is authentication/account identity only: id, email, password hash, name, created time.
- `game.player_profiles` is the game account profile: cached display name/image, shared balance, shared inventory, and selected character pointer.
- `game.player_characters` is the canonical owned-character table: class id, character image, level, EXP, max EXP, stats, ranged flags, source, and timestamps.
- `game.match_players` is a per-match snapshot copied from the selected character and shared profile inventory when the match starts.

Fresh database checks after `docker compose down -v && docker compose up -d --build`:

```sql
SELECT to_regclass('public.player_profiles');
SELECT to_regclass('public.player_characters');
SELECT to_regclass('public.match_players');
SELECT to_regclass('public.players');
SELECT to_regclass('public.player_heroes');
```

Expected:

- `player_profiles`, `player_characters`, and `match_players` exist.
- `players` returns null.
- `player_heroes` returns null.

Inspect active character progression and stats with:

```sql
SELECT
  pc.id,
  pc.user_id,
  pc.hero_class_id,
  pc.level,
  pc.exp,
  pc.max_exp,
  pc.max_energy,
  pc.max_health,
  pc.attack,
  pc.defense,
  pc.mobility,
  pc.agility,
  pc.sight_range,
  pc.is_ranged,
  pc.attack_range
FROM player_profiles pp
JOIN player_characters pc ON pc.id = pp.selected_character_id
WHERE pp.user_id = 1;
```

Inspect match snapshots with:

```sql
SELECT
  instance_id,
  user_id,
  character_id,
  character_type,
  level,
  experience,
  max_experience,
  attack,
  defense,
  inventory
FROM match_players
WHERE instance_id = '<match-id>';
```
