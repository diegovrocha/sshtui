package generate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultOutPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	tests := []struct {
		algo string
		want string
	}{
		{"ed25519", filepath.Join(home, ".ssh", "id_ed25519")},
		{"rsa", filepath.Join(home, ".ssh", "id_rsa")},
		{"ecdsa", filepath.Join(home, ".ssh", "id_ecdsa")},
		{"dsa", filepath.Join(home, ".ssh", "id_dsa")},
	}
	for _, tc := range tests {
		t.Run(tc.algo, func(t *testing.T) {
			got := defaultOutPath(tc.algo)
			if got != tc.want {
				t.Errorf("defaultOutPath(%q) = %q, want %q", tc.algo, got, tc.want)
			}
		})
	}
}

func TestValidateAlgoBits(t *testing.T) {
	tests := []struct {
		name string
		algo string
		bits string
		want bool
	}{
		{"rsa 1024 rejected", "rsa", "1024", false},
		{"rsa 2048 ok", "rsa", "2048", true},
		{"rsa 3072 ok", "rsa", "3072", true},
		{"rsa 4096 ok", "rsa", "4096", true},
		{"rsa 8192 ok (>=2048)", "rsa", "8192", true},
		{"rsa non-numeric rejected", "rsa", "abcd", false},
		{"ed25519 ignores bits (empty)", "ed25519", "", true},
		{"ed25519 ignores bits (bogus)", "ed25519", "99", true},
		{"dsa ignores bits", "dsa", "", true},
		{"ecdsa 256 ok", "ecdsa", "256", true},
		{"ecdsa 384 ok", "ecdsa", "384", true},
		{"ecdsa 521 ok", "ecdsa", "521", true},
		{"ecdsa 123 rejected", "ecdsa", "123", false},
		{"unknown algo rejected", "bogus", "2048", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateAlgoBits(tc.algo, tc.bits); got != tc.want {
				t.Errorf("validateAlgoBits(%q, %q) = %v, want %v",
					tc.algo, tc.bits, got, tc.want)
			}
		})
	}
}

func TestBuildArgs(t *testing.T) {
	tests := []struct {
		name string
		m    *KeyModel
		want []string
	}{
		{
			name: "ed25519 no bits no passphrase",
			m: &KeyModel{
				algo:       "ed25519",
				bits:       "",
				out:        "/home/u/.ssh/id_ed25519",
				comment:    "u@host",
				passphrase: "",
			},
			want: []string{
				"ssh-keygen", "-t", "ed25519",
				"-f", "/home/u/.ssh/id_ed25519",
				"-C", "u@host",
				"-N", "",
			},
		},
		{
			name: "rsa 4096 with passphrase",
			m: &KeyModel{
				algo:       "rsa",
				bits:       "4096",
				out:        "/tmp/key",
				comment:    "alice@box",
				passphrase: "s3cret",
			},
			want: []string{
				"ssh-keygen", "-t", "rsa", "-b", "4096",
				"-f", "/tmp/key",
				"-C", "alice@box",
				"-N", "s3cret",
			},
		},
		{
			name: "ecdsa 384",
			m: &KeyModel{
				algo:       "ecdsa",
				bits:       "384",
				out:        "/home/u/.ssh/id_ecdsa",
				comment:    "c",
				passphrase: "",
			},
			want: []string{
				"ssh-keygen", "-t", "ecdsa", "-b", "384",
				"-f", "/home/u/.ssh/id_ecdsa",
				"-C", "c",
				"-N", "",
			},
		},
		{
			name: "dsa no bits",
			m: &KeyModel{
				algo:       "dsa",
				bits:       "",
				out:        "/tmp/dsa",
				comment:    "legacy",
				passphrase: "",
			},
			want: []string{
				"ssh-keygen", "-t", "dsa",
				"-f", "/tmp/dsa",
				"-C", "legacy",
				"-N", "",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildArgs(tc.m)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("buildArgs mismatch\n  got:  %#v\n  want: %#v", got, tc.want)
			}
		})
	}
}

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	tests := []struct {
		in   string
		want string
	}{
		{"~/foo", filepath.Join(home, "foo")},
		{"~/.ssh/id_rsa", filepath.Join(home, ".ssh", "id_rsa")},
		{"~", home},
		{"/abs/path", "/abs/path"},
		{"relative/path", "relative/path"},
		{"", ""},
		{"~username/foo", "~username/foo"}, // only "~/" is expanded
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := expandTilde(tc.in); got != tc.want {
				t.Errorf("expandTilde(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
