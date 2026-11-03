package main

import (
	"encoding/json"
	"reflect"
	"testing"

	sdk "github.com/JuanenRac/HYDRA-UMC-SDK/clients/go"
)

func TestParseFlagSetMixesOptionsAndPositionals(t *testing.T) {
	for _, args := range [][]string{
		{"3", "--force", "--reason", "maintenance"},
		{"--force", "3", "--reason", "maintenance"},
		{"--reason", "maintenance", "3", "--force"},
	} {
		p, err := parseFlagSet("claim", args, []string{"--reason"}, []string{"--force"})
		if err != nil {
			t.Fatal(err)
		}
		if !p.Bool("--force") || p.String("--reason") != "maintenance" || !reflect.DeepEqual(p.Rest, []string{"3"}) {
			t.Fatalf("%v parsed as force=%v reason=%q rest=%v", args, p.Bool("--force"), p.String("--reason"), p.Rest)
		}
	}
}

func TestParseFlagSetKeepsUndeclaredTokensInRest(t *testing.T) {
	p, err := parseFlagSet("claims", []string{"--sever", "x", "--json"}, nil, []string{"--json"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Bool("--json") || !reflect.DeepEqual(p.Rest, []string{"--sever", "x"}) {
		t.Fatalf("got json=%v rest=%v", p.Bool("--json"), p.Rest)
	}
}

func TestParseFlagSetValueMayLookLikeAnOption(t *testing.T) {
	p, err := parseFlagSet("claim", []string{"--reason", "--json", "--json"}, []string{"--reason"}, []string{"--json"})
	if err != nil {
		t.Fatal(err)
	}
	if p.String("--reason") != "--json" || !p.Bool("--json") || len(p.Rest) != 0 {
		t.Fatalf("reason=%q json=%v rest=%v", p.String("--reason"), p.Bool("--json"), p.Rest)
	}
}

func TestParseFlagSetMissingValueIsAUsageError(t *testing.T) {
	_, err := parseFlagSet("claim", []string{"3", "--ttl-ms"}, []string{"--ttl-ms"}, nil)
	if err == nil || exitCodeFor(err) != ExitUsageError || err.Error() != "--ttl-ms needs a value" {
		t.Fatalf("got %v (exit %v)", err, exitCodeFor(err))
	}
}

func TestParseFlagSetLastOccurrenceWinsAndSeen(t *testing.T) {
	p, err := parseFlagSet("audit", []string{"--limit", "5", "--limit", "7"}, []string{"--limit", "--type"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.String("--limit") != "7" || !p.Seen("--limit") || p.Seen("--type") || p.String("--type") != "" {
		t.Fatalf("limit=%q seen=%v/%v", p.String("--limit"), p.Seen("--limit"), p.Seen("--type"))
	}
}

func TestTakeFlagAndTakeSwitchKeepTheirContract(t *testing.T) {
	v, rest, err := takeFlag([]string{"a", "--x", "1", "b"}, "--x")
	if err != nil || v != "1" || !reflect.DeepEqual(rest, []string{"a", "b"}) {
		t.Fatalf("takeFlag: %q %v %v", v, rest, err)
	}
	on, rest := takeSwitch([]string{"a", "--j", "b"}, "--j")
	if !on || !reflect.DeepEqual(rest, []string{"a", "b"}) {
		t.Fatalf("takeSwitch: %v %v", on, rest)
	}
}

// the identity answer is the SDK's own ServerDiscovery type, plus the CLI's pointer counts and status text
func TestHydraInfoIsBuiltOnTheSDKServerDiscovery(t *testing.T) {
	body := `{"schema_version":"1.0","product":"HYDRA-UMC-SERVER","remoteApiVersion":3,"appVersion":"1.2.3","hostname":"h","controllerCount":2,"robotCount":0,"uptimeSeconds":9,"status":"ok","future":true}`
	var info hydraInfo
	if err := json.Unmarshal([]byte(body), &info); err != nil {
		t.Fatal(err)
	}
	var viaSDK sdk.ServerDiscovery
	if err := json.Unmarshal([]byte(body), &viaSDK); err != nil {
		t.Fatal(err)
	}
	if info.ServerDiscovery.AppVersion != viaSDK.AppVersion || info.RemoteAPIVersion != 3 || info.UptimeSeconds != 9 || info.Status != "ok" {
		t.Fatalf("got %+v", info)
	}
	if info.ControllerCount == nil || *info.ControllerCount != 2 || info.RobotCount == nil || *info.RobotCount != 0 {
		t.Fatal("reported counts, including zero, must be present")
	}
	var older hydraInfo
	if err := json.Unmarshal([]byte(`{"appVersion":"1"}`), &older); err != nil || older.ControllerCount != nil || older.RobotCount != nil {
		t.Fatalf("unreported counts must stay nil: %+v %v", older, err)
	}
}
