package integrity

import "testing"

func TestManifestCanonicalizesOrder(t *testing.T) {
	first, err := NewManifest([]Artifact{{Path: "z/report", Digest: "bbb"}, {Path: "a/counts", Digest: "aaa"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewManifest([]Artifact{{Path: "a/counts", Digest: "aaa"}, {Path: "z/report", Digest: "bbb"}})
	if err != nil {
		t.Fatal(err)
	}
	if first.DigestHex() != second.DigestHex() {
		t.Fatal("artifact order changed manifest identity")
	}
	if string(first.CanonicalBytes()) != `{"artifacts":[{"path":"a/counts","digest":"aaa"},{"path":"z/report","digest":"bbb"}]}` {
		t.Fatalf("canonical bytes: %s", first.CanonicalBytes())
	}
}
