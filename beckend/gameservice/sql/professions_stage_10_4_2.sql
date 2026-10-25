BEGIN;
-- Run inside a transaction, after the base profession schema exists.
-- The marker and conversion commit together. Never infer format from level alone:
-- old levels 1..3 overlap the new ranks. No match staging/ledger is rewritten.
SELECT pg_advisory_xact_lock(hashtext('nachalo:profession-schema'));
LOCK TABLE character_professions IN ACCESS EXCLUSIVE MODE;
CREATE TABLE IF NOT EXISTS profession_progression_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM profession_progression_migrations WHERE version='10.4.2-cumulative-three-ranks') THEN
        -- Restore 100 * sum(1^2..(L-1)^2) plus the old in-level remainder.
        -- Retain the entire earned total, including legacy totals above 7000.
        WITH restored AS (
            SELECT character_id,
                gathering_xp + 100::bigint*(gathering_level-1)*gathering_level*(2*gathering_level-1)/6 AS gathering_total,
                processing_xp + 100::bigint*(processing_level-1)*processing_level*(2*processing_level-1)/6 AS processing_total
            FROM character_professions
        )
        UPDATE character_professions p SET
            gathering_xp=r.gathering_total,
            gathering_level=CASE WHEN r.gathering_total>=7000 THEN 3 WHEN r.gathering_total>=2000 THEN 2 ELSE 1 END,
            processing_xp=r.processing_total,
            processing_level=CASE WHEN r.processing_total>=7000 THEN 3 WHEN r.processing_total>=2000 THEN 2 ELSE 1 END
        FROM restored r WHERE p.character_id=r.character_id;

        ALTER TABLE character_professions DROP CONSTRAINT IF EXISTS character_professions_gathering_level_check;
        ALTER TABLE character_professions DROP CONSTRAINT IF EXISTS character_professions_processing_level_check;
        ALTER TABLE character_professions ADD CONSTRAINT character_professions_gathering_level_check CHECK (gathering_level BETWEEN 1 AND 3);
        ALTER TABLE character_professions ADD CONSTRAINT character_professions_processing_level_check CHECK (processing_level BETWEEN 1 AND 3);
        ALTER TABLE character_professions ADD CONSTRAINT character_professions_gathering_progress_check CHECK
            (gathering_level=CASE WHEN gathering_xp>=7000 THEN 3 WHEN gathering_xp>=2000 THEN 2 ELSE 1 END);
        ALTER TABLE character_professions ADD CONSTRAINT character_professions_processing_progress_check CHECK
            (processing_level=CASE WHEN processing_xp>=7000 THEN 3 WHEN processing_xp>=2000 THEN 2 ELSE 1 END);
        INSERT INTO profession_progression_migrations(version) VALUES('10.4.2-cumulative-three-ranks');
    END IF;
END;
$$;

COMMIT;
