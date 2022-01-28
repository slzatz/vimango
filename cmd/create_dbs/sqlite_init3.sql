CREATE TABLE task (
        id INTEGER NOT NULL, 
        tid INTEGER, 
        star BOOLEAN DEFAULT FALSE, 
        title TEXT NOT NULL, 
        folder_tid INTEGER DEFAULT 1, 
        context_tid INTEGER DEFAULT 1, 
        note TEXT, 
        archived BOOLEAN DEFAULT FALSE, 
        deleted BOOLEAN DEFAULT FALSE, 
        added TEXT NOT NULL, 
        modified TEXT DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (id), 
        FOREIGN KEY(folder_tid) REFERENCES folder (tid), 
        FOREIGN KEY(context_tid) REFERENCES context (tid), 
        UNIQUE (tid), 
        CHECK (star IN (0, 1)), 
        CHECK (archived IN (0, 1)), 
        CHECK (deleted IN (0, 1))
);
CREATE TABLE context (
        id INTEGER NOT NULL, 
        tid INTEGER, 
        title TEXT NOT NULL, 
        star BOOLEAN DEFAULT FALSE, 
        deleted BOOLEAN DEFAULT FALSE, 
        modified TEXT DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (id), 
        UNIQUE (tid), 
        UNIQUE (title), 
        CHECK (star IN (0, 1)), 
        CHECK (deleted IN (0, 1))
);
CREATE TABLE folder (
        id INTEGER NOT NULL, 
        tid INTEGER, 
        title TEXT NOT NULL, 
        star BOOLEAN DEFAULT FALSE, 
        deleted BOOLEAN DEFAULT FALSE, 
        modified TEXT DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (id), 
        UNIQUE (tid), 
        UNIQUE (title), 
        CHECK (star IN (0, 1)), 
        CHECK (deleted IN (0, 1))
);
CREATE TABLE keyword (
        id INTEGER NOT NULL, 
        tid INTEGER, 
        title TEXT NOT NULL, 
        star BOOLEAN DEFAULT FALSE, 
        deleted BOOLEAN DEFAULT FALSE, 
        modified TEXT DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (id), 
        UNIQUE (tid), 
        UNIQUE (title), 
        CHECK (star IN (0, 1)), 
        CHECK (deleted IN (0, 1))
);
CREATE TABLE sync (
        id INTEGER NOT NULL, 
        machine TEXT NOT NULL, 
        timestamp TEXT DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (id),
        UNIQUE (machine) 
);
CREATE TABLE task_keyword (
        task_tid INTEGER NOT NULL, 
        keyword_tid INTEGER NOT NULL, 
        PRIMARY KEY (task_tid, keyword_tid), 
        FOREIGN KEY(task_tid) REFERENCES task (tid), 
        FOREIGN KEY(keyword_tid) REFERENCES keyword (tid)
);
CREATE TABLE sync_log (
        id INTEGER NOT NULL,
        title TEXT,
        modified TEXT,
        note TEXT,
        PRIMARY KEY (id)
);

