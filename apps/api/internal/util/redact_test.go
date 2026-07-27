package util

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const secretValue = "Xk9pQ2mZvR7t" // deliberately contains none of the redactor keywords

func TestIsSecretFlagName(t *testing.T) {
	tests := []struct {
		name string
		flag string
		want bool
	}{
		{name: "mysql long password", flag: "--password", want: true},
		{name: "redis long pass", flag: "--pass", want: true},
		{name: "redis auth", flag: "--auth", want: true},
		{name: "redis short auth", flag: "-a", want: true},
		{name: "postgres env var", flag: "PGPASSWORD", want: true},
		{name: "generic secret", flag: "--client-secret", want: true},
		{name: "generic token", flag: "--api-token", want: true},
		{name: "generic key", flag: "--access-key-id", want: true},
		{name: "mixed case", flag: "--Password", want: true},
		{name: "host is not a secret", flag: "--host", want: false},
		{name: "user is not a secret", flag: "--user", want: false},
		{name: "redis port short flag", flag: "-p", want: false},
		{name: "negated boolean takes no value", flag: "--no-password", want: false},
		{name: "skip boolean takes no value", flag: "--skip-password", want: false},
		{name: "empty", flag: "--", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsSecretFlagName(tt.flag))
		})
	}
}

func TestRedactArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "mysqldump attached password",
			args: []string{"--host=db", "--user=root", "--password=" + secretValue, "mydb"},
			want: []string{"--host=db", "--user=root", "--password=" + Redacted, "mydb"},
		},
		{
			name: "separate token password",
			args: []string{"--host", "db", "--password", secretValue, "mydb"},
			want: []string{"--host", "db", "--password", Redacted, "mydb"},
		},
		{
			name: "redis-cli auth short flag",
			args: []string{"-h", "db", "-p", "6379", "-a", secretValue, "--rdb", "/tmp/x.rdb"},
			want: []string{"-h", "db", "-p", "6379", "-a", Redacted, "--rdb", "/tmp/x.rdb"},
		},
		{
			name: "redis-cli long pass",
			args: []string{"-h", "db", "--pass", secretValue, "PING"},
			want: []string{"-h", "db", "--pass", Redacted, "PING"},
		},
		{
			name: "postgres env-style token",
			args: []string{"PGPASSWORD=" + secretValue, "psql"},
			want: []string{"PGPASSWORD=" + Redacted, "psql"},
		},
		{
			name: "pg_dump argv is untouched and --no-password keeps its neighbour",
			args: []string{"--host=db", "--username=postgres", "--no-password", "--no-owner", "mydb"},
			want: []string{"--host=db", "--username=postgres", "--no-password", "--no-owner", "mydb"},
		},
		{
			name: "--no-password followed by the database name",
			args: []string{"--no-password", "production", "-c", "SELECT 1;"},
			want: []string{"--no-password", "production", "-c", "SELECT 1;"},
		},
		{
			// A password may itself start with '-', so the token after a
			// secret flag is masked whatever it looks like.
			name: "password that looks like a flag",
			args: []string{"-h", "db", "-a", "-Xk9pQ2mZ", "--rdb", "/tmp/x.rdb"},
			want: []string{"-h", "db", "-a", Redacted, "--rdb", "/tmp/x.rdb"},
		},
		{
			name: "attached short password from additional_args",
			args: []string{"--host=db", "-p" + secretValue, "mydb"},
			want: []string{"--host=db", "-p" + Redacted, "mydb"},
		},
		{
			name: "generic secret/token/key flags",
			args: []string{"--client-secret=" + secretValue, "--api-token", secretValue, "--access-key=" + secretValue},
			want: []string{"--client-secret=" + Redacted, "--api-token", Redacted, "--access-key=" + Redacted},
		},
		{
			name: "trailing secret flag with no value",
			args: []string{"--host=db", "--password"},
			want: []string{"--host=db", "--password"},
		},
		{
			name: "secret embedded in a single shell token",
			args: []string{"-c", "mysqldump --host=db --password=" + secretValue + " | gzip"},
			want: []string{"-c", "mysqldump --host=db --password=" + Redacted + " | gzip"},
		},
		{name: "empty slice", args: []string{}, want: []string{}},
		{name: "nil slice", args: nil, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactArgs(tt.args)
			assert.Equal(t, tt.want, got)
			for _, arg := range got {
				assert.NotContains(t, arg, secretValue)
			}
		})
	}
}

func TestRedactArgs_DoesNotMutateInput(t *testing.T) {
	args := []string{"--password=" + secretValue}
	_ = RedactArgs(args)
	assert.Equal(t, "--password="+secretValue, args[0], "input argv must be left usable for exec")
}

func TestRedactSecrets(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "the reproduction from issue #133, verbatim",
			text:        "command failed: /usr/bin/false [--host=db --user=root --password=SuperSecret123] (exit code 1): exit status 1",
			wantContain: []string{"--host=db", "--user=root", "--password=" + Redacted, "exit code 1"},
			wantAbsent:  []string{"SuperSecret123"},
		},
		{
			name:        "attached long password",
			text:        fmt.Sprintf("command failed: mysqldump [--host=db --password=%s] (exit code 1)", secretValue),
			wantContain: []string{"--host=db", "--password=" + Redacted},
			wantAbsent:  []string{secretValue},
		},
		{
			name:        "separate token form",
			text:        "command failed: mysql [--host db --password " + secretValue + "]",
			wantContain: []string{"--host db", "--password " + Redacted},
			wantAbsent:  []string{secretValue},
		},
		{
			name:        "redis-cli auth flag",
			text:        "command failed: redis-cli [-h db -p 6379 -a " + secretValue + " PING]",
			wantContain: []string{"-h db", "-p 6379", "-a " + Redacted},
			wantAbsent:  []string{secretValue},
		},
		{
			name:        "password that looks like a flag",
			text:        "command failed: redis-cli [-h db -p 6379 -a -Xk9pQ2mZ --rdb /tmp/x.rdb]",
			wantContain: []string{"-h db", "-a " + Redacted},
			wantAbsent:  []string{"-Xk9pQ2mZ"},
		},
		{
			name:        "attached short password",
			text:        "command failed: mysqldump [--host=db -p" + secretValue + " mydb]",
			wantContain: []string{"--host=db", "-p" + Redacted, "mydb"},
			wantAbsent:  []string{secretValue},
		},
		{
			name:        "postgres env variable echoed in stderr",
			text:        "psql: could not connect (PGPASSWORD=" + secretValue + ")",
			wantContain: []string{"PGPASSWORD=" + Redacted},
			wantAbsent:  []string{secretValue},
		},
		{
			name:        "generic secret and token flags",
			text:        "aws [--secret-access-key=" + secretValue + " --session-token " + secretValue + "]",
			wantContain: []string{"--secret-access-key=" + Redacted, "--session-token " + Redacted},
			wantAbsent:  []string{secretValue},
		},
		{
			name:        "non-secret text is preserved",
			text:        "pg_dump [--host=db --username=postgres --no-password --no-owner mydb]: exit status 1",
			wantContain: []string{"--host=db", "--username=postgres", "--no-password --no-owner mydb", "exit status 1"},
		},
		{
			name:        "psql --set assignment is preserved",
			text:        "psql [--no-password --set ON_ERROR_STOP=on mydb]",
			wantContain: []string{"--set ON_ERROR_STOP=on", "mydb"},
		},
		{
			name:        "empty text",
			text:        "",
			wantContain: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactSecrets(tt.text)
			for _, want := range tt.wantContain {
				assert.Contains(t, got, want)
			}
			for _, absent := range tt.wantAbsent {
				assert.NotContains(t, got, absent)
			}
		})
	}
}

func TestRedactSecrets_MatchesFormattedArgv(t *testing.T) {
	// The persisted error strings were produced by fmt's %v of an argv slice,
	// so the text redactor has to cope with the bracketed form.
	args := []string{"--host=db", "--user=root", "--password=" + secretValue}
	text := fmt.Sprintf("command failed: mysqldump %v (exit code 1)", args)

	got := RedactSecrets(text)

	assert.NotContains(t, got, secretValue)
	assert.Contains(t, got, "--password="+Redacted)
}

func TestRedactErr(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		assert.NoError(t, RedactErr(nil))
	})

	t.Run("clean error is returned unchanged", func(t *testing.T) {
		orig := errors.New("exit status 1")
		assert.Same(t, orig, RedactErr(orig))
	})

	t.Run("secret is masked and the chain is preserved", func(t *testing.T) {
		sentinel := errors.New("sentinel")
		orig := fmt.Errorf("mysqldump [--password=%s]: %w", secretValue, sentinel)

		redacted := RedactErr(orig)

		require.Error(t, redacted)
		assert.NotContains(t, redacted.Error(), secretValue)
		assert.Contains(t, redacted.Error(), "--password="+Redacted)
		assert.True(t, errors.Is(redacted, sentinel), "errors.Is must still reach the cause")
	})
}

func TestRedactSecrets_LeavesNoSecretSuffix(t *testing.T) {
	// The value pattern is greedy on purpose: it must swallow a delimiter
	// rather than leave the tail of a password behind.
	for _, password := range []string{"pa]ss", "pass]", `pa"ss`, "pass,word", "p@ss w0rd"} {
		text := "cmd [--password=" + password + "]"
		got := RedactSecrets(text)
		first := strings.SplitN(password, " ", 2)[0]
		assert.NotContains(t, got, first, "password fragment survived redaction: %q", got)
	}
}
