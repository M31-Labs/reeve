package workspaces

import "testing"

func TestParseJSONAcceptsCommonShapes(t *testing.T) {
	body := []byte(`{"workspaces":[{"name":"reeve","path":"/home/draco/work/reeve"}],"hyphae":"/home/draco/work/hyphae"}`)
	got, err := ParseJSON(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2: %#v", len(got), got)
	}
	if got[0].Name != "hyphae" || got[1].Name != "reeve" {
		t.Fatalf("unexpected order/content: %#v", got)
	}
}
