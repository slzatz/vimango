package main

// (*App).Synchronize is a thin wrapper that delegates the actual work to
// the internal/sync package. The same package is consumed by the
// vimango-sync CLI under cmd/vimango-sync/.

import (
	"github.com/slzatz/vimango/internal/sync"
)

// Synchronize synchronizes data between local and remote databases.
// reportOnly: if true, only reports changes without applying them.
func (a *App) Synchronize(reportOnly bool) string {
	if a.Database.PG == nil {
		return "### Remote sync not available\n\nPostgreSQL is not configured. To enable sync, edit config.json and add your PostgreSQL connection details in the \"postgres\" section."
	}

	if a.SyncInProcess {
		return "Synchronization already in process"
	}

	a.SyncInProcess = true
	defer func() { a.SyncInProcess = false }()

	syncer := sync.New(
		a.Database.MainDB,
		a.Database.FtsDB,
		a.Database.PG,
		a.Config.Postgres.Host,
		a.Config.Postgres.DB,
	)
	log, _ := syncer.Synchronize(reportOnly)
	return log
}
