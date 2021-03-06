CREATE TABLE task (
        id INTEGER NOT NULL, 
        tid INTEGER NOT NULL, 
        title VARCHAR(255) NOT NULL, 
        tag VARCHAR(255), 
        folder_tid INTEGER, 
        context_tid INTEGER, 
        duetime TEXT, 
        star BOOLEAN, 
        added TEXT, 
        completed TEXT, 
        duedate TEXT, 
        note TEXT, 
        deleted BOOLEAN, 
        created TEXT, 
        modified TEXT, 
        startdate TEXT, 
        PRIMARY KEY (id), 
        FOREIGN KEY(folder_tid) REFERENCES folder (tid), 
        FOREIGN KEY(context_tid) REFERENCES context (tid), 
        UNIQUE (tid), 
        CHECK (star IN (0, 1)), 
        CHECK (deleted IN (0, 1))
);
CREATE TABLE context (
        id INTEGER NOT NULL, 
        tid INTEGER NOT NULL, 
        title VARCHAR(32) NOT NULL, 
        star BOOLEAN, 
        created TEXT, 
        deleted BOOLEAN, 
        modified TEXT, 
        PRIMARY KEY (id), 
        UNIQUE (tid), 
        UNIQUE (title), 
        CHECK (star IN (0, 1)), 
        CHECK (deleted IN (0, 1))
);
CREATE TABLE folder (
        id INTEGER NOT NULL, 
        tid INTEGER NOT NULL, 
        title VARCHAR(32) NOT NULL, 
        star BOOLEAN, 
        archived BOOLEAN, 
        created TEXT, 
        deleted BOOLEAN, 
        modified TEXT, 
        PRIMARY KEY (id), 
        UNIQUE (tid), 
        UNIQUE (title), 
        CHECK (star IN (0, 1)), 
        CHECK (deleted IN (0, 1))
);
CREATE TABLE keyword (
        id INTEGER NOT NULL, 
        name VARCHAR(255) NOT NULL,
        tid INTEGER NOT NULL,
        star BOOLEAN,
        modified TEXT,
        deleted BOOLEAN, 
        PRIMARY KEY (id), 
        UNIQUE (name)
        UNIQUE (tid), 
        CHECK (star IN (0, 1)), 
        CHECK (deleted IN (0, 1))
);
CREATE TABLE sync (
        machine VARCHAR(255) NOT NULL, 
        timestamp TEXT, 
        PRIMARY KEY (machine)
);
CREATE TABLE task_keyword (
        task_id INTEGER NOT NULL, 
        keyword_id INTEGER NOT NULL, 
        PRIMARY KEY (task_id, keyword_id), 
        FOREIGN KEY(task_id) REFERENCES task (id), 
        FOREIGN KEY(keyword_id) REFERENCES keyword (id)
);
CREATE TABLE sync_log (
        id INTEGER NOT NULL,
        title TEXT,
        modified TEXT,
        note TEXT,
        PRIMARY KEY (id)
);

