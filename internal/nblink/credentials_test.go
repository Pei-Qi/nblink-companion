package nblink

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCredentialsAndWakeTargets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user_service.db")
	data := "{\"version\":1,\"sembast\":1}\n" +
		"{\"key\":\"jdxb-uid\",\"value\":\"100\"}\n" +
		"{\"key\":\"jdxb-owcode\",\"value\":\"secret\"}\n" +
		"{\"key\":\"jdxb-dev-list\",\"value\":[{\"peerid\":\"peer\",\"name\":\"PC\",\"isOnline\":false,\"nics\":[{\"mac\":\"001122334455\",\"ifname\":\"eth0\"}]}]}\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	creds, err := LoadCredentials(path)
	if err != nil {
		t.Fatal(err)
	}
	if creds.UID != "100" || creds.OwCode != "secret" {
		t.Fatalf("unexpected credentials: %+v", creds)
	}
	targets := WakeTargets(creds)
	if len(targets) != 1 || targets[0].MAC != "00:11:22:33:44:55" {
		t.Fatalf("unexpected targets: %+v", targets)
	}
}

func TestNormalizeMACRejectsPrivatePlaceholder(t *testing.T) {
	if _, ok := normalizeMAC("02:00:00:00:00:00"); ok {
		t.Fatal("expected placeholder MAC to be rejected")
	}
}
