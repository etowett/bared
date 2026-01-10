// Package main provides the command-line interface for BareD, a backup and restore daemon for databases.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"bared/internal/app"
	"bared/internal/config"
	"bared/internal/daemon"
	"bared/internal/util"
	"bared/internal/version"
)

var (
	cfgFile string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "brd",
	Short: "BareD - Backup and Restore Daemon",
	Long: `BareD is a simple yet powerful backup and restore daemon for databases.
It supports MySQL, PostgreSQL, and Redis with multiple storage backends.`,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "bared.yml", "config file path")

	// Set version info (cobra automatically adds -v/--version flag)
	rootCmd.Version = version.GetFullVersion()

	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(validateConfigCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(configCmd)
}

// initializeLogger initializes the logger with config settings
func initializeLogger(cfg *config.Config) {
	logLevel := util.ParseLogLevel(cfg.LogLevel)

	// Prepare logger options if configured
	var logOpts *util.LoggerOptions
	if cfg.LogOptions != nil {
		logOpts = &util.LoggerOptions{
			AddSource:  cfg.LogOptions.AddSource,
			TimeFormat: cfg.LogOptions.TimeFormat,
		}
	}

	// Initialize with options
	util.InitLoggerWithOptions(logLevel, cfg.LogFormat, logOpts)
}

var validateConfigCmd = &cobra.Command{
	Use:   "validate-config",
	Short: "Validate configuration file",
	RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize logger with config settings
		initializeLogger(cfg)

		if validateErr := cfg.Validate(); validateErr != nil {
			return validateErr
		}

		logger := util.GetLogger()
		logger.InfoS("Configuration validation successful",
			"component", "cli",
			"command", "validate",
			"targets", len(cfg.Targets),
			"storages", len(cfg.Storages),
			"notifiers", len(cfg.Notifiers))

		return nil
	},
}

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup a target database",
	RunE: func(cmd *cobra.Command, _ []string) error {
		targetName, err := cmd.Flags().GetString("target")
		if err != nil {
			return fmt.Errorf("failed to get target flag: %w", err)
		}
		if targetName == "" {
			return fmt.Errorf("--target flag is required")
		}

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize logger with config settings
		initializeLogger(cfg)

		if validateErr := cfg.Validate(); validateErr != nil {
			return validateErr
		}

		// Find the target
		target, err := cfg.FindTarget(targetName)
		if err != nil {
			return err
		}

		// Execute backup
		logger := util.GetLogger()
		ctx := context.Background()
		result, err := app.BackupTarget(ctx, cfg, target, nil) // nil = no progress tracking for CLI
		if err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}

		if result.Success {
			logger.InfoS("Backup completed successfully",
				"component", "cli",
				"command", "backup",
				"target", result.Target,
				"storage", result.StorageName,
				"backup_path", result.BackupPath,
				"duration", result.Duration.String(),
				"size_bytes", result.Size,
				"size_formatted", formatBytes(result.Size))
		}

		return nil
	},
}

func init() {
	backupCmd.Flags().String("target", "", "target name to backup")
}

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore a target database",
	Long: `Restore a database from a backup file.

The restore command supports both regular targets and dedicated restore targets.
Restore targets allow you to restore backups to different databases/hosts.

Examples:
  # Restore latest backup to a target
  brd restore --target mydb --backup latest

  # Restore specific backup to a restore target
  brd restore --target staging_restore --backup et-backups/prod/prod-postgres-2025-12-03.tar.gz

  # Dry-run to validate without restoring
  brd restore --target mydb --backup latest --dry-run

  # Use separate restore config
  brd restore --config restore-config.yml --target staging_restore --backup latest`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		targetName, err := cmd.Flags().GetString("target")
		if err != nil {
			return fmt.Errorf("failed to get target flag: %w", err)
		}
		backupPath, err := cmd.Flags().GetString("backup")
		if err != nil {
			return fmt.Errorf("failed to get backup flag: %w", err)
		}
		dryRun, err := cmd.Flags().GetBool("dry-run")
		if err != nil {
			return fmt.Errorf("failed to get dry-run flag: %w", err)
		}
		skipValidation, err := cmd.Flags().GetBool("skip-validation")
		if err != nil {
			return fmt.Errorf("failed to get skip-validation flag: %w", err)
		}
		skipVerify, err := cmd.Flags().GetBool("skip-verify")
		if err != nil {
			return fmt.Errorf("failed to get skip-verify flag: %w", err)
		}
		noConfirm, err := cmd.Flags().GetBool("yes")
		if err != nil {
			return fmt.Errorf("failed to get yes flag: %w", err)
		}

		if targetName == "" {
			return fmt.Errorf("--target flag is required")
		}

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize logger with config settings
		initializeLogger(cfg)

		if validateErr := cfg.Validate(); validateErr != nil {
			return validateErr
		}

		// Resolve target (could be regular target or restore target)
		target, restoreTarget, isRestoreTarget, err := cfg.ResolveRestoreTarget(targetName)
		if err != nil {
			return fmt.Errorf("failed to resolve restore target: %w", err)
		}

		logger := util.GetLogger()

		logger.InfoS("Resolved restore target", "target", target, "restoreTarget", restoreTarget, "isRestoreTarget", isRestoreTarget)

		// Get storage info for logging
		var storageCfg *config.Storage
		if isRestoreTarget {
			storageCfg, err = cfg.GetStorageForRestoreTarget(restoreTarget)
		} else {
			storageCfg, err = cfg.GetStorageForTarget(target)
		}
		if err != nil {
			return fmt.Errorf("failed to get storage: %w", err)
		}

		ctx := context.Background()

		// If backup is "latest", find the most recent backup
		if backupPath == "latest" {
			logger.InfoS("Finding latest backup for target",
				"component", "cli",
				"command", "restore",
				"target", target.Name)
			latestBackup, findErr := app.FindLatestBackup(ctx, cfg, target)
			if findErr != nil {
				return fmt.Errorf("failed to find latest backup: %w", findErr)
			}
			backupPath = latestBackup.Path
			logger.InfoS("Latest backup found",
				"component", "cli",
				"command", "restore",
				"backup_path", backupPath)
		}

		// Log restore details
		mode := "LIVE"
		if dryRun {
			mode = "DRY-RUN"
		}

		if isRestoreTarget {
			logger.InfoS("Restore operation starting",
				"component", "cli",
				"command", "restore",
				"mode", mode,
				"restore_target", restoreTarget.Name,
				"description", restoreTarget.Description,
				"source_target", restoreTarget.SourceTarget,
				"database_user", target.Conn.User,
				"database_host", target.Conn.Host,
				"database_port", target.Conn.Port,
				"database_name", target.Conn.Database,
				"backup_path", backupPath,
				"storage", storageCfg.Name,
				"storage_type", storageCfg.Type)
		} else {
			logger.InfoS("Restore operation starting",
				"component", "cli",
				"command", "restore",
				"mode", mode,
				"target", target.Name,
				"database_user", target.Conn.User,
				"database_host", target.Conn.Host,
				"database_port", target.Conn.Port,
				"database_name", target.Conn.Database,
				"backup_path", backupPath,
				"storage", storageCfg.Name,
				"storage_type", storageCfg.Type)
		}

		// Confirmation prompt (unless --yes or --dry-run)
		if !dryRun && !noConfirm {
			logger.WarnS("Restore requires confirmation - will overwrite database",
				"component", "cli",
				"command", "restore",
				"database", target.Conn.Database,
				"host", target.Conn.Host)

			fmt.Printf("⚠️  WARNING: This will overwrite the database '%s' on %s!\n",
				target.Conn.Database, target.Conn.Host)
			fmt.Printf("Continue with restore? (yes/no): ")

			var response string
			//nolint:errcheck // Error reading user input is handled by checking response value
			_, _ = fmt.Scanln(&response)
			if strings.ToLower(response) != "yes" {
				logger.InfoS("Restore cancelled by user",
					"component", "cli",
					"command", "restore",
					"user_response", response)
				return nil
			}
			logger.InfoS("Restore confirmed by user",
				"component", "cli",
				"command", "restore")
		}

		// Execute restore with options
		options := &app.RestoreOptions{
			DryRun:           dryRun,
			SkipValidation:   skipValidation,
			SkipBackupVerify: skipVerify,
		}

		result, err := app.RestoreTargetWithOptions(ctx, cfg, target, backupPath, options, nil)
		if err != nil {
			return fmt.Errorf("restore failed: %w", err)
		}

		if result.Success {
			if result.DryRun {
				logger.InfoS("Dry-run completed successfully",
					"component", "cli",
					"command", "restore",
					"validations", result.Validations,
					"validation_count", len(result.Validations))
			} else {
				logger.InfoS("Restore completed successfully",
					"component", "cli",
					"command", "restore",
					"target", result.Target,
					"storage", result.StorageName,
					"backup_path", result.BackupPath,
					"duration", result.Duration.String())
			}
		}

		return nil
	},
}

func init() {
	restoreCmd.Flags().String("target", "", "target or restore target name")
	restoreCmd.Flags().String("backup", "latest", "backup file to restore (or 'latest')")
	restoreCmd.Flags().Bool("dry-run", false, "validate without executing restore")
	restoreCmd.Flags().Bool("skip-validation", false, "skip database connection validation")
	restoreCmd.Flags().Bool("skip-verify", false, "skip backup file verification")
	restoreCmd.Flags().BoolP("yes", "y", false, "skip confirmation prompt")
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List backups for a target",
	RunE: func(cmd *cobra.Command, _ []string) error {
		targetName, err := cmd.Flags().GetString("target")
		if err != nil {
			return fmt.Errorf("failed to get target flag: %w", err)
		}
		if targetName == "" {
			return fmt.Errorf("--target flag is required")
		}

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize logger with config settings
		initializeLogger(cfg)

		if validateErr := cfg.Validate(); validateErr != nil {
			return validateErr
		}

		// Find the target
		target, err := cfg.FindTarget(targetName)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// List backups
		logger := util.GetLogger()
		backups, err := app.ListBackups(ctx, cfg, target)
		if err != nil {
			return fmt.Errorf("failed to list backups: %w", err)
		}

		if len(backups) == 0 {
			logger.InfoS("No backups found for target",
				"component", "cli",
				"command", "list-backups",
				"target", target.Name)
			return nil
		}

		logger.InfoS("Backups found for target",
			"component", "cli",
			"command", "list-backups",
			"target", target.Name,
			"backup_count", len(backups))

		for i, backup := range backups {
			isLatest := i == 0
			logger.InfoS("Backup details",
				"component", "cli",
				"command", "list-backups",
				"index", i+1,
				"path", backup.Path,
				"size_bytes", backup.Size,
				"size_formatted", formatBytes(backup.Size),
				"modified", backup.LastModified.Format("2006-01-02 15:04:05"),
				"storage", backup.StorageName,
				"is_latest", isLatest)
		}

		return nil
	},
}

func init() {
	listCmd.Flags().String("target", "", "target name to list backups for")
}

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run as daemon with optional scheduler and/or HTTP API",
	Long: `Run BareD as a daemon with scheduled backups and/or HTTP API server.

The daemon can run in three modes:
  - Cron-only: Configure schedules in targets, no --http flag (requires at least one schedule)
  - API-only: Enable --http flag, no schedules required (enables manual backups via web/API)
  - Hybrid: Both schedules and --http flag (scheduled + manual backups)

At least one mode (cron or API) must be active for daemon to start.

Configuration can be provided via YAML file or database (requires persistence enabled).
When the config file is not present, daemon will load configuration from the database.

Examples:
  # API-only mode (manual backups via web/API)
  brd daemon --http :8080 --http-user admin --http-pass secret

  # Cron-only mode (scheduled backups only)
  brd daemon

  # Hybrid mode (both scheduled and manual backups)
  brd daemon --http :8080 --http-user admin --http-pass secret`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, fileExists, err := config.LoadOrEmpty(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize logger with config settings
		initializeLogger(cfg)

		logger := util.GetLogger()

		// Only validate if config file exists
		// When file doesn't exist, config will be loaded from database in daemon.Start()
		if fileExists {
			if validateErr := cfg.Validate(); validateErr != nil {
				return validateErr
			}
			logger.InfoS("Loaded configuration from file",
				"component", "daemon",
				"config_file", cfgFile)
		} else {
			logger.InfoS("Config file not found, will load from database",
				"component", "daemon",
				"config_file", cfgFile)
		}

		// Get HTTP flags
		httpAddr, err := cmd.Flags().GetString("http")
		if err != nil {
			return fmt.Errorf("failed to get http flag: %w", err)
		}
		authUser, err := cmd.Flags().GetString("http-user")
		if err != nil {
			return fmt.Errorf("failed to get http-user flag: %w", err)
		}
		authPass, err := cmd.Flags().GetString("http-pass")
		if err != nil {
			return fmt.Errorf("failed to get http-pass flag: %w", err)
		}

		// Prepare daemon options
		var opts []daemon.Option
		if httpAddr != "" {
			if authUser == "" || authPass == "" {
				return fmt.Errorf("--http-user and --http-pass are required when --http is set")
			}
			opts = append(opts, daemon.WithHTTP(httpAddr, authUser, authPass))
		}

		// Create and start daemon
		d := daemon.New(cfg, opts...)
		return d.Start()
	},
}

func init() {
	daemonCmd.Flags().String("http", "", "HTTP server address (e.g., :8080)")
	daemonCmd.Flags().String("http-user", "", "HTTP basic auth username")
	daemonCmd.Flags().String("http-pass", "", "HTTP basic auth password")
}

// formatBytes converts bytes to a human-readable format (KB, MB, GB, etc.)
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
