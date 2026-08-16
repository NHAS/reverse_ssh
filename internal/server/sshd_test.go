package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestMain(m *testing.M) {
	// readPubKeys and parseOptions deliberately log skipped keys and unsupported
	// options, and several tests below exercise those paths on purpose.
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// containsIP reports whether any of nets covers the given address.
func containsIP(nets []*net.IPNet, ip string) bool {
	parsed := net.ParseIP(ip)
	for _, n := range nets {
		if n.Contains(parsed) {
			return true
		}
	}

	return false
}

func TestParseAddress(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		wantLen  int
		covers   []string
		excludes []string
		wantErr  bool
	}{
		{
			name:    "wildcard covers both families",
			address: "*",
			wantLen: 2,
			covers:  []string{"192.168.1.1", "2001:db8::1"},
		},
		{
			name:     "ipv4 cidr",
			address:  "10.0.0.0/8",
			wantLen:  1,
			covers:   []string{"10.1.2.3"},
			excludes: []string{"11.0.0.1"},
		},
		{
			name:     "ipv6 cidr",
			address:  "2001:db8::/32",
			wantLen:  1,
			covers:   []string{"2001:db8::1"},
			excludes: []string{"2001:db9::1"},
		},
		{
			// Regression: a bare literal used to build an IPNet with a nil IP,
			// which covered nothing. The entry was then dropped, and an empty
			// allow list means "no restriction" to CheckAuth.
			name:     "bare ipv4 literal covers only itself",
			address:  "192.168.1.5",
			wantLen:  1,
			covers:   []string{"192.168.1.5"},
			excludes: []string{"192.168.1.6", "192.168.2.5"},
		},
		{
			name:     "bare ipv6 literal covers only itself",
			address:  "2001:db8::1",
			wantLen:  1,
			covers:   []string{"2001:db8::1"},
			excludes: []string{"2001:db8::2"},
		},
		{
			// Holds whether the resolver answers NXDOMAIN or is absent entirely,
			// since both produce an error from net.LookupIP.
			name:    "unresolvable hostname",
			address: "rssh-test-host.invalid",
			wantErr: true,
		},
		{
			name:    "not an address at all",
			address: "not an address",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAddress(tt.address)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseAddress(%q) succeeded, expected an error", tt.address)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAddress(%q) failed: %v", tt.address, err)
			}

			if len(got) != tt.wantLen {
				t.Fatalf("ParseAddress(%q) returned %d networks, want %d", tt.address, len(got), tt.wantLen)
			}

			for _, ip := range tt.covers {
				if !containsIP(got, ip) {
					t.Fatalf("ParseAddress(%q) does not cover %s", tt.address, ip)
				}
			}

			for _, ip := range tt.excludes {
				if containsIP(got, ip) {
					t.Fatalf("ParseAddress(%q) unexpectedly covers %s", tt.address, ip)
				}
			}
		})
	}
}

func TestParseFromDirective(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		wantAllow   int
		wantDeny    int
		allowCovers []string
		denyCovers  []string
		wantErr     bool
	}{
		{
			name:        "allow only",
			value:       "10.0.0.0/8",
			wantAllow:   1,
			allowCovers: []string{"10.1.2.3"},
		},
		{
			name:       "deny only",
			value:      "!10.0.0.0/8",
			wantDeny:   1,
			denyCovers: []string{"10.1.2.3"},
		},
		{
			name:        "mixed deny and allow",
			value:       "!10.1.0.0/16,10.0.0.0/8",
			wantAllow:   1,
			wantDeny:    1,
			allowCovers: []string{"10.2.0.1"},
			denyCovers:  []string{"10.1.0.1"},
		},
		{
			name:      "surrounding quotes are stripped",
			value:     `"10.0.0.0/8"`,
			wantAllow: 1,
		},
		{
			name:      "wildcard expands to both families",
			value:     "*",
			wantAllow: 2,
		},
		{
			name:      "whitespace between entries",
			value:     "10.0.0.0/8, 192.168.0.0/16",
			wantAllow: 2,
		},
		{
			name:  "empty value is not a restriction",
			value: "",
		},
		{
			// Regression: an unparsable entry used to be logged and skipped. If it
			// was the only allow entry the key ended up unrestricted.
			name:    "unparsable allow entry is an error",
			value:   "10.0.0.0/8,not an address",
			wantErr: true,
		},
		{
			name:    "unparsable deny entry is an error",
			value:   "!not an address",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deny, allow, err := ParseFromDirective(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseFromDirective(%q) succeeded, expected an error", tt.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFromDirective(%q) failed: %v", tt.value, err)
			}

			if len(allow) != tt.wantAllow {
				t.Fatalf("ParseFromDirective(%q) returned %d allow entries, want %d", tt.value, len(allow), tt.wantAllow)
			}

			if len(deny) != tt.wantDeny {
				t.Fatalf("ParseFromDirective(%q) returned %d deny entries, want %d", tt.value, len(deny), tt.wantDeny)
			}

			for _, ip := range tt.allowCovers {
				if !containsIP(allow, ip) {
					t.Fatalf("ParseFromDirective(%q) allow list does not cover %s", tt.value, ip)
				}
			}

			for _, ip := range tt.denyCovers {
				if !containsIP(deny, ip) {
					t.Fatalf("ParseFromDirective(%q) deny list does not cover %s", tt.value, ip)
				}
			}
		})
	}
}

func TestParseOwnerDirective(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    []string
		wantErr bool
	}{
		{
			name:  "single owner",
			value: `"jim"`,
			want:  []string{"jim"},
		},
		{
			name:  "multiple owners",
			value: `"jim,bob"`,
			want:  []string{"jim", "bob"},
		},
		{
			name:  "surrounding whitespace is trimmed",
			value: `" jim , bob "`,
			want:  []string{"jim", "bob"},
		},
		{
			name:  "explicitly empty means public",
			value: `""`,
			want:  nil,
		},
		{
			// Regression: an unquoted value used to yield a nil owner list, and a
			// nil owner list means the client is public to every user.
			name:    "unquoted value is an error",
			value:   "jim",
			wantErr: true,
		},
		{
			name:    "unbalanced quotes are an error",
			value:   `"jim`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOwnerDirective(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseOwnerDirective(%q) succeeded, expected an error", tt.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseOwnerDirective(%q) failed: %v", tt.value, err)
			}

			if !slices.Equal(got, tt.want) {
				t.Fatalf("ParseOwnerDirective(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestParseOptions(t *testing.T) {
	tests := []struct {
		name       string
		options    []string
		wantSingle bool
		wantAllow  int
		wantDeny   int
		wantOwners []string
		wantErr    bool
	}{
		{
			name:       "single_session",
			options:    []string{"single_session"},
			wantSingle: true,
		},
		{
			name:    "single_session does not take a value",
			options: []string{"single_session=yes"},
			wantErr: true,
		},
		{
			name:      "from allow entry",
			options:   []string{`from="10.0.0.0/8"`},
			wantAllow: 1,
		},
		{
			// Regression: OpenSSH option names are case insensitive, so From= used
			// to miss the from= handler and be silently unenforced.
			name:      "from is matched case insensitively",
			options:   []string{`From="10.0.0.0/8"`},
			wantAllow: 1,
		},
		{
			name:       "owner",
			options:    []string{`owner="jim,bob"`},
			wantOwners: []string{"jim", "bob"},
		},
		{
			name:       "owner is matched case insensitively",
			options:    []string{`Owner="jim"`},
			wantOwners: []string{"jim"},
		},
		{
			name:    "from without a value is an error",
			options: []string{"from"},
			wantErr: true,
		},
		{
			name:    "owner without a value is an error",
			options: []string{"owner"},
			wantErr: true,
		},
		{
			name:    "unparsable from is an error",
			options: []string{`from="not an address"`},
			wantErr: true,
		},
		{
			name:    "unquoted owner is an error",
			options: []string{"owner=jim"},
			wantErr: true,
		},
		{
			name:    "unsupported openssh option is rejected",
			options: []string{"no-pty"},
			wantErr: true,
		},
		{
			name:    "unsupported option with a value is rejected",
			options: []string{`command="/bin/false"`},
			wantErr: true,
		},
		{
			// A misspelling of from= is indistinguishable from an option this
			// server does not implement, so rejecting unknown options is what
			// stops it from silently loading a key with no address restriction.
			name:    "misspelled from is rejected",
			options: []string{`form="10.0.0.0/8"`},
			wantErr: true,
		},
		{
			name:       "several options together",
			options:    []string{`from="10.0.0.0/8"`, `owner="jim"`, "single_session"},
			wantAllow:  1,
			wantOwners: []string{"jim"},
			wantSingle: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseOptions(tt.options)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseOptions(%q) succeeded, expected an error", tt.options)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseOptions(%q) failed: %v", tt.options, err)
			}

			if opts.SingleSession != tt.wantSingle {
				t.Fatalf("SingleSession = %t, want %t", opts.SingleSession, tt.wantSingle)
			}

			if len(opts.AllowList) != tt.wantAllow {
				t.Fatalf("AllowList has %d entries, want %d", len(opts.AllowList), tt.wantAllow)
			}

			if len(opts.DenyList) != tt.wantDeny {
				t.Fatalf("DenyList has %d entries, want %d", len(opts.DenyList), tt.wantDeny)
			}

			if !slices.Equal(opts.Owners, tt.wantOwners) {
				t.Fatalf("Owners = %q, want %q", opts.Owners, tt.wantOwners)
			}
		})
	}
}

func testPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("could not generate test key: %v", err)
	}

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("could not convert test key: %v", err)
	}

	return sshPub
}

// authorizedKeyLine renders key as an authorized_keys entry, optionally prefixed
// with an options field.
func authorizedKeyLine(options string, key ssh.PublicKey) string {
	entry := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
	if options == "" {
		return entry
	}

	return options + " " + entry
}

func writeKeysFile(t *testing.T, lines ...string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "authorized_keys")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatalf("could not write test keys file: %v", err)
	}

	return path
}

func TestReadPubKeys(t *testing.T) {
	good := testPublicKey(t)

	t.Run("a malformed line does not discard the rest of the file", func(t *testing.T) {
		// Regression: one bad line used to abort the whole file, which turned a
		// single typo into a lockout of every administrator listed in it.
		path := writeKeysFile(t, "this is not a public key", authorizedKeyLine("", good))

		keys, err := readPubKeys(path)
		if err != nil {
			t.Fatalf("readPubKeys failed: %v", err)
		}

		if len(keys) != 1 {
			t.Fatalf("loaded %d keys, want 1", len(keys))
		}
	})

	t.Run("a key with an unparsable from is dropped but others load", func(t *testing.T) {
		bad := testPublicKey(t)
		path := writeKeysFile(t,
			authorizedKeyLine(`from="not an address"`, bad),
			authorizedKeyLine("", good),
		)

		keys, err := readPubKeys(path)
		if err != nil {
			t.Fatalf("readPubKeys failed: %v", err)
		}

		if len(keys) != 1 {
			t.Fatalf("loaded %d keys, want 1", len(keys))
		}

		if _, ok := keys[string(ssh.MarshalAuthorizedKey(bad))]; ok {
			t.Fatal("key with an unparsable from directive was loaded anyway")
		}
	})

	t.Run("a key carrying an unsupported option is dropped but others load", func(t *testing.T) {
		// rssh cannot honour arbitrary authorized_keys options, so a key that
		// asks for one is rejected rather than loaded with the option ignored.
		bad := testPublicKey(t)
		path := writeKeysFile(t,
			authorizedKeyLine("no-pty", bad),
			authorizedKeyLine("", good),
		)

		keys, err := readPubKeys(path)
		if err != nil {
			t.Fatalf("readPubKeys failed: %v", err)
		}

		if len(keys) != 1 {
			t.Fatalf("loaded %d keys, want 1", len(keys))
		}

		if _, ok := keys[string(ssh.MarshalAuthorizedKey(bad))]; ok {
			t.Fatal("key carrying an unsupported option was loaded anyway")
		}
	})

	t.Run("a missing file reports os.ErrNotExist", func(t *testing.T) {
		// CheckAuth distinguishes "no such file" from a real IO error using
		// errors.Is, which only works while the wrapping preserves the chain.
		_, err := readPubKeys(filepath.Join(t.TempDir(), "does-not-exist"))
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("error %v does not wrap os.ErrNotExist", err)
		}
	})
}

func TestCheckAuth(t *testing.T) {
	key := testPublicKey(t)

	t.Run("key in the list with no restriction is allowed", func(t *testing.T) {
		path := writeKeysFile(t, authorizedKeyLine("", key))

		if _, err := CheckAuth(path, key, net.ParseIP("192.168.1.1"), false); err != nil {
			t.Fatalf("CheckAuth failed: %v", err)
		}
	})

	t.Run("source inside the allow list is permitted", func(t *testing.T) {
		path := writeKeysFile(t, authorizedKeyLine(`from="10.0.0.0/8"`, key))

		if _, err := CheckAuth(path, key, net.ParseIP("10.1.2.3"), false); err != nil {
			t.Fatalf("CheckAuth failed: %v", err)
		}
	})

	t.Run("source outside the allow list is refused", func(t *testing.T) {
		path := writeKeysFile(t, authorizedKeyLine(`from="10.0.0.0/8"`, key))

		if _, err := CheckAuth(path, key, net.ParseIP("192.168.1.1"), false); err == nil {
			t.Fatal("CheckAuth allowed a source outside the allow list")
		}
	})

	t.Run("single host allow list permits only that host", func(t *testing.T) {
		path := writeKeysFile(t, authorizedKeyLine(`from="192.168.1.5"`, key))

		if _, err := CheckAuth(path, key, net.ParseIP("192.168.1.5"), false); err != nil {
			t.Fatalf("CheckAuth refused the permitted host: %v", err)
		}

		if _, err := CheckAuth(path, key, net.ParseIP("192.168.1.6"), false); err == nil {
			t.Fatal("CheckAuth allowed a host outside a single host allow list")
		}
	})

	t.Run("source on the deny list is refused", func(t *testing.T) {
		path := writeKeysFile(t, authorizedKeyLine(`from="!10.1.0.0/16,10.0.0.0/8"`, key))

		if _, err := CheckAuth(path, key, net.ParseIP("10.1.2.3"), false); err == nil {
			t.Fatal("CheckAuth allowed a source on the deny list")
		}

		if _, err := CheckAuth(path, key, net.ParseIP("10.2.0.1"), false); err != nil {
			t.Fatalf("CheckAuth refused an allowed source: %v", err)
		}
	})

	t.Run("unknown key reports ErrKeyNotInList", func(t *testing.T) {
		path := writeKeysFile(t, authorizedKeyLine("", key))

		_, err := CheckAuth(path, testPublicKey(t), net.ParseIP("10.1.2.3"), false)
		if !errors.Is(err, ErrKeyNotInList) {
			t.Fatalf("error = %v, want ErrKeyNotInList", err)
		}
	})

	t.Run("owners are recorded in the permissions", func(t *testing.T) {
		path := writeKeysFile(t, authorizedKeyLine(`owner="jim,bob"`, key))

		perms, err := CheckAuth(path, key, net.ParseIP("10.1.2.3"), false)
		if err != nil {
			t.Fatalf("CheckAuth failed: %v", err)
		}

		if got := perms.Extensions["owners"]; got != "jim,bob" {
			t.Fatalf("owners extension = %q, want %q", got, "jim,bob")
		}
	})

	t.Run("insecure mode tolerates a missing key file", func(t *testing.T) {
		// Regression: the file read used to short circuit before the insecure
		// check, so an absent authorized_controllee_keys blocked every client
		// even when the server was started with --insecure.
		missing := filepath.Join(t.TempDir(), "does-not-exist")

		if _, err := CheckAuth(missing, key, net.ParseIP("10.1.2.3"), true); err != nil {
			t.Fatalf("CheckAuth in insecure mode failed: %v", err)
		}
	})

	t.Run("secure mode refuses when the key file is missing", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "does-not-exist")

		_, err := CheckAuth(missing, key, net.ParseIP("10.1.2.3"), false)
		if !errors.Is(err, ErrKeyNotInList) {
			t.Fatalf("error = %v, want ErrKeyNotInList", err)
		}
	})
}
