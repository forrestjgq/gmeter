package meter

import (
	"errors"
	"io"
	"testing"
)

type tfeed struct {
	err error
	c   content
}

func (f *tfeed) close() {
}

func (f *tfeed) feed(_ *background) (content, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.c, nil
}

func TestDynamicProvider(t *testing.T) {
	m := &tfeed{c: make(content)}

	fprov, err := makeFeedProvider(m)
	if err != nil {
		t.Fatalf("make provider fail: %+v", err)
	}

	bg, err := makeBackground(nil, nil)
	if err != nil {
		t.Fatalf("create bg fail: %+v", err)
	}

	var prov provider
	var n next
	var url, body string
	var hdr map[string]string

	// normal without header
	m.c[catMethod] = "GET"
	m.c[catURL] = "http://127.0.0.1"

	prov, n = fprov.getProvider(bg)
	if n != nextContinue {
		t.Fatalf("create bg fail: %v", n)
	}

	if prov.getMethod(bg) != m.c[catMethod] {
		t.Fatalf("method not match")
	}

	url, n = prov.getUrl(bg)
	if n != nextContinue {
		t.Fatalf("expect continue")
	}
	if url != m.c[catURL] {
		t.Fatalf("url not match")
	}

	body, n = prov.getRequestBody(bg)
	if n != nextContinue {
		t.Fatalf("expect continue")
	}
	if len(body) != 0 {
		t.Fatalf("expect empty body")
	}

	hdr, n = prov.getHeaders(bg)
	if n != nextContinue {
		t.Fatalf("expect continue")
	}
	if len(hdr) > 0 {
		t.Fatalf("hdr not match")
	}

	// normal with header
	m.c[catMethod] = "POST"
	m.c[catURL] = "http://128.0.0.1"
	m.c[category("content-type")] = "application/json"
	m.c[catBody] = "{}"

	prov, n = fprov.getProvider(bg)
	if n != nextContinue {
		t.Fatalf("create bg fail: %v", n)
	}

	if prov.getMethod(bg) != m.c[catMethod] {
		t.Fatalf("method not match")
	}

	url, n = prov.getUrl(bg)
	if n != nextContinue {
		t.Fatalf("expect continue")
	}
	if url != m.c[catURL] {
		t.Fatalf("url not match")
	}

	body, n = prov.getRequestBody(bg)
	if n != nextContinue {
		t.Fatalf("expect continue")
	}
	if body != m.c[catBody] {
		t.Fatalf("body not match")
	}

	hdr, n = prov.getHeaders(bg)
	if n != nextContinue {
		t.Fatalf("expect continue")
	}
	if len(hdr) != 1 {
		t.Fatalf("hdr not found")
	} else if hdr["content-type"] != "application/json" {
		t.Fatalf("hdr not expected")
	}

	// provide error
	m.err = errors.New("some error")
	_, n = fprov.getProvider(bg)
	if n != nextAbortPlan {
		t.Fatalf("expect abortion")
	}

	// provide EOF
	m.err = io.EOF
	_, n = fprov.getProvider(bg)
	if n != nextFinished {
		t.Fatalf("expect finish")
	}

	m.err = nil
	delete(m.c, catMethod)
	_, n = fprov.getProvider(bg)
	if n != nextAbortPlan {
		t.Fatalf("expect abortion")
	}

	m.c[catMethod] = "POST"
	delete(m.c, catURL)
	_, n = fprov.getProvider(bg)
	if n != nextAbortPlan {
		t.Fatalf("expect abortion")
	}
}
