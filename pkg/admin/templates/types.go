// CLAUDE:SUMMARY View model types for admin panel templates.
package templates

// DashboardData holds data for the admin dashboard.
type DashboardData struct {
	DictCount    int
	SourceCount  int
	ImportRuns   int
	TotalEntries int
	RecentRuns   []ImportRunView
}

// DictView is a view model for a dictionary in the admin panel.
type DictView struct {
	ID           string
	Type         string
	Jurisdiction string
	EntityType   string
	Domain       string
	Status       string
	EntryCount   int
	CreatedAt    string
	UpdatedAt    string
}

// SourceView is a view model for a source in the admin panel.
type SourceView struct {
	ID              string
	DictID          string
	AdapterID       string
	Description     string
	SourceURL       string
	License         string
	Format          string
	UpdateFrequency string
	LastStatus      string
	LastImport      string
	LastImportCount int
}

// ImportRunView is a view model for an import run.
type ImportRunView struct {
	ID         string
	SourceID   string
	DictID     string
	StartedAt  string
	Status     string
	EntryCount int
	DurationMs int64
	Error      string
}

// AuditEntryView is a view model for an audit log entry.
type AuditEntryView struct {
	EntryID    string
	Timestamp  string
	Action     string
	Parameters string
	Result     string
	Status     string
}
