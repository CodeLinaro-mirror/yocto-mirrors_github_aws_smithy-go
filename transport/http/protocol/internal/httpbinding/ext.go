package httpbinding

import (
	"net/http"
	"strings"

	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/traits"
)

// bindKind is the HTTP binding a member carries, resolved once from its traits.
type bindKind uint8

const (
	bindNone bindKind = iota
	bindKindHeader
	bindKindPrefixHeaders
	bindKindLabel
	bindKindQuery
	bindKindQueryParams
	bindKindPayload
	bindKindResponseCode
)

// memberBinding is one HTTP-bound member of a top-level input/output struct,
// with its name already resolved (canonicalized, for headers).
type memberBinding struct {
	schema *smithy.Schema
	kind   bindKind
	name   string
}

// bindingExt caches per-schema facts about a shape's HTTP bindings that would
// otherwise be recomputed by walking its members on every request.
//
// A schema is either a member or a shape, so the two halves below are disjoint
// in practice.
type bindingExt struct {
	// member facts
	kind bindKind
	name string // canonical header name, query param name, prefix, or label member name

	// shape facts
	hasBlobPayload bool
	hasPayload     bool
	hasBodyMembers bool
	bindings       []memberBinding

	// Response-direction dispatch. A response carries a handful of the headers a
	// shape declares (S3's GetObjectOutput declares 35), so the deserializer
	// walks the response's actual headers and looks the member up here, rather
	// than asking "is it set?" once per declared member.
	//
	// Keyed by canonical header name, which is the form net/http stores.
	scalarHeaders map[string]*smithy.Schema

	// Bindings that can't be resolved by a single header-name hit, so they stay
	// schema-driven. All of these are empty or single-element for AWS shapes.
	listHeaders   []memberBinding // need every value, not a presence check
	prefixHeaders []memberBinding
	responseCode  *smithy.Schema
	payload       *smithy.Schema
}

func getExt(s *smithy.Schema) *bindingExt {
	return smithy.SchemaExtension(s, smithy.ExtHTTPBinding, buildExt)
}

func buildExt(s *smithy.Schema) *bindingExt {
	e := &bindingExt{}
	e.kind, e.name = resolveKind(s)

	members := s.Members()
	if len(members) == 0 {
		return e
	}

	e.hasBlobPayload = hasBlobPayload(s)
	for _, m := range members {
		kind, name := resolveKind(m)
		if kind == bindNone {
			e.hasBodyMembers = true
			continue
		}

		mb := memberBinding{schema: m, kind: kind, name: name}
		e.bindings = append(e.bindings, mb)

		switch kind {
		case bindKindHeader:
			if m.Type() == smithy.ShapeTypeList {
				e.listHeaders = append(e.listHeaders, mb)
				break
			}
			if e.scalarHeaders == nil {
				e.scalarHeaders = make(map[string]*smithy.Schema, len(members))
			}
			e.scalarHeaders[name] = m
		case bindKindPrefixHeaders:
			e.prefixHeaders = append(e.prefixHeaders, mb)
		case bindKindResponseCode:
			e.responseCode = m
		case bindKindPayload:
			e.hasPayload = true
			e.payload = m
		}
	}
	return e
}

// resolveKind reads a member's binding traits. Called once per schema.
func resolveKind(s *smithy.Schema) (bindKind, string) {
	if s == nil {
		return bindNone, ""
	}
	if h, ok := smithy.SchemaTrait[*traits.HTTPHeader](s); ok {
		return bindKindHeader, http.CanonicalHeaderKey(h.Name)
	}
	if p, ok := smithy.SchemaTrait[*traits.HTTPPrefixHeaders](s); ok {
		return bindKindPrefixHeaders, http.CanonicalHeaderKey(p.Prefix)
	}
	if _, ok := smithy.SchemaTrait[*traits.HTTPPayload](s); ok {
		return bindKindPayload, ""
	}
	if _, ok := smithy.SchemaTrait[*traits.HTTPResponseCode](s); ok {
		return bindKindResponseCode, ""
	}
	if _, ok := smithy.SchemaTrait[*traits.HTTPLabel](s); ok {
		return bindKindLabel, s.MemberName()
	}
	if q, ok := smithy.SchemaTrait[*traits.HTTPQuery](s); ok {
		return bindKindQuery, q.Name
	}
	if _, ok := smithy.SchemaTrait[*traits.HTTPQueryParams](s); ok {
		return bindKindQueryParams, ""
	}
	return bindNone, ""
}

func hasBlobPayload(output *smithy.Schema) bool {
	for _, member := range output.Members() {
		if _, ok := smithy.SchemaTrait[*traits.HTTPPayload](member); !ok {
			continue
		}
		if member.Type() == smithy.ShapeTypeBlob {
			return true
		}
	}
	return false
}

// prefixMatches reports whether name is under the (already canonical) prefix.
func prefixMatches(name, canonPrefix string) bool {
	return len(name) > len(canonPrefix) && strings.EqualFold(name[:len(canonPrefix)], canonPrefix)
}
