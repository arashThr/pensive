-- Remove the HNSW index
DROP INDEX IF EXISTS content_embedding_hnsw_idx;

-- Remove the embedding column
ALTER TABLE library_contents DROP COLUMN IF EXISTS content_embedding;
