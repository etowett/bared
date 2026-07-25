package database

import (
	"fmt"

	"bared/internal/config"
)

// NewDumper creates a new Dumper based on the target configuration
func NewDumper(target *config.Target) (Dumper, error) {
	switch target.Conn.Type {
	case "mysql":
		return NewMySQL(target.Conn, target.ExcludeTables, target.AdditionalArgs), nil
	case "postgres":
		return NewPostgres(target.Conn, target.ExcludeTables, target.AdditionalArgs), nil
	case "redis":
		return NewRedis(target.Conn), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", target.Conn.Type)
	}
}

// NewRestorer creates a new Restorer based on the target configuration
func NewRestorer(target *config.Target) (Restorer, error) {
	switch target.Conn.Type {
	case "mysql":
		return NewMySQL(target.Conn, target.ExcludeTables, target.AdditionalArgs), nil
	case "postgres":
		return NewPostgres(target.Conn, target.ExcludeTables, target.AdditionalArgs), nil
	case "redis":
		return NewRedis(target.Conn), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", target.Conn.Type)
	}
}
