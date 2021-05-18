package meter

import (
	"encoding/json"

	"github.com/forrestjgq/gmeter/internal/meter"
)

type JSONC interface {
	Compare(message json.RawMessage) error
	Set(key, value string)
	Get(key string) string
	Reset()
}

func MakeJSONC(message json.RawMessage) (JSONC, error) {
	return meter.MakeJsonComparator(message)
}
