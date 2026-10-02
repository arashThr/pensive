-- Merged duplicates are not split back apart.
DROP INDEX IF EXISTS feeds_url_key_idx;
ALTER TABLE feeds DROP COLUMN url_key;
