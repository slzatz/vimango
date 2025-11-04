-- Migration script to fix the 'added' field timezone issue
-- This changes the PostgreSQL 'added' column from DATE to TIMESTAMP WITHOUT TIME ZONE
-- to match the SQLite datetime behavior and preserve time information

-- Before running this migration:
-- 1. Back up your PostgreSQL database
-- 2. Verify you have permission to alter the table
-- 3. Note that existing DATE values will be converted to timestamps with 00:00:00 time

-- Change the 'added' column type from DATE to TIMESTAMP WITHOUT TIME ZONE
ALTER TABLE task ALTER COLUMN added TYPE timestamp without time zone;

-- Verify the change
-- You can run this query to confirm the new type:
-- SELECT column_name, data_type
-- FROM information_schema.columns
-- WHERE table_name = 'task' AND column_name = 'added';

-- Note: Existing data will have 00:00:00 as the time component since they were
-- stored as DATE. New entries created after this migration will preserve the
-- full timestamp from SQLite clients.
