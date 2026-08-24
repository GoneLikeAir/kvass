package metricdrop

const (
	DefaultDir      = "/etc/prometheus/metric-drop"
	FileGeneration  = "generation"
	FileContentHash = "content-hash"
	FileEnabled     = "enabled"
	FileNamesGZ     = "names.gz"
	MaxNamesGZBytes = 800 * 1024
	MaxObjectBytes  = 1024 * 1024
)
