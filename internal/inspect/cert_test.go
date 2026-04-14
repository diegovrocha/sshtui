package inspect

import (
	"strings"
	"testing"
)

func TestParseKeygenL(t *testing.T) {
	canonicalUser := `/home/diego/.ssh/id_ed25519-cert.pub:
        Type: ssh-ed25519-cert-v01@openssh.com user certificate
        Public key: ED25519-CERT SHA256:abc123def456
        Signing CA: ED25519 SHA256:ca-fp-xyz789 (using ssh-ed25519)
        Key ID: "diego@corp"
        Serial: 12345
        Valid: from 2026-01-01T00:00:00 to 2027-01-01T00:00:00
        Principals:
                diego
                admin-group
        Critical Options:
                force-command=/bin/date
        Extensions:
                permit-pty
                permit-port-forwarding
`

	hostCert := `/etc/ssh/ssh_host_ecdsa_key-cert.pub:
        Type: ecdsa-sha2-nistp256-cert-v01@openssh.com host certificate
        Public key: ECDSA-CERT SHA256:hostfp
        Signing CA: RSA SHA256:ca-rsa (using rsa-sha2-256)
        Key ID: "host-key-01"
        Serial: 42
        Valid: from 2026-02-01T00:00:00 to 2026-08-01T00:00:00
        Principals:
                host1.example.com
                host1
        Critical Options: (none)
        Extensions: (none)
`

	noOptsNoExts := `cert.pub:
        Type: ssh-rsa-cert-v01@openssh.com user certificate
        Public key: RSA-CERT SHA256:rsafp
        Signing CA: RSA SHA256:cafp (using rsa-sha2-512)
        Key ID: "minimal"
        Serial: 0
        Valid: forever
        Principals:
                alice
`

	tests := []struct {
		name            string
		input           string
		wantErr         bool
		wantKind        string
		wantType        string
		wantKeyID       string
		wantSerial      string
		wantFrom        string
		wantUntil       string
		wantPrincipals  []string
		wantCritical    []string
		wantExtensions  []string
		wantPubKeyAlg   string
		wantPubKeyFP    string
		wantSigningCAFP string
	}{
		{
			name:            "canonical user cert",
			input:           canonicalUser,
			wantKind:        "user",
			wantType:        "ssh-ed25519-cert-v01@openssh.com user certificate",
			wantKeyID:       "diego@corp",
			wantSerial:      "12345",
			wantFrom:        "2026-01-01T00:00:00",
			wantUntil:       "2027-01-01T00:00:00",
			wantPrincipals:  []string{"diego", "admin-group"},
			wantCritical:    []string{"force-command=/bin/date"},
			wantExtensions:  []string{"permit-pty", "permit-port-forwarding"},
			wantPubKeyAlg:   "ED25519-CERT",
			wantPubKeyFP:    "SHA256:abc123def456",
			wantSigningCAFP: "SHA256:ca-fp-xyz789",
		},
		{
			name:            "host cert",
			input:           hostCert,
			wantKind:        "host",
			wantType:        "ecdsa-sha2-nistp256-cert-v01@openssh.com host certificate",
			wantKeyID:       "host-key-01",
			wantSerial:      "42",
			wantFrom:        "2026-02-01T00:00:00",
			wantUntil:       "2026-08-01T00:00:00",
			wantPrincipals:  []string{"host1.example.com", "host1"},
			wantPubKeyAlg:   "ECDSA-CERT",
			wantPubKeyFP:    "SHA256:hostfp",
			wantSigningCAFP: "SHA256:ca-rsa",
		},
		{
			name:            "no critical options and no extensions sections",
			input:           noOptsNoExts,
			wantKind:        "user",
			wantType:        "ssh-rsa-cert-v01@openssh.com user certificate",
			wantKeyID:       "minimal",
			wantSerial:      "0",
			wantUntil:       "forever",
			wantPrincipals:  []string{"alice"},
			wantPubKeyAlg:   "RSA-CERT",
			wantPubKeyFP:    "SHA256:rsafp",
			wantSigningCAFP: "SHA256:cafp",
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "malformed input with no recognizable fields",
			input:   "this is not\na certificate output\nat all\n",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			info, err := parseKeygenL(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (info=%+v)", info)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantType != "" && info.Type != tc.wantType {
				t.Errorf("Type: got %q want %q", info.Type, tc.wantType)
			}
			if tc.wantKind != "" && info.CertKind != tc.wantKind {
				t.Errorf("CertKind: got %q want %q", info.CertKind, tc.wantKind)
			}
			if tc.wantKeyID != "" && info.KeyID != tc.wantKeyID {
				t.Errorf("KeyID: got %q want %q", info.KeyID, tc.wantKeyID)
			}
			if tc.wantSerial != "" && info.Serial != tc.wantSerial {
				t.Errorf("Serial: got %q want %q", info.Serial, tc.wantSerial)
			}
			if tc.wantFrom != "" && info.ValidFrom != tc.wantFrom {
				t.Errorf("ValidFrom: got %q want %q", info.ValidFrom, tc.wantFrom)
			}
			if tc.wantUntil != "" && info.ValidUntil != tc.wantUntil {
				t.Errorf("ValidUntil: got %q want %q", info.ValidUntil, tc.wantUntil)
			}
			if tc.wantPubKeyAlg != "" && info.PublicKeyAlg != tc.wantPubKeyAlg {
				t.Errorf("PublicKeyAlg: got %q want %q", info.PublicKeyAlg, tc.wantPubKeyAlg)
			}
			if tc.wantPubKeyFP != "" && info.PublicKeyFP != tc.wantPubKeyFP {
				t.Errorf("PublicKeyFP: got %q want %q", info.PublicKeyFP, tc.wantPubKeyFP)
			}
			if tc.wantSigningCAFP != "" && info.SigningCAFP != tc.wantSigningCAFP {
				t.Errorf("SigningCAFP: got %q want %q", info.SigningCAFP, tc.wantSigningCAFP)
			}
			if !equalStringSlice(info.Principals, tc.wantPrincipals) {
				t.Errorf("Principals: got %v want %v", info.Principals, tc.wantPrincipals)
			}
			if tc.wantCritical != nil && !equalStringSlice(info.CriticalOptions, tc.wantCritical) {
				t.Errorf("CriticalOptions: got %v want %v", info.CriticalOptions, tc.wantCritical)
			}
			if tc.wantCritical == nil && len(info.CriticalOptions) != 0 {
				t.Errorf("CriticalOptions: expected none, got %v", info.CriticalOptions)
			}
			if tc.wantExtensions != nil && !equalStringSlice(info.Extensions, tc.wantExtensions) {
				t.Errorf("Extensions: got %v want %v", info.Extensions, tc.wantExtensions)
			}
			if tc.wantExtensions == nil && len(info.Extensions) != 0 {
				t.Errorf("Extensions: expected none, got %v", info.Extensions)
			}
		})
	}
}

func TestParseKeygenL_SigningCAAlg(t *testing.T) {
	input := `file-cert.pub:
        Type: ssh-ed25519-cert-v01@openssh.com user certificate
        Public key: ED25519-CERT SHA256:xyz
        Signing CA: ED25519 SHA256:cafp (using ssh-ed25519)
        Key ID: "kid"
        Serial: 1
        Valid: from 2026-01-01T00:00:00 to 2027-01-01T00:00:00
        Principals:
                x
`
	info, err := parseKeygenL(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.SigningCAAlg != "ssh-ed25519" {
		t.Errorf("SigningCAAlg: got %q want %q", info.SigningCAAlg, "ssh-ed25519")
	}
	if !strings.Contains(info.SigningCA, "SHA256:cafp") {
		t.Errorf("SigningCA should retain the original value, got %q", info.SigningCA)
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
