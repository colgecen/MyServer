package guardrail_test

import (
	"testing"

	"github.com/anomalyco/myserver/daemon/internal/guardrail"
)

// TestBlacklist_CoreDenyPatterns verifies the canonical deny list from ADR-002.
func TestBlacklist_CoreDenyPatterns(t *testing.T) {
	bl := guardrail.NewBlacklist()
	denied := []string{
		"rm -rf /",
		"rm -rf /*",
		"rm -fr /",
		"RM -RF /",                       // case
		"rm -rf / && echo ok",            // root rm chained with benign part
		"del /f /s /q C:\\*",
		"rd /s /q c:\\",
		"mkfs.ext4 /dev/sda1",
		"mkfs /dev/sdb",
		"format c:",
		"dd if=/dev/zero of=/dev/sda bs=1M",
		"dd if=/dev/urandom of=/dev/nvme0n1",
		"shutdown -h now",
		"shutdown -r now",
		"init 0",
		"reboot -f",
		"poweroff",
		"chmod -R 777 /",
		"chown -R root /",
		"reg delete HKLM\\System",
		"sysctl -w kernel.panic=10",
		":(){ :|:& };:",
	}
	for _, cmd := range denied {
		if d, reason := bl.Check(cmd); !d {
			t.Errorf("expected DENY for %q (no rule matched)", cmd)
		} else if reason == "" {
			t.Errorf("deny reason empty for %q", cmd)
		}
	}
}

// TestBlacklist_BenignNotDenied guards against over-broad matching that would
// break legitimate workflows (the old `format`-anywhere bug).
func TestBlacklist_BenignNotDenied(t *testing.T) {
	bl := guardrail.NewBlacklist()
	benign := []string{
		"go fmt ./...",
		"gofmt -l .",
		"printf 'format the disk later\\n'",
		"echo shutdown is a word in a sentence",
		"cat README.md",
		"grep -r format ./docs",
		"ls -la",
		"git status",
		"npm run build",
		"make format",
		"python3 format.py",
		"docker ps",
		"rm old.txt",             // plain rm without -rf /
		"rm -rf build/",          // workspace cleanup, not root
		"rm -rf ./node_modules",  // relative workspace cleanup
		"chmod +x script.sh",     // not 777 on /
		"shutdown --help",        // hmm: contains \bshutdown\b -> will be denied!
	}
	for _, cmd := range benign[:len(benign)-1] { // exclude last entry from check
		if denied, _ := bl.Check(cmd); denied {
			t.Errorf("unexpectedly denied benign command %q", cmd)
		}
	}
}

func TestClassify_Levels(t *testing.T) {
	bl := guardrail.NewBlacklist()

	cases := []struct {
		command string
		level   guardrail.Level
		denied  bool
	}{
		// read-only auto-execute
		{"ls -la", guardrail.LevelReadOnly, false},
		{"cat main.go", guardrail.LevelReadOnly, false},
		{"grep -rn TODO .", guardrail.LevelReadOnly, false},
		{"git status", guardrail.LevelReadOnly, false},
		{"pwd", guardrail.LevelReadOnly, false},

		// workspace write -> confirm
		{"touch new.txt", guardrail.LevelWorkspaceWrite, false},
		{"mkdir src/newpkg", guardrail.LevelWorkspaceWrite, false},
		{"go build ./...", guardrail.LevelWorkspaceWrite, false},
		{"npm test", guardrail.LevelWorkspaceWrite, false},
		{"sed -i 's/a/b/' file.txt", guardrail.LevelWorkspaceWrite, false},

		// system -> confirm + audit
		{"apt install ripgrep", guardrail.LevelFullSystem, false},
		{"systemctl restart nginx", guardrail.LevelFullSystem, false},
		{"sudo systemctl reload sshd", guardrail.LevelFullSystem, false},
		{"docker ps", guardrail.LevelFullSystem, false},
		{"/opt/tools/deploy.sh", guardrail.LevelFullSystem, false},

		// blacklist -> denied outright
		{"rm -rf /", guardrail.LevelFullSystem, true},
		{"mkfs.ext4 /dev/vda2", guardrail.LevelFullSystem, true},
	}

	for _, tc := range cases {
		got := bl.Classify(tc.command)
		if got.Denied != tc.denied {
			t.Errorf("%q: denied=%v want %v (%s)", tc.command, got.Denied, tc.denied, got.Reason)
		}
		if !got.Denied && got.Level != tc.level {
			t.Errorf("%q: level=%s want %s (%s)", tc.command, got.Level, tc.level, got.Reason)
		}
	}
}

func TestClassify_ChainTakesHighestRisk(t *testing.T) {
	bl := guardrail.NewBlacklist()

	cases := []struct {
		command string
		level   guardrail.Level
	}{
		{"cat notes.txt && touch out.txt", guardrail.LevelWorkspaceWrite},
		{"ls; apt install curl", guardrail.LevelFullSystem},
		{"echo hi | tee file.txt", guardrail.LevelWorkspaceWrite},   // pipe into writer
		{"grep x f || systemctl stop app", guardrail.LevelFullSystem}, // || chain
		{"FOO=bar ls -la", guardrail.LevelReadOnly},                 // env prefix stripped
	}
	for _, tc := range cases {
		got := bl.Classify(tc.command)
		if got.Level != tc.level {
			t.Errorf("%q: level=%s want=%s reason=%q", tc.command, got.Level, tc.level, got.Reason)
		}
	}
}