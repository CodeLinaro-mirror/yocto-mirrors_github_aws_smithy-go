package serde

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// stands in for the body codegen emits for the serde benchmarks: satisfaction is
// structural, so PayloadBytes is the whole contract
type inMemoryBody struct {
	*bytes.Reader
	payload []byte
}

func (b *inMemoryBody) PayloadBytes() []byte { return b.payload }
func (*inMemoryBody) Close() error           { return nil }

func TestInMemoryPayloadHit(t *testing.T) {
	want := []byte(`{"foo":"bar"}`)
	var body io.ReadCloser = &inMemoryBody{Reader: bytes.NewReader(want), payload: want}

	got, ok := InMemoryPayload(body)
	if !ok {
		t.Fatalf("%T not recognized as an in-memory body", body)
	}
	if &got[0] != &want[0] {
		t.Error("payload was copied")
	}

	// reading it dry must not change the answer: the same response is
	// deserialized more than once
	if _, err := io.ReadAll(body); err != nil {
		t.Fatal(err)
	}
	if got, _ := InMemoryPayload(body); !bytes.Equal(got, want) {
		t.Errorf("after read: got %q, want %q", got, want)
	}
}

func TestInMemoryPayloadMiss(t *testing.T) {
	// what a real response body looks like: a stream with no way to expose its
	// contents, so protocols must fall back to buffering
	for _, r := range []io.Reader{
		bytes.NewReader([]byte("body")),
		strings.NewReader("body"),
		io.NopCloser(bytes.NewBufferString("body")),
	} {
		if _, ok := InMemoryPayload(r); ok {
			t.Errorf("%T reported an in-memory payload", r)
		}
	}
}
