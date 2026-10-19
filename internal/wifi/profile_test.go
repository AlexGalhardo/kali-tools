package wifi

import "testing"

func TestSyntheticProfileStates(t *testing.T) {
	for _, test := range []struct{ raw, state, value string }{{`<WLANProfile><MSM><security><sharedKey><protected>false</protected><keyMaterial>synthetic-key</keyMaterial></sharedKey></security></MSM></WLANProfile>`, "available", "synthetic-key"}, {`<WLANProfile><MSM><security><sharedKey><protected>true</protected><keyMaterial>encrypted</keyMaterial></sharedKey></security></MSM></WLANProfile>`, "permission_required", ""}, {`<WLANProfile/>`, "no_saved_key", ""}} {
		got, e := secretFromProfile([]byte(test.raw))
		if e != nil || got.State != test.state || got.Value != test.value {
			t.Fatal(got.State, e)
		}
	}
	if _, e := secretFromProfile([]byte(`<bad/>`)); e == nil {
		t.Fatal("invalid profile accepted")
	}
}
