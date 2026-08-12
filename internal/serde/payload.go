package serde

import "io"

// payloadBytesReader is a response body whose contents are already entirely in
// memory and which can hand them over without a copy.
//
// This is matched structurally and is deliberately not published: net/http never
// gives out a body like this. http.body reads off the connection's bufio.Reader,
// which defaults to 4KiB, so for any response larger than that the full payload
// is not in memory at once and there is nothing to point at -- and even when it
// would fit, the type is unexported and anything that wraps Body (checksum
// validation, decompression) hides it anyway.
//
// What does satisfy it is a caller that supplies the body itself. Today that
// means the generated serde benchmarks, whose serdBenchmarkBody exists so that
// buffering the body -- transport work, not serde -- stays out of the measured
// window, and so one response can be deserialized on every iteration. The method
// name is therefore a contract with smithy-go's protocol test generator; see
// HttpProtocolTestGenerator.generateSerdBenchmarkHelpers.
type payloadBytesReader interface {
	io.Reader

	// PayloadBytes returns the reader's full contents without copying them, and
	// without treating the call as consuming them.
	PayloadBytes() []byte
}

// InMemoryPayload returns the bytes behind r, without copying, if r is a body
// that can expose them.
//
// A protocol that gets a hit here can skip buffering the body entirely: the
// bytes are already in memory, they outlive the deserialization, and the reader
// is left untouched so it can be handed over again.
//
// The returned slice is not copied, so a blob @httpPayload in the deserialized
// output aliases it.
func InMemoryPayload(r io.Reader) ([]byte, bool) {
	pr, ok := r.(payloadBytesReader)
	if !ok {
		return nil, false
	}
	return pr.PayloadBytes(), true
}
