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
	feed(bg *background) (content, error)
}

// stringFeeder provides a string
type stringFeeder func(bg *background) (string, error)

// mapFeeder provides a map, usually it's header feeder
type mapFeeder func(bg *background) (map[string]string, error)

// feedCombiner is a feeder that combines several sub-feeders to provide http elements.
type feedCombiner struct {
	headers           mapFeeder
	url, method, body stringFeeder
}

// feed implements feeder
func (fc *feedCombiner) feed(bg *background) (content, error) {
	m := make(content)
	var err error

	if fc.url != nil {
		m[catURL], err = fc.url(bg)
		if err != nil {
			return nil, err
		}
	}

	if fc.method != nil {
		m[catMethod], err = fc.method(bg)
		if err != nil {
			return nil, err
		}
	}

	if fc.body != nil {
		m[catBody], err = fc.body(bg)
		if err != nil {
			return nil, err
		}
	}

	if fc.headers != nil {
		h, err := fc.headers(bg)
		if err != nil {
			return nil, err
		}
		for k, v := range h {
			m[category(k)] = v
		}
	}

	return m, nil
}

func _testFeed() (provider, error) {
	s, _ := makeStaticProvider("", "", "", 1)
	return makeFeedProvider(s, &feedCombiner{})
}
