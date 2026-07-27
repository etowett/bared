package database

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/etowett/bared/apps/api/internal/config"
	"github.com/etowett/bared/apps/api/internal/util"
)

const sanitizeTestPassword = "Xk9pQ2mZvR7t" // deliberately contains none of the redactor keywords

// TestSanitizeArgs_DelegatesToSharedRedactor checks the per-engine helpers
// against the argv they really build. The rule itself lives in
// util.RedactArgs so the same masking reaches util.ExecuteCommand's error
// paths (issue #133).
func TestSanitizeArgs_DelegatesToSharedRedactor(t *testing.T) {
	conn := &config.Connection{
		Type:     "mysql",
		Host:     "db",
		Port:     3306,
		User:     "root",
		Password: sanitizeTestPassword,
		Database: "mydb",
	}
	mysql := NewMySQL(conn, nil, nil)

	redisConn := &config.Connection{
		Type:     "redis",
		Host:     "db",
		Port:     6379,
		Password: sanitizeTestPassword,
	}
	redis := NewRedis(redisConn)

	tests := []struct {
		name        string
		got         []string
		wantMasked  string
		wantVisible []string
	}{
		{
			name:        "mysql dump args",
			got:         mysql.sanitizeArgs(mysql.buildDumpArgs()),
			wantMasked:  "--password=" + util.Redacted,
			wantVisible: []string{"--host=db", "--user=root", "mydb"},
		},
		{
			name:        "mysql restore args",
			got:         mysql.sanitizeArgs(mysql.buildRestoreArgs()),
			wantMasked:  "--password=" + util.Redacted,
			wantVisible: []string{"--binary-mode=1"},
		},
		{
			name:        "redis dump args",
			got:         redis.sanitizeArgs(redis.buildDumpArgs("/tmp/dump.rdb")),
			wantMasked:  util.Redacted,
			wantVisible: []string{"-a", "-p", "6379", "--rdb"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			joined := strings.Join(tt.got, " ")
			assert.NotContains(t, joined, sanitizeTestPassword)
			assert.Contains(t, joined, tt.wantMasked)
			for _, visible := range tt.wantVisible {
				assert.Contains(t, tt.got, visible)
			}
		})
	}
}
