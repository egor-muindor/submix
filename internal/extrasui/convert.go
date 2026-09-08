package extrasui

import (
	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/mixer"
)

// ConversionErrors runs the entry through the mixer converters and returns
// errors keyed by format. Returns nil if the entry converts to every format.
func ConversionErrors(e entry.Entry) map[string]string {
	errs := map[string]string{}
	if _, err := mixer.ClashProxy(e); err != nil {
		errs[entry.OverrideClash] = err.Error()
	}
	if _, err := mixer.SingboxOutbound(e); err != nil {
		errs[entry.OverrideSingbox] = err.Error()
	}
	if _, err := mixer.XrayOutbound(e); err != nil {
		errs[entry.OverrideXray] = err.Error()
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}
