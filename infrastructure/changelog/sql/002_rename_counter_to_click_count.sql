-- COUNTER_VALUE counted how often a URL was shortened; from now on the column counts
-- redirects (clicks). Old values meant something else, so they are reset to 0.
ALTER TABLE URLS RENAME COLUMN COUNTER_VALUE TO CLICK_COUNT;
UPDATE URLS SET CLICK_COUNT = 0;
ALTER TABLE URLS ALTER COLUMN CLICK_COUNT SET DEFAULT 0;
