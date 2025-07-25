-- Add extraction method column to library_items table
-- Values can be: server, client-readability and client-html-extraction
ALTER TABLE library_items ADD COLUMN extraction_method TEXT NOT NULL DEFAULT 'server-side';