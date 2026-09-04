package nblink

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseServiceList(t *testing.T) {
	data := []byte(`{"jd":[{"ip":16777343,"name":"DB","ports":[3306],"peerid":"peer","icon":"db","otype":1}]}`)
	services, err := ParseServiceList(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 1 || services[0].Endpoint.Host != "127.0.0.1" || services[0].Endpoint.TargetPort != 3306 {
		t.Fatalf("unexpected services: %+v", services)
	}
}

func TestIPv4FromUint32LE(t *testing.T) {
	if got := IPv4FromUint32LE(16777343); got != "127.0.0.1" {
		t.Fatalf("got %s", got)
	}
}

func TestParseServiceListAcceptsSuccessfulEmptyList(t *testing.T) {
	services, err := ParseServiceList([]byte(`{"rtn":0,"jd":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 0 {
		t.Fatalf("unexpected services: %+v", services)
	}
}

func TestReadLastServiceListAllowsTrailingLogText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nblink.log")
	line := `INFO [/jdis/servicelist] {"jd":[{"ip":16777343,"name":"SSH","ports":[22],"peerid":"peer"}]} duration=20ms` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	services, err := readLastServiceList(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 1 || services[0].Endpoint.TargetPort != 22 {
		t.Fatalf("unexpected services: %+v", services)
	}
}
