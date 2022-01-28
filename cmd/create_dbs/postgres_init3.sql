--SELECT pg_catalog.set_config('search_path', '', false);
SET search_path = public;
SET check_function_bodies = false;
SET client_min_messages = warning;
SET default_tablespace = '';

CREATE TABLE task (
    tid serial PRIMARY KEY,
    star boolean DEFAULT FALSE,
    title character varying(255) NOT NULL,
    folder_tid integer DEFAULT 1,
    context_tid integer DEFAULT 1,
    note text,
    archived boolean DEFAULT FALSE,
    deleted boolean DEFAULT FALSE,
    added date NOT NULL,
    modified timestamp without time zone DEFAULT now()
);

CREATE TABLE context (
    tid serial PRIMARY KEY,
    title character varying(32) UNIQUE NOT NULL,
    star boolean DEFAULT FALSE,
    deleted boolean DEFAULT FALSE,
    modified timestamp without time zone DEFAULT now()
);

CREATE TABLE folder (
    tid serial PRIMARY KEY,
    title character varying(32) UNIQUE NOT NULL,
    star boolean DEFAULT FALSE,
    deleted boolean DEFAULT FALSE,
    modified timestamp without time zone DEFAULT now()
);

CREATE TABLE keyword (
    tid serial PRIMARY KEY,
    title character varying(32) UNIQUE NOT NULL,
    star boolean DEFAULT FALSE,
    deleted boolean DEFAULT FALSE,
    modified timestamp without time zone DEFAULT now()
);

CREATE TABLE task_keyword (
    task_tid integer NOT NULL,
    keyword_tid integer NOT NULL,
    PRIMARY KEY (task_tid, keyword_tid)
);

ALTER TABLE task
    ADD CONSTRAINT task_context_tid_fkey FOREIGN KEY (context_tid) REFERENCES context(tid);

ALTER TABLE task
    ADD CONSTRAINT task_folder_tid_fkey FOREIGN KEY (folder_tid) REFERENCES folder(tid);

ALTER TABLE task_keyword
    ADD CONSTRAINT task_keyword_keyword_tid_fkey FOREIGN KEY (keyword_tid) REFERENCES keyword(tid);

ALTER TABLE task_keyword
    ADD CONSTRAINT task_keyword_task_tid_fkey FOREIGN KEY (task_tid) REFERENCES task(tid);



