package api

import (
	"github.com/forrestjgq/gmeter/config"
	"github.com/forrestjgq/gmeter/internal/meter"
)

// Run gmeter from programmatic api instead of command line
func Run(options *config.GOptions) error {
	return meter.Execute(options)
}
