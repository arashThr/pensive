-- Rebuild the full-text vector so the title (A) and excerpt (B) outrank body
-- text; ts_rank's default weights make an A match count 10x a body match.
ALTER TABLE library_contents DROP COLUMN search_vector;
ALTER TABLE library_contents ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
  setweight(to_tsvector('english', title), 'A') ||
  setweight(to_tsvector('english', excerpt), 'B') ||
  to_tsvector('english', content)
) STORED;
CREATE INDEX search_vector_idx ON library_contents USING GIN(search_vector);

-- Renamed bookmarks only updated library_items.title; bring the searchable copy in sync.
UPDATE library_contents lc
SET title = li.title
FROM library_items li
WHERE lc.id = li.id AND lc.title IS DISTINCT FROM li.title;
