# fts5_vimango.db cleanup

One-off cleanup for the full-text search index, `fts5_vimango.db`. Run it
against **every local copy** of `fts5_vimango.db`, on each machine that runs
vimango or vimango_hybrid. Postgres sync never touches the search index, so
each copy has to be cleaned on its own machine.

Before commit `27d8b0e`, sync deleted notes without removing their FTS rows,
so orphan rows built up. It also left rows with `tid = 0` from notes that were
indexed before they had a tid. Search already skips both kinds, so this is
tidying and nothing is broken.

## Steps

1. Install the build that contains `27d8b0e` or later on that machine: both
   `vimango` and the `vimango-sync` bundled with hybrid. Without it, new
   orphans keep appearing.
2. `cd` to the directory that holds that machine's databases. Use the
   `sqlite3` paths in its `config.json`.
3. Make sure no sync is running, then run:

```bash
sqlite3 fts5_vimango.db "ATTACH 'vimango.db' AS m; DELETE FROM fts WHERE tid NOT IN (SELECT tid FROM m.task WHERE tid IS NOT NULL);"
```

**Check that `vimango.db` exists at that path first.** If it doesn't,
`ATTACH` silently creates an empty database. The `DELETE` then removes every
row in the index.

## Checking the result

Optional. The first number should equal the second:

```bash
sqlite3 -readonly fts5_vimango.db "ATTACH 'vimango.db' AS m; SELECT (SELECT count(*) FROM fts), (SELECT count(*) FROM m.task WHERE tid IS NOT NULL);"
```
