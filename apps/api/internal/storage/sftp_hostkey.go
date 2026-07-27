package storage

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"bared/internal/config"
	"bared/internal/util"
)

// defaultKnownHostsPath is consulted when known_hosts_path is not configured.
// It is the same file the ssh(1) client uses, so an operator who has already
// connected once needs no extra configuration.
const defaultKnownHostsPath = "~/.ssh/known_hosts"

// hostKeyCallback builds the SSH host key verification for this backend.
//
// Verification is on by default. The order is:
//
//  1. insecure_skip_host_key_verify — accept anything (opt-in, warns loudly)
//  2. host_key_fingerprint — pin exactly one key
//  3. known_hosts_path (default ~/.ssh/known_hosts) — OpenSSH known_hosts
//
// Every failure path names what to do about it: an unverified SFTP connection
// hands a MITM the credentials *and* the dump it is carrying, so failing closed
// is only useful if the operator can tell how to fix it.
func hostKeyCallback(cfg *config.Storage) (ssh.HostKeyCallback, error) {
	if cfg.InsecureSkipHostKeyVerify {
		// #nosec G106 -- explicit, documented opt-in; warned about at construction.
		return ssh.InsecureIgnoreHostKey(), nil
	}

	if fingerprint := strings.TrimSpace(cfg.HostKeyFingerprint); fingerprint != "" {
		return pinnedHostKeyCallback(fingerprint)
	}

	return knownHostsCallback(cfg.KnownHostsPath)
}

// pinnedHostKeyCallback verifies the server against a single SHA256
// fingerprint, in the "SHA256:<base64>" form ssh-keygen -l prints. The bare
// base64 body is accepted too, since it is easy to copy without the prefix.
func pinnedHostKeyCallback(fingerprint string) (ssh.HostKeyCallback, error) {
	want := normaliseFingerprint(fingerprint)
	if want == "" {
		return nil, fmt.Errorf(
			"invalid host_key_fingerprint %q: expected a SHA256 fingerprint like "+
				"\"SHA256:n3s1Xb…\" (get it with: ssh-keyscan -t ed25519 HOST | ssh-keygen -lf -)",
			fingerprint)
	}

	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		got := ssh.FingerprintSHA256(key)
		if subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1 {
			return nil
		}
		return fmt.Errorf(
			"SFTP host key does not match the pinned host_key_fingerprint: server offered %s (%s), expected %s; "+
				"either the server's key changed or the connection is being intercepted — "+
				"update host_key_fingerprint only if you know why it changed",
			got, key.Type(), want)
	}, nil
}

// normaliseFingerprint returns the canonical "SHA256:<base64>" form, or "" if
// the value cannot be one.
func normaliseFingerprint(fingerprint string) string {
	body := strings.TrimSpace(fingerprint)
	if after, ok := strings.CutPrefix(body, "SHA256:"); ok {
		body = after
	}
	body = strings.TrimRight(body, "=")

	// A SHA256 digest is 32 bytes, which is 43 unpadded base64 characters.
	// Anything else is a typo or an MD5 fingerprint, which is too weak to pin.
	if len(body) != 43 {
		return ""
	}
	for _, r := range body {
		isBase64 := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') || r == '+' || r == '/'
		if !isBase64 {
			return ""
		}
	}

	return "SHA256:" + body
}

// knownHostsCallback verifies the server against an OpenSSH known_hosts file.
func knownHostsCallback(path string) (ssh.HostKeyCallback, error) {
	resolved, err := resolveKnownHostsPath(path)
	if err != nil {
		return nil, err
	}

	if _, statErr := os.Stat(resolved); statErr != nil {
		return nil, fmt.Errorf(
			"SFTP known_hosts file %q is unreadable: %w; %s",
			resolved, statErr, remediation(resolved))
	}

	inner, err := knownhosts.New(resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to load SFTP known_hosts file %q: %w", resolved, err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := inner(hostname, remote, key)
		if err == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) {
			return describeKeyError(keyErr, resolved, hostname, key)
		}

		var revokedErr *knownhosts.RevokedError
		if errors.As(err, &revokedErr) {
			return fmt.Errorf("SFTP host key for %s is marked revoked in %q; refusing to connect", hostname, resolved)
		}

		return fmt.Errorf("SFTP host key verification failed for %s: %w", hostname, err)
	}, nil
}

// describeKeyError turns knownhosts' terse errors into something an operator
// can act on without reading this file. The three cases are genuinely
// different: an unknown host is a setup step, a key of an unlisted type is a
// configuration mismatch, and a real mismatch may be an attack.
func describeKeyError(keyErr *knownhosts.KeyError, path, hostname string, key ssh.PublicKey) error {
	if len(keyErr.Want) == 0 {
		return fmt.Errorf(
			"SFTP host key for %s not found in known_hosts (%q); "+
				"the server offered %s %s — add it after verifying it out of band "+
				"(ssh-keyscan -H %s >> %s), or set host_key_fingerprint, "+
				"or set insecure_skip_host_key_verify: true to accept any key (unsafe)",
			hostname, path, key.Type(), ssh.FingerprintSHA256(key),
			knownHostsScanTarget(hostname), path)
	}

	sameType := false
	types := make([]string, 0, len(keyErr.Want))
	for _, want := range keyErr.Want {
		types = append(types, want.Key.Type())
		if want.Key.Type() == key.Type() {
			sameType = true
		}
	}

	if !sameType {
		return fmt.Errorf(
			"SFTP host key type mismatch for %s: the server offered a %s key but known_hosts (%q) only lists %s; "+
				"add the server's %s key (ssh-keyscan -H -t %s %s >> %s)",
			hostname, key.Type(), path, strings.Join(types, ", "),
			key.Type(), key.Type(), knownHostsScanTarget(hostname), path)
	}

	return fmt.Errorf(
		"SFTP host key for %s does NOT match known_hosts (%q): server offered %s %s; "+
			"this is either a rebuilt server or a man-in-the-middle — "+
			"verify the new key out of band before editing %q",
		hostname, path, key.Type(), ssh.FingerprintSHA256(key), path)
}

// knownHostsScanTarget strips the port from a "host:port" so the suggested
// ssh-keyscan command is copy-pasteable.
func knownHostsScanTarget(hostname string) string {
	if host, _, err := net.SplitHostPort(hostname); err == nil {
		return host
	}
	return hostname
}

func remediation(path string) string {
	return fmt.Sprintf(
		"add the server's key with `ssh-keyscan -H HOST >> %s`, point known_hosts_path at an existing file, "+
			"pin host_key_fingerprint, or set insecure_skip_host_key_verify: true to accept any host key (unsafe)",
		path)
}

// resolveKnownHostsPath expands "" to the default and a leading ~ to the
// current user's home directory. YAML configs are written by humans, and "~"
// is what they write.
func resolveKnownHostsPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		path = defaultKnownHostsPath
	}

	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf(
				"cannot expand %q: no home directory for this process (%w); "+
					"set known_hosts_path to an absolute path", path, err)
		}
		path = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(path, "~"), "/"))
	}

	return path, nil
}

// sshAuthMethods builds the authentication methods for an SFTP connection.
// Public key auth is offered first when configured, with the password as a
// fallback, matching what ssh(1) does.
func sshAuthMethods(cfg *config.Storage) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if keyPath := strings.TrimSpace(cfg.PrivateKeyPath); keyPath != "" {
		signer, err := loadPrivateKey(keyPath, cfg.PrivateKeyPassphrase)
		if err != nil {
			return nil, err
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	if cfg.Password != "" {
		methods = append(methods, ssh.Password(cfg.Password))
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("SFTP storage %q has no credentials: set password or private_key_path", cfg.Name)
	}

	return methods, nil
}

// loadPrivateKey reads and parses an OpenSSH private key. Errors deliberately
// carry the path and never the key material or the passphrase.
func loadPrivateKey(path, passphrase string) (ssh.Signer, error) {
	// #nosec G304 -- the path comes from the operator's own config, not a request.
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read SFTP private key %q: %w", path, err)
	}

	if passphrase != "" {
		signer, parseErr := ssh.ParsePrivateKeyWithPassphrase(pem, []byte(passphrase))
		if parseErr != nil {
			return nil, fmt.Errorf("failed to decrypt SFTP private key %q: %w", path, parseErr)
		}
		return signer, nil
	}

	signer, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		var passErr *ssh.PassphraseMissingError
		if errors.As(err, &passErr) {
			return nil, fmt.Errorf(
				"SFTP private key %q is passphrase-protected: set private_key_passphrase", path)
		}
		return nil, fmt.Errorf("failed to parse SFTP private key %q: %w", path, err)
	}

	return signer, nil
}

// warnIfHostKeyVerificationDisabled makes the insecure opt-in impossible to
// miss in the log. It is called once per backend construction rather than per
// connection so it lands at daemon startup without flooding.
func warnIfHostKeyVerificationDisabled(cfg *config.Storage) {
	if cfg == nil || !cfg.InsecureSkipHostKeyVerify {
		return
	}

	util.GetLogger().WarnS(
		"INSECURE: SFTP host key verification is disabled — this connection can be intercepted, "+
			"exposing the SFTP credentials and every backup sent over it. "+
			"Remove insecure_skip_host_key_verify and use known_hosts_path or host_key_fingerprint.",
		"component", "storage",
		"storage", cfg.Name,
		"host", cfg.Host)
}
