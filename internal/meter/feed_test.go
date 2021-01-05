package meter

import "testing"

func TestDynamicFeed(t *testing.T) {
	def := map[string]string{
		string(catMethod): "POST",
		string(catURL):    "http://127.0.0.1",
		string(catBody):   `{"seq": 0}`,
	}
	f, err := makeDynamicFeeder(
		def,
		1,
		[]string{
			"",
		},
	)
	if err != nil {
		t.Fatalf(err.Error())
	}

	bg, err := createDefaultBackground()
	if err != nil {
		t.Fatalf(err.Error())
	}

	c, err := f.feed(bg)
	if err != nil {
		t.Fatalf(err.Error())
	}

	fc := func(cat category) {
		if c[cat] != def[string(cat)] {
			t.Fatalf("%s != %s", c[cat], def[string(cat)])
		}
	}

	fc(catMethod)
	fc(catURL)
	fc(catBody)

	_, err = f.feed(bg)
	if err == nil {
		t.Fatalf("expect eof")
	}
	if !isEof(err) {
		t.Fatalf(err.Error())
	}
}
func TestDynamicFeedDefaultMethod(t *testing.T) {
	def := map[string]string{
		string(catURL):  "http://127.0.0.1",
		string(catBody): `{"seq": 0}`,
	}
	f, err := makeDynamicFeeder(
		def,
		1,
		[]string{
			"",
		},
	)
	if err != nil {
		t.Fatalf(err.Error())
	}

	bg, err := createDefaultBackground()
	if err != nil {
		t.Fatalf(err.Error())
	}

	c, err := f.feed(bg)
	if err != nil {
		t.Fatalf(err.Error())
	}

	fc := func(cat category) {
		if c[cat] != def[string(cat)] {
			t.Fatalf("%s != %s", c[cat], def[string(cat)])
		}
	}

	if c[catMethod] != "GET" {
		t.Fatalf("%s != %s", c[catMethod], "GET")
	}
	fc(catURL)
	fc(catBody)

	_, err = f.feed(bg)
	if err == nil {
		t.Fatalf("expect eof")
	}
	if !isEof(err) {
		t.Fatalf(err.Error())
	}
}
func TestDynamicFeedCount0(t *testing.T) {
	def := map[string]string{
		string(catURL):  "http://127.0.0.1",
		string(catBody): `{"seq": 0}`,
	}
	f, err := makeDynamicFeeder(
		def,
		0,
		[]string{
			"",
		},
	)
	if err != nil {
		t.Fatalf(err.Error())
	}

	bg, err := createDefaultBackground()
	if err != nil {
		t.Fatalf(err.Error())
	}

	c, err := f.feed(bg)
	if err != nil {
		t.Fatalf(err.Error())
	}

	fc := func(cat category) {
		if c[cat] != def[string(cat)] {
			t.Fatalf("%s != %s", c[cat], def[string(cat)])
		}
	}

	if c[catMethod] != "GET" {
		t.Fatalf("%s != %s", c[catMethod], "GET")
	}
	fc(catURL)
	fc(catBody)

	_, err = f.feed(bg)
	if err != nil {
		t.Fatalf("non error")
	}
}
func TestDynamicFeedIterable(t *testing.T) {
	def := map[string]string{
		string(catURL):  "http://127.0.0.1",
		string(catBody): `{"seq": 0}`,
	}
	f, err := makeDynamicFeeder(
		def,
		0,
		[]string{
			"`list ./feed_test.go`",
		},
	)
	if err != nil {
		t.Fatalf(err.Error())
	}

	bg, err := createDefaultBackground()
	if err != nil {
		t.Fatalf(err.Error())
	}

	for {
		_, err := f.feed(bg)
		if err != nil {
			if isEof(err) {
				break
			}
			t.Fatalf(err.Error())
		}
	}

	_, err = f.feed(bg)
	if err == nil {
		t.Fatalf("non error")
	} else if !isEof(err) {
		t.Fatalf("expect eof")
	}

}
func TestDynamicFeedIterableURL(t *testing.T) {
	def := map[string]string{
		string(catURL):  "`list ./feed_test.go`",
		string(catBody): `{"seq": 0}`,
	}
	f, err := makeDynamicFeeder(
		def,
		0,
		nil,
	)
	if err != nil {
		t.Fatalf(err.Error())
	}

	bg, err := createDefaultBackground()
	if err != nil {
		t.Fatalf(err.Error())
	}

	for {
		_, err := f.feed(bg)
		if err != nil {
			if isEof(err) {
				break
			}
			t.Fatalf(err.Error())
		}
	}

	_, err = f.feed(bg)
	if err == nil {
		t.Fatalf("non error")
	} else if !isEof(err) {
		t.Fatalf("expect eof")
	}

}
