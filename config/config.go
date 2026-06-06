package config

var (
	Host       string
	Port       int
	KeysLimit  int
	AOFFile    string
	AOFEnabled bool
	ServerMode string // "auto", "sync", or "async"
)
