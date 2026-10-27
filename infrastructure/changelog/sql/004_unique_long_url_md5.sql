-- One row per long URL, enforced by the database. Shortening runs a single
-- INSERT ... ON CONFLICT ((md5(LONG_URL))) DO UPDATE ... RETURNING, which uses this index
-- to find a URL that was shortened before and return its existing code.
-- The index is on md5(LONG_URL), not LONG_URL itself: a btree entry is capped at about
-- 2.7 kB, which a 1000-character multi-byte URL can exceed, and md5 keeps entries small.
-- The changeset's precondition stops the migration if two rows share an md5: they have
-- different short codes (SHORT_URL is unique since 003), so deleting one would break a link.
CREATE UNIQUE INDEX ux_urls_long_url_md5 ON URLS ((md5(LONG_URL)));
