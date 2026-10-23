-- Before this migration nothing stopped two rows sharing a SHORT_URL: concurrent
-- requests for the same URL could insert it twice, and a hash collision between two
-- different URLs went unnoticed. Clean that up, then enforce uniqueness.

-- 1. Exact duplicates (same SHORT_URL and LONG_URL): keep the oldest row, with the clicks of all of them.
UPDATE URLS u
SET CLICK_COUNT = d.TOTAL_CLICKS
FROM (
    SELECT MIN(ID) AS KEEP_ID, SUM(CLICK_COUNT) AS TOTAL_CLICKS
    FROM URLS
    GROUP BY SHORT_URL, LONG_URL
    HAVING COUNT(*) > 1
) d
WHERE u.ID = d.KEEP_ID;

DELETE FROM URLS u
USING URLS k
WHERE u.SHORT_URL = k.SHORT_URL AND u.LONG_URL = k.LONG_URL AND u.ID > k.ID;

-- 2. Collisions (same SHORT_URL, different LONG_URL): only one URL can own a code.
-- Keep the oldest; the others already redirected unpredictably and must be shortened again.
DELETE FROM URLS u
USING URLS k
WHERE u.SHORT_URL = k.SHORT_URL AND u.ID > k.ID;

-- 3. Enforce one row per code. Also used by INSERT ... ON CONFLICT (SHORT_URL).
ALTER TABLE URLS ADD CONSTRAINT URLS_SHORT_URL_KEY UNIQUE (SHORT_URL);

-- 4. Record when each link was created (existing rows get the migration time).
ALTER TABLE URLS ADD COLUMN CREATED_AT TIMESTAMPTZ NOT NULL DEFAULT NOW();
