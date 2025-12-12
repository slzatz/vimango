-- PostgreSQL UUID Migration Script
-- This script migrates an existing vimango PostgreSQL database to include UUID columns.
-- It is idempotent - safe to run multiple times.
--
-- IMPORTANT: Backup your database before running this script!
--   pg_dump -h your_host -U your_user -d your_db > backup.sql
--
-- Usage:
--   psql -h your_host -U your_user -d your_db -f postgres_migrate_uuid.sql

-- Enable UUID extension if not already enabled
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Default UUIDs for "none" containers (must match init.go constants)
-- DefaultContextUUID = '00000000-0000-0000-0000-000000000001'
-- DefaultFolderUUID  = '00000000-0000-0000-0000-000000000002'

BEGIN;

-- ============================================================================
-- Step 1: Add UUID columns to container tables
-- ============================================================================

-- Add uuid column to context table
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'context' AND column_name = 'uuid') THEN
        ALTER TABLE context ADD COLUMN uuid TEXT UNIQUE;
        RAISE NOTICE 'Added uuid column to context table';
    ELSE
        RAISE NOTICE 'uuid column already exists in context table';
    END IF;
END $$;

-- Add uuid column to folder table
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'folder' AND column_name = 'uuid') THEN
        ALTER TABLE folder ADD COLUMN uuid TEXT UNIQUE;
        RAISE NOTICE 'Added uuid column to folder table';
    ELSE
        RAISE NOTICE 'uuid column already exists in folder table';
    END IF;
END $$;

-- Add uuid column to keyword table
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'keyword' AND column_name = 'uuid') THEN
        ALTER TABLE keyword ADD COLUMN uuid TEXT UNIQUE;
        RAISE NOTICE 'Added uuid column to keyword table';
    ELSE
        RAISE NOTICE 'uuid column already exists in keyword table';
    END IF;
END $$;

-- ============================================================================
-- Step 2: Generate UUIDs for existing containers
-- ============================================================================

-- Set default UUID for "none" context (tid=1)
UPDATE context
SET uuid = '00000000-0000-0000-0000-000000000001'
WHERE tid = 1 AND (uuid IS NULL OR uuid = '');

-- Set default UUID for "none" folder (tid=1)
UPDATE folder
SET uuid = '00000000-0000-0000-0000-000000000002'
WHERE tid = 1 AND (uuid IS NULL OR uuid = '');

-- Generate UUIDs for all other contexts without UUIDs
UPDATE context
SET uuid = uuid_generate_v4()::TEXT
WHERE uuid IS NULL OR uuid = '';

-- Generate UUIDs for all other folders without UUIDs
UPDATE folder
SET uuid = uuid_generate_v4()::TEXT
WHERE uuid IS NULL OR uuid = '';

-- Generate UUIDs for all keywords without UUIDs
UPDATE keyword
SET uuid = uuid_generate_v4()::TEXT
WHERE uuid IS NULL OR uuid = '';

-- Report counts
DO $$
DECLARE
    ctx_count INTEGER;
    fld_count INTEGER;
    kwd_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO ctx_count FROM context WHERE uuid IS NOT NULL;
    SELECT COUNT(*) INTO fld_count FROM folder WHERE uuid IS NOT NULL;
    SELECT COUNT(*) INTO kwd_count FROM keyword WHERE uuid IS NOT NULL;
    RAISE NOTICE 'Containers with UUIDs - contexts: %, folders: %, keywords: %',
                 ctx_count, fld_count, kwd_count;
END $$;

-- ============================================================================
-- Step 3: Add UUID columns to task table
-- ============================================================================

-- Add context_uuid column to task table
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'task' AND column_name = 'context_uuid') THEN
        ALTER TABLE task ADD COLUMN context_uuid TEXT
            DEFAULT '00000000-0000-0000-0000-000000000001';
        RAISE NOTICE 'Added context_uuid column to task table';
    ELSE
        RAISE NOTICE 'context_uuid column already exists in task table';
    END IF;
END $$;

-- Add folder_uuid column to task table
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'task' AND column_name = 'folder_uuid') THEN
        ALTER TABLE task ADD COLUMN folder_uuid TEXT
            DEFAULT '00000000-0000-0000-0000-000000000002';
        RAISE NOTICE 'Added folder_uuid column to task table';
    ELSE
        RAISE NOTICE 'folder_uuid column already exists in task table';
    END IF;
END $$;

-- ============================================================================
-- Step 4: Populate task UUID references from tid references
-- ============================================================================

-- Migrate context_uuid from context_tid
UPDATE task t
SET context_uuid = c.uuid
FROM context c
WHERE c.tid = t.context_tid
  AND (t.context_uuid IS NULL OR t.context_uuid = ''
       OR t.context_uuid = '00000000-0000-0000-0000-000000000001');

-- Migrate folder_uuid from folder_tid
UPDATE task t
SET folder_uuid = f.uuid
FROM folder f
WHERE f.tid = t.folder_tid
  AND (t.folder_uuid IS NULL OR t.folder_uuid = ''
       OR t.folder_uuid = '00000000-0000-0000-0000-000000000002');

-- Report task migration counts
DO $$
DECLARE
    task_count INTEGER;
    ctx_migrated INTEGER;
    fld_migrated INTEGER;
BEGIN
    SELECT COUNT(*) INTO task_count FROM task;
    SELECT COUNT(*) INTO ctx_migrated FROM task WHERE context_uuid IS NOT NULL AND context_uuid != '';
    SELECT COUNT(*) INTO fld_migrated FROM task WHERE folder_uuid IS NOT NULL AND folder_uuid != '';
    RAISE NOTICE 'Tasks total: %, with context_uuid: %, with folder_uuid: %',
                 task_count, ctx_migrated, fld_migrated;
END $$;

-- ============================================================================
-- Step 5: Add UUID column to task_keyword table
-- ============================================================================

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'task_keyword' AND column_name = 'keyword_uuid') THEN
        ALTER TABLE task_keyword ADD COLUMN keyword_uuid TEXT;
        RAISE NOTICE 'Added keyword_uuid column to task_keyword table';
    ELSE
        RAISE NOTICE 'keyword_uuid column already exists in task_keyword table';
    END IF;
END $$;

-- ============================================================================
-- Step 6: Populate task_keyword UUID references
-- ============================================================================

-- Migrate keyword_uuid from keyword_tid
UPDATE task_keyword tk
SET keyword_uuid = k.uuid
FROM keyword k
WHERE k.tid = tk.keyword_tid
  AND (tk.keyword_uuid IS NULL OR tk.keyword_uuid = '');

-- Report task_keyword migration
DO $$
DECLARE
    tk_count INTEGER;
    tk_migrated INTEGER;
BEGIN
    SELECT COUNT(*) INTO tk_count FROM task_keyword;
    SELECT COUNT(*) INTO tk_migrated FROM task_keyword WHERE keyword_uuid IS NOT NULL AND keyword_uuid != '';
    RAISE NOTICE 'task_keyword total: %, with keyword_uuid: %', tk_count, tk_migrated;
END $$;

COMMIT;

-- ============================================================================
-- Summary
-- ============================================================================
DO $$
BEGIN
    RAISE NOTICE '';
    RAISE NOTICE '=== UUID Migration Complete ===';
    RAISE NOTICE 'All container tables (context, folder, keyword) now have uuid columns';
    RAISE NOTICE 'All task entries now have context_uuid and folder_uuid columns';
    RAISE NOTICE 'All task_keyword entries now have keyword_uuid column';
    RAISE NOTICE '';
    RAISE NOTICE 'The tid columns are preserved for backward compatibility.';
    RAISE NOTICE 'Both tid and uuid can be used for synchronization.';
END $$;
