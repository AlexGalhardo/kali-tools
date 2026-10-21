package app

// Application is the shared composition-root value. H04 replaces the
// transitional Service field with the application facade.
type Application struct {
	Service *Service
}

// Close releases the application and is idempotent.
func (a *Application) Close() error { return a.Service.Close() }
