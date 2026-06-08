package hypha

import (
	"encoding/json"
	"testing"
)

func TestEnvelopeDecodeTraceShape(t *testing.T) {
	var env envelope[[]Trace]
	body := []byte(`{"ok":true,"data":[{"id":"trace.1","space":"hypha://m31labs/reeve","agent":"agent://x","status":"open","ticks":2,"phase":"maintenance"}],"warnings":[],"errors":[]}`)
	if err := jsonUnmarshalForTest(body, &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 1 || env.Data[0].ID != "trace.1" || env.Data[0].Ticks != 2 {
		t.Fatalf("unexpected env: %#v", env)
	}
}

func jsonUnmarshalForTest(body []byte, dest any) error {
	return json.Unmarshal(body, dest)
}
