package mixer

import "github.com/egor-muindor/submix/internal/entry"

// Merge detects the body format and mixes the entries in. For an unknown format
// it returns the body unchanged.
func Merge(body []byte, entries []entry.Entry) ([]byte, entry.Format, error) {
	format := Detect(body)
	if len(entries) == 0 {
		return body, format, nil
	}

	switch format {
	case entry.FormatBase64:
		out, err := MergeBase64(body, entries)
		return out, format, err
	case entry.FormatLinks:
		out, err := MergeLinks(body, entries)
		return out, format, err
	case entry.FormatClash:
		out, err := MergeClash(body, entries)
		return out, format, err
	case entry.FormatSingbox:
		out, err := MergeSingbox(body, entries)
		return out, format, err
	case entry.FormatXrayJSON:
		out, err := MergeXrayJSON(body, entries)
		return out, format, err
	default:
		return body, format, nil
	}
}
