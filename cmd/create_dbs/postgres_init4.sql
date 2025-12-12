-- PostgreSQL Schema v4 with UUID Support
-- This schema is for new installations that include UUID columns from the start.
--
-- Usage:
--   psql -h your_host -U your_user -d your_db -f postgres_init4.sql

SET search_path = public;
SET check_function_bodies = false;
SET client_min_messages = warning;
SET default_tablespace = '';

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================================
-- Container Tables
-- ============================================================================

CREATE TABLE context (
    tid serial PRIMARY KEY,
    uuid TEXT UNIQUE NOT NULL DEFAULT uuid_generate_v4()::TEXT,
    title character varying(32) UNIQUE NOT NULL,
    star boolean DEFAULT FALSE,
    deleted boolean DEFAULT FALSE,
    modified timestamp without time zone DEFAULT now()
);

CREATE TABLE folder (
    tid serial PRIMARY KEY,
    uuid TEXT UNIQUE NOT NULL DEFAULT uuid_generate_v4()::TEXT,
    title character varying(32) UNIQUE NOT NULL,
    star boolean DEFAULT FALSE,
    deleted boolean DEFAULT FALSE,
    modified timestamp without time zone DEFAULT now()
);

CREATE TABLE keyword (
    tid serial PRIMARY KEY,
    uuid TEXT UNIQUE NOT NULL DEFAULT uuid_generate_v4()::TEXT,
    title character varying(32) UNIQUE NOT NULL,
    star boolean DEFAULT FALSE,
    deleted boolean DEFAULT FALSE,
    modified timestamp without time zone DEFAULT now()
);

-- ============================================================================
-- Task Table
-- ============================================================================

CREATE TABLE task (
    tid serial PRIMARY KEY,
    star boolean DEFAULT FALSE,
    title character varying(255) NOT NULL,
    folder_tid integer DEFAULT 1,
    context_tid integer DEFAULT 1,
    folder_uuid TEXT DEFAULT '00000000-0000-0000-0000-000000000002',
    context_uuid TEXT DEFAULT '00000000-0000-0000-0000-000000000001',
    note text,
    archived boolean DEFAULT FALSE,
    deleted boolean DEFAULT FALSE,
    added timestamp without time zone NOT NULL,
    modified timestamp without time zone DEFAULT now()
);

-- ============================================================================
-- Task-Keyword Junction Table
-- ============================================================================

CREATE TABLE task_keyword (
    task_tid integer NOT NULL,
    keyword_tid integer NOT NULL,
    keyword_uuid TEXT,
    PRIMARY KEY (task_tid, keyword_tid)
);

-- ============================================================================
-- Foreign Key Constraints
-- ============================================================================

ALTER TABLE task
    ADD CONSTRAINT task_context_tid_fkey FOREIGN KEY (context_tid) REFERENCES context(tid);

ALTER TABLE task
    ADD CONSTRAINT task_folder_tid_fkey FOREIGN KEY (folder_tid) REFERENCES folder(tid);

ALTER TABLE task_keyword
    ADD CONSTRAINT task_keyword_keyword_tid_fkey FOREIGN KEY (keyword_tid) REFERENCES keyword(tid);

ALTER TABLE task_keyword
    ADD CONSTRAINT task_keyword_task_tid_fkey FOREIGN KEY (task_tid) REFERENCES task(tid);

-- ============================================================================
-- Default Containers (Required)
-- ============================================================================

-- Insert default "none" context with well-known UUID
INSERT INTO context (tid, uuid, title, star, deleted, modified)
VALUES (1, '00000000-0000-0000-0000-000000000001', 'none', FALSE, FALSE, now());

-- Insert default "none" folder with well-known UUID
INSERT INTO folder (tid, uuid, title, star, deleted, modified)
VALUES (1, '00000000-0000-0000-0000-000000000002', 'none', FALSE, FALSE, now());

-- Reset sequences to start after default entries
SELECT setval('context_tid_seq', 1, true);
SELECT setval('folder_tid_seq', 1, true);
