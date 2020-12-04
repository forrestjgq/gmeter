package meter

type category string

// content that does not take a key defined as explicit category will be
// considered as http header
type content map[category]string

const (
	catMethod category = "_mtd_"
	catURL    category = "_url_"
	catBody   category = "_bdy_"
	catEnd    category = "_eof_"
)

// feeder provide all possible elements http request requires
type feeder interface {
	feed(seq int64) content
}

// stringFeeder provides a string
type stringFeeder func(seq int64) string

// mapFeeder provides a map, usually it's header feeder
type mapFeeder func(seq int64) map[string]string

// feedCombiner is a feeder that combines several sub-feeders to provide http elements.
type feedCombiner struct {
	// callback to decide ending
	end               func(seq int64) bool
	headers           mapFeeder
	url, method, body stringFeeder
}

// feed implements feeder
func (fc *feedCombiner) feed(seq int64) map[category]string {
	if fc.end == nil {
		panic("end decider should not be nil")
	}

	m := make(map[category]string)
	if fc.end(seq) {
		m[catEnd] = "true"
		return m
	}

	if fc.url != nil {
		m[catURL] = fc.url(seq)
	}

	if fc.method != nil {
		m[catMethod] = fc.method(seq)
	}

	if fc.body != nil {
		m[catBody] = fc.body(seq)
	}

	if fc.headers != nil {
		h := fc.headers(seq)
		for k, v := range h {
			m[category(k)] = v
		}
	}

	return m
}
