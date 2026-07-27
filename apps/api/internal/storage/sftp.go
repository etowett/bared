package storage

import (
	"context"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"bared/internal/config"
	"bared/internal/util"
)

// sshDialTimeout bounds the TCP connect and SSH handshake so an unreachable
// backup host cannot wedge a job indefinitely.
const sshDialTimeout = 30 * time.Second

// SFTP implements Storage for SFTP servers
type SFTP struct {
	cfg        *config.Storage
	sshClient  *ssh.Client
	sftpClient *sftp.Client
}

// NewSFTP creates a new SFTP storage backend
func NewSFTP(cfg *config.Storage) *SFTP {
	warnIfHostKeyVerificationDisabled(cfg)
	return &SFTP{cfg: cfg}
}

// Name returns the storage name
func (s *SFTP) Name() string {
	return s.cfg.Name
}

// Validate checks if SFTP is accessible
func (s *SFTP) Validate(_ context.Context) error {
	if err := s.connect(); err != nil {
		return err
	}
	defer s.disconnect()

	// Try to stat the path
	_, err := s.sftpClient.Stat(s.cfg.Path)
	if err != nil {
		return fmt.Errorf("failed to access SFTP path: %w", err)
	}

	return nil
}

// Store writes data from reader to SFTP
func (s *SFTP) Store(ctx context.Context, filePath string, r io.Reader, _ int64) error {
	if err := s.connect(); err != nil {
		return err
	}
	defer s.disconnect()

	fullPath := path.Join(s.cfg.Path, filePath)

	// Create directory if it doesn't exist
	dir := path.Dir(fullPath)
	if err := s.sftpClient.MkdirAll(dir); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Upload with retry
	err := util.Retry(ctx, util.DefaultRetryConfig(), func() error {
		f, err := s.sftpClient.Create(fullPath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer func() {
			//nolint:errcheck // Error closing SFTP file during cleanup is not critical
			_ = f.Close()
		}()

		_, err = io.Copy(f, r)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to upload via SFTP: %w", err)
	}

	return nil
}

// Retrieve reads data from SFTP into writer
func (s *SFTP) Retrieve(_ context.Context, filePath string, w io.Writer) error {
	if err := s.connect(); err != nil {
		return err
	}
	defer s.disconnect()

	fullPath := path.Join(s.cfg.Path, filePath)

	f, err := s.sftpClient.Open(fullPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		//nolint:errcheck // Error closing SFTP file during cleanup is not critical
		_ = f.Close()
	}()

	_, err = io.Copy(w, f)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	return nil
}

// List returns all backup files from SFTP
func (s *SFTP) List(_ context.Context) ([]*BackupInfo, error) {
	if err := s.connect(); err != nil {
		return nil, err
	}
	defer s.disconnect()

	var backups []*BackupInfo

	walker := s.sftpClient.Walk(s.cfg.Path)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			continue
		}

		info := walker.Stat()
		if info.IsDir() {
			continue
		}

		backups = append(backups, &BackupInfo{
			Path:         walker.Path(),
			Size:         info.Size(),
			LastModified: info.ModTime(),
			StorageName:  s.cfg.Name,
		})
	}

	return backups, nil
}

// Delete removes a backup from SFTP
func (s *SFTP) Delete(_ context.Context, filePath string) error {
	if err := s.connect(); err != nil {
		return err
	}
	defer s.disconnect()

	fullPath := path.Join(s.cfg.Path, filePath)

	if err := s.sftpClient.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// connect establishes SSH and SFTP connection
func (s *SFTP) connect() error {
	if s.sftpClient != nil {
		return nil
	}

	// Verify the server's host key. Without this an on-path attacker gets both
	// the SFTP credentials and every byte of the dump. See sftp_hostkey.go for
	// the precedence between known_hosts, a pinned fingerprint, and the
	// insecure opt-in.
	hostKey, err := hostKeyCallback(s.cfg)
	if err != nil {
		return err
	}

	auth, err := sshAuthMethods(s.cfg)
	if err != nil {
		return err
	}

	// Configure SSH client
	config := &ssh.ClientConfig{
		User:            s.cfg.Username,
		Auth:            auth,
		HostKeyCallback: hostKey,
		Timeout:         sshDialTimeout,
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH: %w", err)
	}

	// Create SFTP client
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		//nolint:errcheck // Error closing SSH client during error handling is not critical
		_ = sshClient.Close()
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}

	s.sshClient = sshClient
	s.sftpClient = sftpClient

	return nil
}

// disconnect closes SSH and SFTP connections
func (s *SFTP) disconnect() {
	if s.sftpClient != nil {
		//nolint:errcheck // Error closing SFTP client during cleanup is not critical
		_ = s.sftpClient.Close()
		s.sftpClient = nil
	}
	if s.sshClient != nil {
		//nolint:errcheck // Error closing SSH client during cleanup is not critical
		_ = s.sshClient.Close()
		s.sshClient = nil
	}
}

// Exists checks if a backup file exists in SFTP
func (s *SFTP) Exists(_ context.Context, filePath string) (bool, error) {
	if err := s.connect(); err != nil {
		return false, err
	}
	defer s.disconnect()

	fullPath := path.Join(s.cfg.Path, filePath)

	_, err := s.sftpClient.Stat(fullPath)
	if err != nil {
		// Check if it's a "not exist" error
		if err.Error() == "file does not exist" || err.Error() == "no such file" {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}

	return true, nil
}

// GetInfo returns metadata about a backup file in SFTP
func (s *SFTP) GetInfo(_ context.Context, filePath string) (*BackupInfo, error) {
	if err := s.connect(); err != nil {
		return nil, err
	}
	defer s.disconnect()

	fullPath := path.Join(s.cfg.Path, filePath)

	info, err := s.sftpClient.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return &BackupInfo{
		Path:         filePath,
		Size:         info.Size(),
		LastModified: info.ModTime(),
		StorageName:  s.cfg.Name,
	}, nil
}
