package mixer

import "fmt"

// withRecover turns a panic inside fn into a regular error. The fail-open
// invariant matters more than mixing: on an unexpected panel response structure
// the caller must serve the original body rather than drop the connection.
func withRecover(fn func() ([]byte, error)) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("panic recovered: %v", r)
		}
	}()
	return fn()
}
