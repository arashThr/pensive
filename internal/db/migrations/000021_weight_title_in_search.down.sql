ALTER TABLE library_contents DROP COLUMN search_vector;
ALTER TABLE library_contents ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
  immutable_to_tsvector(title || ' ' || excerpt || ' ' || content)
) STORED;
CREATE INDEX search_vector_idx ON library_contents USING GIN(search_vector);
