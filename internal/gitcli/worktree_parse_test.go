package gitcli

import "testing"

func TestParseWorktreeListZLocked(t *testing.T) {
	h := "1111111111111111111111111111111111111111"
	stream := "worktree /p\x00HEAD " + h + "\x00branch refs/heads/main\x00\x00" +
		"worktree /a\x00HEAD " + h + "\x00branch refs/heads/a\x00locked\x00\x00" +
		"worktree /b\x00HEAD " + h + "\x00detached\x00locked on a usb disk\x00\x00"
	infos, err := parseWorktreeListZ([]byte(stream))
	if err != nil || len(infos) != 3 {
		t.Fatalf("parse = %d stanzas, %v", len(infos), err)
	}
	if infos[0].Locked || !infos[1].Locked || !infos[2].Locked || !infos[2].Detached {
		t.Errorf("locked flags = %v %v %v", infos[0].Locked, infos[1].Locked, infos[2].Locked)
	}
}
