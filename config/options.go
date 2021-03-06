package config

// GOptions is used to package gmeter parameters. Parameters being replaces is commented after each members.
type GOptions struct {
	Vars          map[string]string // "-e"
	Template      string            // "-t"
	Configs       []string          // "-config" or configuration list
	HTTPServerCfg string            // "-httpsrv"
	FileServer    string            // "-fs"
	Call          string            // "-call"
	Final         string            // "-f"
	GoMarkPort    int               // "-gm"
	Plugins       string
}
