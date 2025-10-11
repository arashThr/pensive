-- Add embedding column to library_contents table
-- Using 768 dimensions for Google's gemini-embedding-001 model
ALTER TABLE library_contents ADD COLUMN content_embedding vector(768);

-- Create HNSW index for fast similarity search
-- HNSW (Hierarchical Navigable Small World) is optimized for approximate nearest neighbor search
-- m=16 is the number of connections per layer (higher = better recall, slower build)
-- ef_construction=64 controls index quality (higher = better quality, slower build)
CREATE INDEX content_embedding_hnsw_idx ON library_contents
USING hnsw (content_embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);
