package storage

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/sftp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/etowett/bared/apps/api/internal/config"
)

// newTestHostKey returns a fresh ed25519 signer to act as a server host key.
func newTestHostKey(t *testing.T) ssh.Signer {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)

	return signer
}

// writeKnownHosts writes a known_hosts file mapping addr to key and returns its
// path.
func writeKnownHosts(t *testing.T, addr string, key ssh.PublicKey) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "known_hosts")
	line := knownhosts.Line([]string{knownhosts.Normalize(addr)}, key)
	require.NoError(t, os.WriteFile(path, []byte(line+"\n"), 0o600))

	return path
}

// testSFTPServer is an in-process SSH+SFTP server. Host key verification is a
// handshake-level behaviour, so the tests exercise a real handshake rather than
// asserting on a callback in isolation.
type testSFTPServer struct {
	addr     string
	host     string
	port     int
	hostKey  ssh.Signer
	rootDir  string
	listener net.Listener
}

func startTestSFTPServer(t *testing.T, hostKey ssh.Signer, password string) *testSFTPServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)

	srv := &testSFTPServer{
		addr:     listener.Addr().String(),
		host:     "127.0.0.1",
		port:     tcpAddr.Port,
		hostKey:  hostKey,
		rootDir:  t.TempDir(),
		listener: listener,
	}

	cfg := &ssh.ServerConfig{
		PasswordCallback: func(_ ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if password != "" && string(pass) == password {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected")
		},
		PublicKeyCallback: func(_ ssh.ConnMetadata, _ ssh.PublicKey) (*ssh.Permissions, error) {
			// Any key is accepted: these tests are about the *server's*
			// identity, not the client's.
			return nil, nil
		},
	}
	cfg.AddHostKey(hostKey)

	go srv.acceptLoop(cfg)

	t.Cleanup(func() {
		//nolint:errcheck // listener close during test cleanup
		_ = listener.Close()
	})

	return srv
}

func (s *testSFTPServer) acceptLoop(cfg *ssh.ServerConfig) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.serveConn(conn, cfg)
	}
}

func (s *testSFTPServer) serveConn(conn net.Conn, cfg *ssh.ServerConfig) {
	//nolint:errcheck // best-effort cleanup in a test server
	defer conn.Close()

	sshConn, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		// A rejected host key fails here; that is the point of the test.
		return
	}
	//nolint:errcheck // best-effort cleanup in a test server
	defer sshConn.Close()

	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			//nolint:errcheck // best-effort in a test server
			_ = newChan.Reject(ssh.UnknownChannelType, "unsupported")
			continue
		}

		channel, requests, acceptErr := newChan.Accept()
		if acceptErr != nil {
			return
		}

		go func(in <-chan *ssh.Request) {
			for req := range in {
				//nolint:errcheck // best-effort in a test server
				_ = req.Reply(req.Type == "subsystem" && strings.HasSuffix(string(req.Payload), "sftp"), nil)
			}
		}(requests)

		go func(ch ssh.Channel) {
			server, srvErr := sftp.NewServer(ch, sftp.WithServerWorkingDirectory(s.rootDir))
			if srvErr != nil {
				return
			}
			//nolint:errcheck // the server returns io.EOF on client disconnect
			_ = server.Serve()
			//nolint:errcheck // best-effort cleanup
			_ = server.Close()
		}(channel)
	}
}

// storageCfg builds an SFTP storage config pointing at this test server.
func (s *testSFTPServer) storageCfg() *config.Storage {
	return &config.Storage{
		Name:     "test-sftp",
		Type:     "sftp",
		Host:     s.host,
		Port:     s.port,
		Username: "backup",
		Password: "hunter2",
		Path:     "/",
	}
}

// The headline regression for #73: with nothing configured, a server whose key
// is not in known_hosts must be REFUSED, not silently trusted.
func TestSFTP_Connect_HostKeyVerification(t *testing.T) {
	hostKey := newTestHostKey(t)
	otherKey := newTestHostKey(t)
	srv := startTestSFTPServer(t, hostKey, "hunter2")

	matchingKnownHosts := writeKnownHosts(t, srv.addr, hostKey.PublicKey())
	wrongKnownHosts := writeKnownHosts(t, srv.addr, otherKey.PublicKey())
	emptyKnownHosts := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(emptyKnownHosts, []byte{}, 0o600))

	tests := []struct {
		name string
		// mutate adjusts the base config for this case.
		mutate func(*config.Storage)
		// wantErrContains, when non-empty, is asserted on the connect error.
		wantErrContains []string
		wantConnected   bool
	}{
		{
			name: "unknown host key is rejected by default",
			mutate: func(c *config.Storage) {
				c.KnownHostsPath = emptyKnownHosts
			},
			wantErrContains: []string{
				"not found in known_hosts",
				"ssh-keyscan",
				"host_key_fingerprint",
				"insecure_skip_host_key_verify",
			},
		},
		{
			name: "missing known_hosts file is rejected with remediation",
			mutate: func(c *config.Storage) {
				c.KnownHostsPath = filepath.Join(t.TempDir(), "absent", "known_hosts")
			},
			wantErrContains: []string{"known_hosts", "unreadable", "ssh-keyscan"},
		},
		{
			name: "a different key for the same host is rejected",
			mutate: func(c *config.Storage) {
				c.KnownHostsPath = wrongKnownHosts
			},
			wantErrContains: []string{"does NOT match known_hosts", "man-in-the-middle"},
		},
		{
			name: "matching known_hosts entry connects",
			mutate: func(c *config.Storage) {
				c.KnownHostsPath = matchingKnownHosts
			},
			wantConnected: true,
		},
		{
			name: "matching pinned fingerprint connects",
			mutate: func(c *config.Storage) {
				c.HostKeyFingerprint = ssh.FingerprintSHA256(hostKey.PublicKey())
			},
			wantConnected: true,
		},
		{
			name: "mismatched pinned fingerprint is rejected",
			mutate: func(c *config.Storage) {
				c.HostKeyFingerprint = ssh.FingerprintSHA256(otherKey.PublicKey())
			},
			wantErrContains: []string{"does not match the pinned host_key_fingerprint"},
		},
		{
			name: "malformed pinned fingerprint is rejected before dialling",
			mutate: func(c *config.Storage) {
				c.HostKeyFingerprint = "SHA256:nope"
			},
			wantErrContains: []string{"invalid host_key_fingerprint"},
		},
		{
			name: "explicit insecure opt-in connects to an unverified server",
			mutate: func(c *config.Storage) {
				c.KnownHostsPath = emptyKnownHosts
				c.InsecureSkipHostKeyVerify = true
			},
			wantConnected: true,
		},
		{
			name: "no credentials configured is rejected before dialling",
			mutate: func(c *config.Storage) {
				c.KnownHostsPath = matchingKnownHosts
				c.Password = ""
			},
			wantErrContains: []string{"no credentials", "private_key_path"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := srv.storageCfg()
			tt.mutate(cfg)

			s := NewSFTP(cfg)
			t.Cleanup(s.disconnect)

			err := s.connect()

			if tt.wantConnected {
				require.NoError(t, err)
				assert.NotNil(t, s.sftpClient)
				return
			}

			require.Error(t, err)
			assert.Nil(t, s.sftpClient, "a rejected host key must not leave a live client")
			for _, want := range tt.wantErrContains {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}

// With no host key configuration at all, the default is ~/.ssh/known_hosts —
// and an unlisted server is refused. This is the behaviour change for existing
// users, so the error has to say what to do.
func TestSFTP_Connect_ZeroConfigDefaultsToKnownHosts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir on Windows

	hostKey := newTestHostKey(t)
	srv := startTestSFTPServer(t, hostKey, "hunter2")

	s := NewSFTP(srv.storageCfg())
	t.Cleanup(s.disconnect)

	err := s.connect()

	require.Error(t, err, "an unverifiable host must not be trusted by default")
	assert.Nil(t, s.sftpClient)
	assert.Contains(t, err.Error(), filepath.Join(home, ".ssh", "known_hosts"))
	assert.Contains(t, err.Error(), "ssh-keyscan")
	assert.Contains(t, err.Error(), "host_key_fingerprint")
	assert.Contains(t, err.Error(), "insecure_skip_host_key_verify")
}

// An SFTP round trip over a host-key-verified connection, to prove the new
// callback does not break the actual transfer path.
func TestSFTP_StoreRetrieve_OverVerifiedConnection(t *testing.T) {
	hostKey := newTestHostKey(t)
	srv := startTestSFTPServer(t, hostKey, "hunter2")

	cfg := srv.storageCfg()
	cfg.Path = srv.rootDir
	cfg.KnownHostsPath = writeKnownHosts(t, srv.addr, hostKey.PublicKey())

	s := NewSFTP(cfg)
	payload := []byte("streamed backup bytes")

	require.NoError(t, s.Store(t.Context(), "db/backup.tar.gz", strings.NewReader(string(payload)), int64(len(payload))))

	var buf strings.Builder
	require.NoError(t, s.Retrieve(t.Context(), "db/backup.tar.gz", &buf))
	assert.Equal(t, string(payload), buf.String())
}

// writeClientPrivateKey writes a fresh OpenSSH private key file and returns its
// path.
func writeClientPrivateKey(t *testing.T) string {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	block, err := ssh.MarshalPrivateKey(priv, "")
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "id_ed25519")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(block), 0o600))

	return path
}

// Public key auth must be usable instead of, or alongside, a password.
func TestSFTP_Connect_PublicKeyAuth(t *testing.T) {
	hostKey := newTestHostKey(t)
	srv := startTestSFTPServer(t, hostKey, "")

	cfg := srv.storageCfg()
	cfg.Password = ""
	cfg.PrivateKeyPath = writeClientPrivateKey(t)
	cfg.KnownHostsPath = writeKnownHosts(t, srv.addr, hostKey.PublicKey())

	s := NewSFTP(cfg)
	t.Cleanup(s.disconnect)

	require.NoError(t, s.connect())
	assert.NotNil(t, s.sftpClient)
}

func TestLoadPrivateKey_Errors(t *testing.T) {
	dir := t.TempDir()

	garbage := filepath.Join(dir, "garbage")
	require.NoError(t, os.WriteFile(garbage, []byte("not a key"), 0o600))

	tests := []struct {
		name            string
		path            string
		passphrase      string
		wantErrContains string
	}{
		{
			name:            "missing file",
			path:            filepath.Join(dir, "absent"),
			wantErrContains: "failed to read SFTP private key",
		},
		{
			name:            "unparseable file",
			path:            garbage,
			wantErrContains: "failed to parse SFTP private key",
		},
		{
			name:            "wrong passphrase",
			path:            garbage,
			passphrase:      "secret",
			wantErrContains: "failed to decrypt SFTP private key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadPrivateKey(tt.path, tt.passphrase)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrContains)
			// A key error must never carry the passphrase into the log.
			if tt.passphrase != "" {
				assert.NotContains(t, err.Error(), tt.passphrase)
			}
		})
	}
}

// knownhosts.Normalize keys a non-22 port as "[host]:port", so a suggested
// ssh-keyscan that dropped the port would write an entry that still never
// matches — the operator would run the fix and hit the same error.
func TestKeyscanCommand(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		keyType  string
		want     string
	}{
		{name: "default port", hostname: "backup.example.com:22", want: "ssh-keyscan -H backup.example.com"},
		{
			name:     "non-standard port is carried",
			hostname: "backup.example.com:2222",
			want:     "ssh-keyscan -p 2222 -H backup.example.com",
		},
		{
			name:     "key type is requested when known",
			hostname: "backup.example.com:22",
			keyType:  "ssh-ed25519",
			want:     "ssh-keyscan -H -t ssh-ed25519 backup.example.com",
		},
		{
			name:     "bracketed IPv6 host",
			hostname: "[2001:db8::1]:2222",
			want:     "ssh-keyscan -p 2222 -H 2001:db8::1",
		},
		{name: "no port at all", hostname: "backup.example.com", want: "ssh-keyscan -H backup.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, keyscanCommand(tt.hostname, tt.keyType))
		})
	}
}

// The remediation an operator is given for an unknown host must be runnable as
// printed, port included.
func TestSFTP_Connect_UnknownHostErrorCarriesThePort(t *testing.T) {
	hostKey := newTestHostKey(t)
	srv := startTestSFTPServer(t, hostKey, "hunter2")

	emptyKnownHosts := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(emptyKnownHosts, []byte{}, 0o600))

	cfg := srv.storageCfg()
	cfg.KnownHostsPath = emptyKnownHosts

	s := NewSFTP(cfg)
	t.Cleanup(s.disconnect)

	err := s.connect()

	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("ssh-keyscan -p %d -H %s", srv.port, srv.host))
}

func TestNormaliseFingerprint(t *testing.T) {
	key := newTestHostKey(t).PublicKey()
	canonical := ssh.FingerprintSHA256(key)
	body := strings.TrimPrefix(canonical, "SHA256:")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "canonical form", input: canonical, want: canonical},
		{name: "bare base64 body", input: body, want: canonical},
		{name: "surrounding whitespace", input: "  " + canonical + "  ", want: canonical},
		{name: "trailing padding", input: canonical + "=", want: canonical},
		{name: "empty", input: "", want: ""},
		{name: "too short", input: "SHA256:abc", want: ""},
		{name: "md5 fingerprint is too weak to pin", input: ssh.FingerprintLegacyMD5(key), want: ""},
		{name: "non base64 characters", input: "SHA256:" + strings.Repeat("!", 43), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normaliseFingerprint(tt.input))
		})
	}
}

func TestResolveKnownHostsPath(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty falls back to the ssh default", input: "", want: filepath.Join(home, ".ssh", "known_hosts")},
		{name: "tilde is expanded", input: "~/custom_hosts", want: filepath.Join(home, "custom_hosts")},
		{name: "bare tilde is the home directory", input: "~", want: home},
		{name: "absolute path is untouched", input: "/etc/bared/known_hosts", want: "/etc/bared/known_hosts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, resolveErr := resolveKnownHostsPath(tt.input)

			require.NoError(t, resolveErr)
			assert.Equal(t, tt.want, got)
		})
	}
}
