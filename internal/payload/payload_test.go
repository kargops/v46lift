package payload

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAppendParseRoundTrip(t *testing.T) {
	base := []byte("lift-binary")
	payload := []byte(`{"game":{"executable":"/opt/game"}}`)

	packed := Append(base, KindLauncher, payload)
	kind, got, stripped, err := Parse(packed)
	if err != nil {
		t.Fatal(err)
	}
	if kind != KindLauncher {
		t.Fatalf("kind=%s", kind)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload=%q", got)
	}
	if !bytes.Equal(stripped, base) {
		t.Fatalf("stripped=%q", stripped)
	}
}

func TestStripExistingPayloadBeforeAppend(t *testing.T) {
	base := []byte("lift-binary")
	first := Append(base, KindLauncher, []byte("one"))
	second := Append(first, KindInstaller, []byte("two"))

	kind, got, stripped, err := Parse(second)
	if err != nil {
		t.Fatal(err)
	}
	if kind != KindInstaller {
		t.Fatalf("kind=%s", kind)
	}
	if string(got) != "two" {
		t.Fatalf("payload=%q", got)
	}
	if !bytes.Equal(stripped, base) {
		t.Fatalf("stripped=%q", stripped)
	}
}

func TestInspectFileAndRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "packed")
	data := Append([]byte("abc"), KindLauncher, []byte("cfg"))
	if err := os.WriteFile(path, data, 0755); err != nil {
		t.Fatal(err)
	}

	info, err := InspectFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Kind != KindLauncher || info.Offset != 3 || info.Length != 3 {
		t.Fatalf("info=%+v", info)
	}

	kind, payload, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if kind != KindLauncher || string(payload) != "cfg" {
		t.Fatalf("kind=%s payload=%q", kind, payload)
	}
}

func TestNoMagicIsKindNone(t *testing.T) {
	kind, payload, stripped, err := Parse([]byte("just a regular binary"))
	if err != nil {
		t.Fatal(err)
	}
	if kind != KindNone || payload != nil || string(stripped) != "just a regular binary" {
		t.Fatalf("kind=%s payload=%q stripped=%q", kind, payload, stripped)
	}
}

func TestTruncatedPayload(t *testing.T) {
	var footer [FooterSize]byte
	putFooter(&footer, KindLauncher, 100)
	if _, err := InspectBytes(footer[:]); err == nil {
		t.Fatal("expected truncated payload error")
	}
}
