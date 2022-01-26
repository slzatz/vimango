--SELECT pg_catalog.set_config('search_path', '', false);
SET search_path = public;
SET check_function_bodies = false;
SET client_min_messages = warning;
SET default_tablespace = '';

CREATE TABLE task (
    --id integer PRIMARY KEY NOT NULL,
    --tid serial UNIQUE NOT NULL,
    tid serial PRIMARY KEY,
    star boolean,
    title character varying(255) NOT NULL,
    folder_tid integer,
    context_tid integer,
    note text,
    deleted boolean,
    added date,
    completed date,
    created timestamp without time zone,
    modified timestamp without time zone
);

CREATE TABLE context (
    tid serial PRIMARY KEY,
    title character varying(32) UNIQUE NOT NULL,
    star boolean,
    deleted boolean,
    created timestamp without time zone,
    modified timestamp without time zone
);

CREATE TABLE folder (
    tid serial PRIMARY KEY,
    title character varying(32) UNIQUE NOT NULL,
    star boolean,
    deleted boolean,
    created timestamp without time zone,
    modified timestamp without time zone
);

CREATE TABLE keyword (
    tid serial PRIMARY KEY,
    title character varying(32) UNIQUE NOT NULL,
    star boolean,
    deleted boolean,
    created timestamp without time zone,
    modified timestamp without time zone
);

CREATE TABLE task_keyword (
    task_tid integer NOT NULL,
    keyword_tid integer NOT NULL,
    PRIMARY KEY (task_tid, keyword_tid)
);

--CREATE TABLE sync (
--    id serial PRIMARY KEY,
--    machine character varying(32) UNIQUE NOT NULL,
--    "timestamp" timestamp without time zone,
--    unix_timestamp integer
--);

ALTER TABLE task
    ADD CONSTRAINT task_context_tid_fkey FOREIGN KEY (context_tid) REFERENCES context(tid);

ALTER TABLE task
    ADD CONSTRAINT task_folder_tid_fkey FOREIGN KEY (folder_tid) REFERENCES folder(tid);

-- ALTER TABLE task_keyword
--    ADD CONSTRAINT task_keyword_pkey PRIMARY KEY (task_tid, keyword_tid);

ALTER TABLE task_keyword
    ADD CONSTRAINT task_keyword_keyword_tid_fkey FOREIGN KEY (keyword_tid) REFERENCES keyword(tid);

ALTER TABLE task_keyword
    ADD CONSTRAINT task_keyword_task_tid_fkey FOREIGN KEY (task_tid) REFERENCES task(tid);



