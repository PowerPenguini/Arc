package main

import (
	"strings"
	"testing"
)

func TestRenderArcExports(t *testing.T) {
	got := renderArcExports("1001", "1001")
	want := "/home/arc 10.0.0.2/32(rw,sync,all_squash,no_subtree_check,anonuid=1001,anongid=1001,sec=sys)\n"
	if got != want {
		t.Fatalf("unexpected exports content:\n got: %q\nwant: %q", got, want)
	}
}

func TestRenderArcFstabLine(t *testing.T) {
	got := renderArcFstabLine()
	for _, need := range []string{
		"10.0.0.1:/home/arc",
		"/home/arc",
		"nfs4",
		"x-systemd.automount",
		"_netdev",
		"nofail",
		"nfsvers=4.2",
	} {
		if !strings.Contains(got, need) {
			t.Fatalf("fstab line missing %q: %q", need, got)
		}
	}
}

func TestUpsertFstabEntry_AppendsWhenMissing(t *testing.T) {
	in := "# /etc/fstab\nUUID=abc / ext4 defaults 0 1\n"
	entry := renderArcFstabLine()

	out, changed, err := upsertFstabEntry(in, "/home/arc", entry)
	if err != nil {
		t.Fatalf("upsertFstabEntry: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true when entry is missing")
	}
	if !strings.Contains(out, entry) {
		t.Fatalf("expected output to include new entry; got: %q", out)
	}
}

func TestUpsertFstabEntry_ReplacesExistingEntry(t *testing.T) {
	in := strings.Join([]string{
		"# /etc/fstab",
		"10.0.0.1:/home/arc /home/arc nfs4 defaults 0 0",
		"",
	}, "\n")
	entry := renderArcFstabLine()

	out, changed, err := upsertFstabEntry(in, "/home/arc", entry)
	if err != nil {
		t.Fatalf("upsertFstabEntry: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true when replacing old entry")
	}
	if strings.Contains(out, "nfs4 defaults 0 0") {
		t.Fatalf("expected legacy entry to be replaced, got: %q", out)
	}
	if !strings.Contains(out, entry) {
		t.Fatalf("expected output to include replacement entry; got: %q", out)
	}
}

func TestUpsertFstabEntry_NoChangeForExactEntry(t *testing.T) {
	entry := renderArcFstabLine()
	in := entry + "\n"

	out, changed, err := upsertFstabEntry(in, "/home/arc", entry)
	if err != nil {
		t.Fatalf("upsertFstabEntry: %v", err)
	}
	if changed {
		t.Fatalf("expected changed=false for exact entry")
	}
	if out != in {
		t.Fatalf("unexpected output: got %q want %q", out, in)
	}
}
