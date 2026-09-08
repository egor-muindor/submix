package mixer_test

import (
	"encoding/json"
	"testing"

	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/mixer"
)

const xrayBody = `[
  {
    "remarks": "Panel-1",
    "log": {"loglevel": "warning"},
    "outbounds": [
      {"tag": "proxy", "protocol": "vless", "settings": {"vnext": []}},
      {"tag": "direct", "protocol": "freedom"}
    ]
  }
]`

func TestMergeXrayJSONClonesTemplate(t *testing.T) {
	entries := []entry.Entry{
		{Name: "DE-1", URI: "vless://uuid@de.example.com:443?type=tcp&security=reality&pbk=PK&sid=aa&sni=x.com&fp=chrome#Provider"},
	}

	out, err := mixer.MergeXrayJSON([]byte(xrayBody), entries)
	if err != nil {
		t.Fatalf("MergeXrayJSON: %v", err)
	}

	var arr []map[string]any
	if err := json.Unmarshal(out, &arr); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if len(arr) != 2 {
		t.Fatalf("configs = %d", len(arr))
	}

	added := arr[1]
	if added["remarks"] != "DE-1" {
		t.Fatalf("remarks = %v", added["remarks"])
	}
	if _, ok := added["log"]; !ok {
		t.Fatalf("template fields must be cloned: %#v", added)
	}

	obs := added["outbounds"].([]any)
	if len(obs) != 2 {
		t.Fatalf("outbounds = %#v", obs)
	}
	proxy := obs[0].(map[string]any)
	if proxy["tag"] != "proxy" {
		t.Fatalf("tag of first outbound must be kept: %#v", proxy)
	}
	vnext := proxy["settings"].(map[string]any)["vnext"].([]any)
	server := vnext[0].(map[string]any)
	if server["address"] != "de.example.com" {
		t.Fatalf("server = %#v", server)
	}
	if direct := obs[1].(map[string]any); direct["tag"] != "direct" {
		t.Fatalf("other outbounds must be kept: %#v", direct)
	}

	// The original config is unchanged.
	if arr[0]["remarks"] != "Panel-1" {
		t.Fatalf("original config changed: %#v", arr[0])
	}
	origProxy := arr[0]["outbounds"].([]any)[0].(map[string]any)
	if _, ok := origProxy["streamSettings"]; ok {
		t.Fatalf("original outbound must not be modified: %#v", origProxy)
	}
}

func TestMergeXrayJSONEmptyArray(t *testing.T) {
	entries := []entry.Entry{{Name: "DE-1", URI: "vless://uuid@h:443?security=tls#DE"}}
	if _, err := mixer.MergeXrayJSON([]byte(`[]`), entries); err == nil {
		t.Fatal("want error on empty template array")
	}
}

func TestMergeXrayJSONSkipsUnconvertible(t *testing.T) {
	entries := []entry.Entry{{Name: "H2", URI: "hysteria2://pw@h:443#H2"}}
	out, err := mixer.MergeXrayJSON([]byte(xrayBody), entries)
	if err != nil {
		t.Fatalf("MergeXrayJSON: %v", err)
	}
	if string(out) != xrayBody {
		t.Fatal("body must be unchanged when nothing was added")
	}
}
