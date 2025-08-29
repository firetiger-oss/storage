package uri

import (
	"iter"
	"strings"
)

// Clean normalizes the path by removing redundant slashes.
//
// This function differs from the standard library's path.Clean because it does
// not remove "." and ".." elements.
func Clean(path string) string {
	return join(func(yield func(string) bool) {
		hasTrailingSlash := strings.HasSuffix(path, "/")
		for elem := range walk(path) {
			if !yield(elem) {
				return
			}
		}
		if hasTrailingSlash {
			yield("")
		}
	})
}

// Split splits a URI into its components: scheme, location, and path.
//
// The path is always cleaned and exposed as relative to the location, even for
// file URIs where the path is represented with a leading slash.
//
// For file:// URIs, the location is always empty and the path contains the
// full path after the scheme with the leading slash trimmed.
func Split(uri string) (scheme, location, path string) {
	if len(uri) == 0 {
		return
	}
	if strings.HasPrefix(uri, ":memory:") {
		location, path = uri[:8], uri[8:]
	} else if i := strings.Index(uri, "://"); i >= 0 {
		scheme, uri = uri[:i], uri[i+3:]
		if scheme == "file" {
			// For file:// URIs, location is always empty and path is the full path
			// after the scheme with leading slash trimmed
			path = uri
		} else {
			location, path, _ = strings.Cut(uri, "/")
		}
	} else {
		path = uri
	}
	return scheme, location, Clean(path)
}

// Join joins the scheme, location, and path into a URI.
//
// Note: for file URIs, the path is always expressed as an absolute reference.
func Join(scheme, location string, path ...string) string {
	var uri string

	if len(path) != 0 {
		var b strings.Builder
		for _, key := range path {
			for elem := range walk(key) {
				b.WriteByte('/')
				b.WriteString(elem)
			}
		}
		if strings.HasSuffix(path[len(path)-1], "/") {
			b.WriteByte('/')
		}
		uri = trimLeadingSlashes(b.String())
	}

	uri = join2(location, uri)
	switch scheme {
	case "":
	case "file":
		uri = "file:///" + uri
	default:
		uri = scheme + "://" + uri
	}
	return uri
}

func join(seq iter.Seq[string]) string {
	var b strings.Builder
	for elem := range seq {
		b.WriteByte('/')
		b.WriteString(elem)
	}
	return trimLeadingSlashes(b.String())
}

func join2(base, name string) string {
	switch {
	case base == "":
		return name
	case name == "":
		return base
	case strings.HasPrefix(base, ":"):
		return base + name
	default:
		return base + "/" + name
	}
}

func walk(key string) iter.Seq[string] {
	return func(yield func(string) bool) {
		for {
			if key = trimLeadingSlashes(key); key == "" {
				break
			}
			var elem string
			elem, key, _ = strings.Cut(key, "/")
			if !yield(elem) {
				break
			}
		}
	}
}

func trimLeadingSlashes(s string) string {
	for len(s) > 0 && s[0] == '/' {
		s = s[1:]
	}
	return s
}
