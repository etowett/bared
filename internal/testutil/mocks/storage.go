package mocks

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"bared/internal/storage"
)

// MockStorage is a mock implementation of the storage.Storage interface
type MockStorage struct {
	mu sync.Mutex

	// Recorded calls for verification
	StoreCalls    []StoreCall
	RetrieveCalls []RetrieveCall
	ListCalls     int
	DeleteCalls   []string
	ValidateCalls int

	// Configurable responses
	StoreFunc    func(ctx context.Context, path string, r io.Reader, size int64) error
	RetrieveFunc func(ctx context.Context, path string, w io.Writer) error
	ListFunc     func(ctx context.Context) ([]*storage.BackupInfo, error)
	DeleteFunc   func(ctx context.Context, path string) error
	ValidateFunc func(ctx context.Context) error

	// Simple response configuration
	StorageNameValue string
	StoreError       error
	RetrieveError    error
	RetrieveData     map[string]string // path -> data to return
	ListError        error
	ListBackups      []*storage.BackupInfo
	DeleteError      error
	ValidateError    error

	// In-memory storage for Store/Retrieve operations
	StoredFiles map[string][]byte
}

// StoreCall records details about a Store call
type StoreCall struct {
	Path string
	Size int64
	Data string // Captured data
}

// RetrieveCall records details about a Retrieve call
type RetrieveCall struct {
	Path string
}

// NewMockStorage creates a new mock storage
func NewMockStorage(name string) *MockStorage {
	return &MockStorage{
		StorageNameValue: name,
		StoreCalls:       make([]StoreCall, 0),
		RetrieveCalls:    make([]RetrieveCall, 0),
		DeleteCalls:      make([]string, 0),
		RetrieveData:     make(map[string]string),
		StoredFiles:      make(map[string][]byte),
	}
}

// Store mocks the storage.Store method
func (m *MockStorage) Store(ctx context.Context, path string, r io.Reader, size int64) error {
	// Capture data for verification
	data := &strings.Builder{}
	if r != nil {
		io.Copy(data, r)
	}

	m.mu.Lock()
	m.StoreCalls = append(m.StoreCalls, StoreCall{
		Path: path,
		Size: size,
		Data: data.String(),
	})
	// Store in memory for later retrieval
	m.StoredFiles[path] = []byte(data.String())
	m.mu.Unlock()

	// Use custom function if provided
	if m.StoreFunc != nil {
		return m.StoreFunc(ctx, path, strings.NewReader(data.String()), size)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Return configured error
	return m.StoreError
}

// Retrieve mocks the storage.Retrieve method
func (m *MockStorage) Retrieve(ctx context.Context, path string, w io.Writer) error {
	m.mu.Lock()
	m.RetrieveCalls = append(m.RetrieveCalls, RetrieveCall{
		Path: path,
	})
	m.mu.Unlock()

	// Use custom function if provided
	if m.RetrieveFunc != nil {
		return m.RetrieveFunc(ctx, path, w)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Check if file was stored previously
	m.mu.Lock()
	storedData, exists := m.StoredFiles[path]
	m.mu.Unlock()

	if exists {
		if _, err := w.Write(storedData); err != nil {
			return err
		}
		return nil
	}

	// Check if retrieve data is configured
	if data, ok := m.RetrieveData[path]; ok {
		if _, err := io.WriteString(w, data); err != nil {
			return err
		}
		return nil
	}

	// Return configured error
	if m.RetrieveError != nil {
		return m.RetrieveError
	}

	// Default: file not found
	return fmt.Errorf("backup file not found: %s", path)
}

// List mocks the storage.List method
func (m *MockStorage) List(ctx context.Context) ([]*storage.BackupInfo, error) {
	m.mu.Lock()
	m.ListCalls++
	m.mu.Unlock()

	// Use custom function if provided
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Return configured error
	if m.ListError != nil {
		return nil, m.ListError
	}

	// Return configured backups
	return m.ListBackups, nil
}

// Delete mocks the storage.Delete method
func (m *MockStorage) Delete(ctx context.Context, path string) error {
	m.mu.Lock()
	m.DeleteCalls = append(m.DeleteCalls, path)
	// Remove from in-memory storage
	delete(m.StoredFiles, path)
	m.mu.Unlock()

	// Use custom function if provided
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, path)
	}

	// Check for context cancellation (handle nil context)
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	// Return configured error
	return m.DeleteError
}

// Name mocks the storage.Name method
func (m *MockStorage) Name() string {
	if m.StorageNameValue == "" {
		return "mock-storage"
	}
	return m.StorageNameValue
}

// Validate mocks the storage.Validate method
func (m *MockStorage) Validate(ctx context.Context) error {
	m.mu.Lock()
	m.ValidateCalls++
	m.mu.Unlock()

	// Use custom function if provided
	if m.ValidateFunc != nil {
		return m.ValidateFunc(ctx)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Return configured error
	return m.ValidateError
}

// Reset clears all recorded calls
func (m *MockStorage) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StoreCalls = make([]StoreCall, 0)
	m.RetrieveCalls = make([]RetrieveCall, 0)
	m.DeleteCalls = make([]string, 0)
	m.ListCalls = 0
	m.ValidateCalls = 0
	m.StoredFiles = make(map[string][]byte)
}

// GetLastStoreCall returns the last Store call, or nil if none
func (m *MockStorage) GetLastStoreCall() *StoreCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.StoreCalls) == 0 {
		return nil
	}
	return &m.StoreCalls[len(m.StoreCalls)-1]
}

// GetLastRetrieveCall returns the last Retrieve call, or nil if none
func (m *MockStorage) GetLastRetrieveCall() *RetrieveCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.RetrieveCalls) == 0 {
		return nil
	}
	return &m.RetrieveCalls[len(m.RetrieveCalls)-1]
}

// AssertStoreCalledWith checks if Store was called with specific path
func (m *MockStorage) AssertStoreCalledWith(path string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, call := range m.StoreCalls {
		if call.Path == path {
			return true
		}
	}
	return false
}

// AssertRetrieveCalledWith checks if Retrieve was called with specific path
func (m *MockStorage) AssertRetrieveCalledWith(path string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, call := range m.RetrieveCalls {
		if call.Path == path {
			return true
		}
	}
	return false
}

// AssertDeleteCalledWith checks if Delete was called with specific path
func (m *MockStorage) AssertDeleteCalledWith(path string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, call := range m.DeleteCalls {
		if call == path {
			return true
		}
	}
	return false
}

// AddBackupToList adds a backup to the list that will be returned by List()
func (m *MockStorage) AddBackupToList(path string, size int64, modTime time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ListBackups == nil {
		m.ListBackups = make([]*storage.BackupInfo, 0)
	}

	m.ListBackups = append(m.ListBackups, &storage.BackupInfo{
		Path:         path,
		Size:         size,
		LastModified: modTime,
		StorageName:  m.StorageNameValue,
	})
}

// SetRetrieveData sets the data that will be returned when Retrieve is called for a specific path
func (m *MockStorage) SetRetrieveData(path, data string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.RetrieveData[path] = data
}
