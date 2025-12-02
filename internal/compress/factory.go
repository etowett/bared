package compress

import (
	"fmt"
)

// New creates a new compressor based on the type
func New(compressType string, filename string) (Compressor, error) {
	switch compressType {
	case "tgz", "tar.gz":
		return NewTarGz(filename), nil
	default:
		return nil, fmt.Errorf("unsupported compression type: %s", compressType)
	}
}
