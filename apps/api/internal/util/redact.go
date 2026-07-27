package util

import (
	"regexp"
	"strings"
)

// Redacted is the placeholder substituted for any value judged to be a secret.
// The flag name is always preserved: an operator debugging a failed dump needs
// to see that `--password` was passed, just not what it was.
const Redacted = "***REDACTED***"

// secretKeywords are matched as substrings against a flag or environment
// variable name (lowercased, leading dashes stripped). "pass" covers
// --password, --passwd and redis-cli's --pass; "key" covers PGPASSKEY-style
// names and generic --api-key/--access-key flags.
var secretKeywords = []string{"pass", "pwd", "secret", "token", "credential", "key", "auth"}

// shortSecretFlags are single-letter flags that own the *following* token.
// `-a` is redis-cli's auth flag.
//
// `-p` is deliberately absent here: redis-cli uses `-p 6379` for the port, and
// no code path passes a MySQL password as a separate `-p` token (mysql.go
// builds `--password=`). Including it would redact every Redis port for no
// security gain.
var shortSecretFlags = map[string]bool{"a": true}

// attachedShortSecretFlags are single-letter flags whose value is glued to
// them: `-phunter2`. MySQL's `-p<password>` is the classic, and it can reach
// argv through a target's user-supplied `additional_args`.
//
// `p` is included here even though redis-cli's attached `-p6379` is a port:
// masking a port we never build ourselves is a cheap price for not leaking a
// hand-written MySQL password.
var attachedShortSecretFlags = map[string]bool{"a": true, "p": true}

// isAttachedShortSecret reports whether arg is a single-dash short flag with
// its secret value glued on, and returns the flag itself (`-p`).
func isAttachedShortSecret(arg string) (string, bool) {
	if len(arg) < 3 || arg[0] != '-' || arg[1] == '-' {
		return "", false
	}
	letter := strings.ToLower(arg[1:2])
	if !attachedShortSecretFlags[letter] {
		return "", false
	}
	return arg[:2], true
}

// IsSecretFlagName reports whether a flag or environment variable name is
// expected to carry a secret value.
func IsSecretFlagName(name string) bool {
	base := strings.ToLower(strings.TrimLeft(name, "-"))
	if base == "" {
		return false
	}

	// Negated booleans never carry a value — pg_dump's `--no-password` is
	// followed by the *next* flag (or the database name), which must survive.
	if strings.HasPrefix(base, "no-") || strings.HasPrefix(base, "skip-") {
		return false
	}

	if len(base) == 1 {
		return shortSecretFlags[base]
	}

	for _, keyword := range secretKeywords {
		if strings.Contains(base, keyword) {
			return true
		}
	}
	return false
}

// RedactArgs returns a copy of args with secret values masked. It handles the
// shapes the database engines actually build:
//
//	--password=hunter2   → --password=***REDACTED***   (mysqldump, mysql)
//	--pass hunter2       → --pass ***REDACTED***       (redis-cli long form)
//	-a hunter2           → -a ***REDACTED***           (redis-cli auth)
//	-phunter2            → -p***REDACTED***            (attached short form)
//	PGPASSWORD=hunter2   → PGPASSWORD=***REDACTED***   (env-style token)
//
// plus any flag whose name contains password/secret/token/key.
//
// Never format a raw argv slice into an error or a log line — run it through
// this first. See internal/util/shell.go for the enforcement point.
func RedactArgs(args []string) []string {
	if args == nil {
		return nil
	}

	out := make([]string, len(args))
	redactNext := false

	for i, arg := range args {
		if redactNext {
			out[i] = Redacted
			redactNext = false
			continue
		}

		// Attached short flags first: in `-phunter2=x` the '=' belongs to the
		// password, so splitting on it would expose the part before it.
		if flag, ok := isAttachedShortSecret(arg); ok {
			out[i] = flag + Redacted
			continue
		}

		if name, _, found := strings.Cut(arg, "="); found && IsSecretFlagName(name) {
			out[i] = name + "=" + Redacted
			continue
		}

		// Separate-token form. The following token is masked unconditionally:
		// a password may itself begin with '-', and IsSecretFlagName has
		// already excluded the booleans (--no-*/--skip-*) that own no value.
		if strings.HasPrefix(arg, "-") && IsSecretFlagName(arg) && i+1 < len(args) {
			redactNext = true
		}

		// A single token can also embed a whole command line — `sh -c "…
		// --password=…"`. Run the text redactor over what's left.
		out[i] = RedactSecrets(arg)
	}

	return out
}

// Value patterns are deliberately greedy (\S+): swallowing an adjacent
// delimiter such as the closing bracket of a formatted argv slice is far
// better than leaving a trailing character of a password behind.
var (
	// name=value, with or without leading dashes (covers PGPASSWORD=…).
	assignedSecretRE = regexp.MustCompile(`(-{0,2}[A-Za-z][A-Za-z0-9_.-]*)=(\S+)`)
	// -flag value / --flag value, anchored on a boundary so we don't match
	// mid-word. The value may itself start with '-': passwords do.
	spacedSecretRE = regexp.MustCompile(`(^|[\s\[(])(-{1,2}[A-Za-z][A-Za-z0-9_.-]*)(\s+)(\S+)`)
	// -phunter2 — short flag with the value glued on.
	attachedSecretRE = regexp.MustCompile(`(^|[\s\[(])(-[A-Za-z])([^\s=]\S*)`)
)

// RedactSecrets masks secret values in free-form text: command stderr, an
// error string built elsewhere, or a job error already persisted by an older
// build. It is the text-level counterpart to RedactArgs and recognises the
// same flag shapes.
//
// Known limit: a value is a run of non-whitespace, so a password containing a
// space is masked only up to that space. Text is a lossy medium — prefer
// RedactArgs, which works on the real token boundaries, wherever an argv slice
// is still in hand.
func RedactSecrets(text string) string {
	if text == "" {
		return text
	}

	// Attached short flags first: `-phunter2` is a single word, so leaving it
	// intact would let the spaced pattern read it as a flag owning the *next*
	// word and mask the wrong token.
	text = attachedSecretRE.ReplaceAllStringFunc(text, func(match string) string {
		groups := attachedSecretRE.FindStringSubmatch(match)
		flag, ok := isAttachedShortSecret(groups[2] + groups[3])
		if !ok {
			return match
		}
		return groups[1] + flag + Redacted
	})

	text = spacedSecretRE.ReplaceAllStringFunc(text, func(match string) string {
		groups := spacedSecretRE.FindStringSubmatch(match)
		if !IsSecretFlagName(groups[2]) {
			return match
		}
		return groups[1] + groups[2] + groups[3] + Redacted
	})

	return assignedSecretRE.ReplaceAllStringFunc(text, func(match string) string {
		groups := assignedSecretRE.FindStringSubmatch(match)
		if !IsSecretFlagName(groups[1]) {
			return match
		}
		return groups[1] + "=" + Redacted
	})
}

// redactedError carries a scrubbed message while keeping the original error in
// the chain so errors.Is/errors.As still work (context cancellation checks
// depend on it). Only Error() is safe to surface to users or persist.
type redactedError struct {
	msg   string
	cause error
}

func (e *redactedError) Error() string { return e.msg }

func (e *redactedError) Unwrap() error { return e.cause }

// RedactErr returns err with any secrets in its message masked. The original
// error is returned unchanged when there is nothing to redact.
func RedactErr(err error) error {
	if err == nil {
		return nil
	}

	original := err.Error()
	redacted := RedactSecrets(original)
	if redacted == original {
		return err
	}

	return &redactedError{msg: redacted, cause: err}
}
