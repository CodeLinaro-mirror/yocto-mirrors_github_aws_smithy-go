package http

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/aws/smithy-go/middleware"
)

var nopBuildHandler = middleware.BuildHandlerFunc(func(ctx context.Context, input middleware.BuildInput) (
	out middleware.BuildOutput, metadata middleware.Metadata, err error) {
	return out, metadata, nil
})

type errorSecondSeekableReader struct {
	err   error
	count int
}

func (r *errorSecondSeekableReader) Read(p []byte) (int, error) {
	return 0, io.EOF
}
func (r *errorSecondSeekableReader) Seek(offset int64, whence int) (int64, error) {
	r.count++
	if r.count == 2 {
		return 0, r.err
	}
	return 0, nil
}

func TestValidateContentLengthHeader(t *testing.T) {
	cases := map[string]struct {
		contentLength int64
		expectError   string
	}{
		"success": {
			contentLength: 10,
		},
		"length set to 0": {
			contentLength: 0,
		},
		"content-length unset": {
			contentLength: -1,
			expectError:   "content length for payload is required and must be at least 0",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var err error
			req := NewStackRequest().(*Request)
			req.ContentLength = c.contentLength

			var m validateContentLength
			_, _, err = m.HandleBuild(context.Background(),
				middleware.BuildInput{Request: req},
				nopBuildHandler,
			)

			if len(c.expectError) != 0 {
				if err == nil {
					t.Fatalf("expect error, got none")
				}
				if e, a := c.expectError, err.Error(); !strings.Contains(a, e) {
					t.Fatalf("expect error to contain %q, got %v", e, a)
				}
			} else if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
		})
	}
}
