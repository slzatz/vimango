--
-- PostgreSQL database dump
--

-- Dumped from database version 13.2
-- Dumped by pg_dump version 13.3

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: task; Type: TABLE; Schema: public; Owner: slzatz
--

CREATE TABLE public.task (
    id integer NOT NULL,
    priority integer,
    title character varying(255),
    tag character varying(64),
    folder_id integer,
    context_id integer,
    duetime timestamp without time zone,
    star boolean,
    added date,
    completed date,
    duedate date,
    note text,
    deleted boolean,
    created timestamp without time zone,
    modified timestamp without time zone,
    startdate timestamp without time zone
);


ALTER TABLE public.task OWNER TO slzatz;

--
-- Name: task_id_seq; Type: SEQUENCE; Schema: public; Owner: slzatz
--

CREATE SEQUENCE public.task_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.task_id_seq OWNER TO slzatz;

--
-- Name: context; Type: TABLE; Schema: public; Owner: slzatz
--

CREATE TABLE public.context (
    id integer NOT NULL,
    title character varying(32) NOT NULL,
    star boolean,
    created timestamp without time zone,
    deleted boolean,
    modified timestamp without time zone
);


ALTER TABLE public.context OWNER TO slzatz;

--
-- Name: context_id_seq; Type: SEQUENCE; Schema: public; Owner: slzatz
--

CREATE SEQUENCE public.context_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.context_id_seq OWNER TO slzatz;

--
-- Name: context_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: slzatz
--

ALTER SEQUENCE public.context_id_seq OWNED BY public.context.id;

--
-- Name: context id; Type: DEFAULT; Schema: public; Owner: slzatz
--

ALTER TABLE ONLY public.context ALTER COLUMN id SET DEFAULT nextval('public.context_id_seq'::regclass);

--
-- Name: context context_pkey; Type: CONSTRAINT; Schema: public; Owner: slzatz
--

ALTER TABLE ONLY public.context
    ADD CONSTRAINT context_pkey PRIMARY KEY (id);

--
-- Name: context context_title_key; Type: CONSTRAINT; Schema: public; Owner: slzatz
--

ALTER TABLE ONLY public.context
    ADD CONSTRAINT context_title_key UNIQUE (title);

--
-- Name: folder; Type: TABLE; Schema: public; Owner: slzatz
--

CREATE TABLE public.folder (
    id integer NOT NULL,
    title character varying(32) NOT NULL,
    star boolean,
    archived boolean,
    created timestamp without time zone,
    deleted boolean,
    modified timestamp without time zone
);


ALTER TABLE public.folder OWNER TO slzatz;

--
-- Name: folder_id_seq; Type: SEQUENCE; Schema: public; Owner: slzatz
--

CREATE SEQUENCE public.folder_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.folder_id_seq OWNER TO slzatz;


--
-- Name: folder_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: slzatz
--

ALTER SEQUENCE public.folder_id_seq OWNED BY public.folder.id;

--
-- Name: folder id; Type: DEFAULT; Schema: public; Owner: slzatz
--

ALTER TABLE ONLY public.folder ALTER COLUMN id SET DEFAULT nextval('public.folder_id_seq'::regclass);

--
-- Name: folder folder_pkey; Type: CONSTRAINT; Schema: public; Owner: slzatz
--

ALTER TABLE ONLY public.folder
    ADD CONSTRAINT folder_pkey PRIMARY KEY (id);

--
-- Name: folder folder_title_key; Type: CONSTRAINT; Schema: public; Owner: slzatz
--

ALTER TABLE ONLY public.folder
    ADD CONSTRAINT folder_title_key UNIQUE (title);

--
-- Name: keyword; Type: TABLE; Schema: public; Owner: slzatz
--

CREATE TABLE public.keyword (
    id integer NOT NULL,
    name character varying(25) NOT NULL,
    star boolean,
    modified timestamp without time zone,
    deleted boolean
);


ALTER TABLE public.keyword OWNER TO slzatz;

--
-- Name: keyword_id_seq; Type: SEQUENCE; Schema: public; Owner: slzatz
--

CREATE SEQUENCE public.keyword_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.keyword_id_seq OWNER TO slzatz;

--
-- Name: task_keyword; Type: TABLE; Schema: public; Owner: slzatz
--

CREATE TABLE public.task_keyword (
    task_id integer NOT NULL,
    keyword_id integer NOT NULL
);


ALTER TABLE public.task_keyword OWNER TO slzatz;

--
-- Name: task_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: slzatz
--

ALTER SEQUENCE public.task_id_seq OWNED BY public.task.id;


--
-- Name: task id; Type: DEFAULT; Schema: public; Owner: slzatz
--

ALTER TABLE ONLY public.task ALTER COLUMN id SET DEFAULT nextval('public.task_id_seq'::regclass);


--
-- Name: task task_pkey; Type: CONSTRAINT; Schema: public; Owner: slzatz
--

ALTER TABLE ONLY public.task
    ADD CONSTRAINT task_pkey PRIMARY KEY (id);


--
-- Name: task task_context_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: slzatz
--

ALTER TABLE ONLY public.task
    ADD CONSTRAINT task_context_id_fkey FOREIGN KEY (context_id) REFERENCES public.context(id);


--
-- Name: task task_folder_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: slzatz
--

ALTER TABLE ONLY public.task
    ADD CONSTRAINT task_folder_id_fkey FOREIGN KEY (folder_id) REFERENCES public.folder(id);

--
-- Name: keyword_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: slzatz
--

ALTER SEQUENCE public.keyword_id_seq OWNED BY public.keyword.id;


--
-- Name: keyword id; Type: DEFAULT; Schema: public; Owner: slzatz
--

ALTER TABLE ONLY public.keyword ALTER COLUMN id SET DEFAULT nextval('public.keyword_id_seq'::regclass);


--
-- Name: keyword keyword_name_key; Type: CONSTRAINT; Schema: public; Owner: slzatz
--

ALTER TABLE ONLY public.keyword
    ADD CONSTRAINT keyword_name_key UNIQUE (name);


--
-- Name: keyword keyword_pkey; Type: CONSTRAINT; Schema: public; Owner: slzatz
--

ALTER TABLE ONLY public.keyword
    ADD CONSTRAINT keyword_pkey PRIMARY KEY (id);


--
-- Name: task_keyword task_keyword_pkey; Type: CONSTRAINT; Schema: public; Owner: slzatz
--

ALTER TABLE ONLY public.task_keyword
    ADD CONSTRAINT task_keyword_pkey PRIMARY KEY (task_id, keyword_id);


--
-- Name: task_keyword task_keyword_keyword_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: slzatz
--

ALTER TABLE ONLY public.task_keyword
    ADD CONSTRAINT task_keyword_keyword_id_fkey FOREIGN KEY (keyword_id) REFERENCES public.keyword(id);


--
-- Name: task_keyword task_keyword_task_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: slzatz
--

ALTER TABLE ONLY public.task_keyword
    ADD CONSTRAINT task_keyword_task_id_fkey FOREIGN KEY (task_id) REFERENCES public.task(id);


--
-- Name: sync; Type: TABLE; Schema: public; Owner: slzatz
--

CREATE TABLE public.sync (
    machine character varying(20) NOT NULL,
    "timestamp" timestamp without time zone,
    unix_timestamp integer
);


ALTER TABLE public.sync OWNER TO slzatz;

--
-- Name: sync sync_pkey; Type: CONSTRAINT; Schema: public; Owner: slzatz
--

ALTER TABLE ONLY public.sync
    ADD CONSTRAINT sync_pkey PRIMARY KEY (machine);


--
-- PostgreSQL database dump complete
--

