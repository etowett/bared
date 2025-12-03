package main

import (
	"context"
	"fmt"
	"log"
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
	cfg     *config.Config
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
}

var validateConfigCmd = &cobra.Command{
	Use:   "validate-config",
	Short: "Validate configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize logger with configured level
		logLevel := util.ParseLogLevel(cfg.LogLevel)
		util.InitLogger(logLevel)

		if err := cfg.Validate(); err != nil {
			return err
		}

		fmt.Println("Configuration is valid ✓")
		fmt.Printf("  Targets: %d\n", len(cfg.Targets))
		fmt.Printf("  Storages: %d\n", len(cfg.Storages))
		fmt.Printf("  Notifiers: %d\n", len(cfg.Notifiers))

		return nil
	},
}

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup a target database",
	RunE: func(cmd *cobra.Command, args []string) error {
		targetName, _ := cmd.Flags().GetString("target")
		if targetName == "" {
			return fmt.Errorf("--target flag is required")
		}

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize logger with configured level
		logLevel := util.ParseLogLevel(cfg.LogLevel)
		util.InitLogger(logLevel)

		if err := cfg.Validate(); err != nil {
			return err
		}

		// Find the target
		target, err := cfg.FindTarget(targetName)
		if err != nil {
			return err
		}

		// Execute backup
		ctx := context.Background()
		result, err := app.BackupTarget(ctx, cfg, target, nil) // nil = no progress tracking for CLI
		if err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}

		if result.Success {
			fmt.Printf("\n✓ Backup completed successfully\n")
			fmt.Printf("  Target: %s\n", result.Target)
			fmt.Printf("  Storage: %s\n", result.StorageName)
			fmt.Printf("    Path: %s\n", result.BackupPath)
			fmt.Printf("    Duration: %v\n", result.Duration)
			fmt.Printf("    Size: %s\n", formatBytes(result.Size))

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
	RunE: func(cmd *cobra.Command, args []string) error {
		targetName, _ := cmd.Flags().GetString("target")
		backupPath, _ := cmd.Flags().GetString("backup")

		if targetName == "" {
			return fmt.Errorf("--target flag is required")
		}

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize logger with configured level
		logLevel := util.ParseLogLevel(cfg.LogLevel)
		util.InitLogger(logLevel)

		if err := cfg.Validate(); err != nil {
			return err
		}

		// Find the target
		target, err := cfg.FindTarget(targetName)
		if err != nil {
			return err
		}

		// Get storage info for logging (before calling RestoreTarget)
		storageCfg, err := cfg.GetStorageForTarget(target)
		if err != nil {
			return fmt.Errorf("failed to get storage: %w", err)
		}

		// Build storage path string for display
		storagePath := ""
		if storageCfg.Path != "" {
			storagePath = fmt.Sprintf("%s/%s", storageCfg.Path, backupPath)
		} else if storageCfg.Bucket != "" {
			storagePath = fmt.Sprintf("s3://%s/%s", storageCfg.Bucket, backupPath)
		} else if storageCfg.Host != "" {
			storagePath = fmt.Sprintf("sftp://%s@%s:%d%s/%s",
				storageCfg.Username, storageCfg.Host, storageCfg.Port, storageCfg.Path, backupPath)
		}

		// Determine if backup is compressed
		needsDecompression := strings.HasSuffix(backupPath, ".tar.gz") || strings.HasSuffix(backupPath, ".tgz")
		compressionInfo := ""
		if needsDecompression {
			compressionInfo = " (compressed)"
		}

		log.Printf("[%s] Restore details:", target.Name)
		log.Printf("\tStorage: %s", storageCfg.Name)
		if storagePath != "" {
			log.Printf("\t\tStorage path: %s", storagePath)
		}
		log.Printf("\tDatabase: %s (type: %s, host: %s:%d, user: %s)",
			target.Conn.Database,
			target.Conn.Type,
			target.Conn.Host,
			target.Conn.Port,
			target.Conn.User)
		log.Printf("\tBackup file: %s%s", backupPath, compressionInfo)

		ctx := context.Background()

		// If backup is "latest", find the most recent backup
		if backupPath == "latest" {
			latestBackup, err := app.FindLatestBackup(ctx, cfg, target)
			if err != nil {
				return fmt.Errorf("failed to find latest backup: %w", err)
			}
			backupPath = latestBackup.Path
			fmt.Printf("Using latest backup: %s\n", backupPath)
			// Update compression info after resolving latest backup
			needsDecompression = strings.HasSuffix(backupPath, ".tar.gz") || strings.HasSuffix(backupPath, ".tgz")
			if needsDecompression {
				compressionInfo = " (compressed)"
			} else {
				compressionInfo = ""
			}
			log.Printf("\tBackup file: %s%s", backupPath, compressionInfo)
		}

		// Execute restore
		result, err := app.RestoreTarget(ctx, cfg, target, backupPath, nil) // nil = no progress tracking for CLI
		if err != nil {
			return fmt.Errorf("restore failed: %w", err)
		}

		if result.Success {
			fmt.Printf("\n✓ Restore completed successfully\n")
			fmt.Printf("  Target: %s\n", result.Target)
			fmt.Printf("  Storage: %s\n", result.StorageName)
			fmt.Printf("  Backup: %s\n", result.BackupPath)
			fmt.Printf("  Duration: %v\n", result.Duration)
		}

		return nil
	},
}

func init() {
	restoreCmd.Flags().String("target", "", "target name to restore")
	restoreCmd.Flags().String("backup", "latest", "backup file to restore (or 'latest')")
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List backups for a target",
	RunE: func(cmd *cobra.Command, args []string) error {
		targetName, _ := cmd.Flags().GetString("target")
		if targetName == "" {
			return fmt.Errorf("--target flag is required")
		}

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize logger with configured level
		logLevel := util.ParseLogLevel(cfg.LogLevel)
		util.InitLogger(logLevel)

		if err := cfg.Validate(); err != nil {
			return err
		}

		// Find the target
		target, err := cfg.FindTarget(targetName)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// List backups
		backups, err := app.ListBackups(ctx, cfg, target)
		if err != nil {
			return fmt.Errorf("failed to list backups: %w", err)
		}

		if len(backups) == 0 {
			fmt.Printf("No backups found for target: %s\n", target.Name)
			return nil
		}

		fmt.Printf("Backups for target '%s' (%d total):\n\n", target.Name, len(backups))
		for i, backup := range backups {
			fmt.Printf("%d. %s\n", i+1, backup.Path)
			fmt.Printf("   Size: %s\n", formatBytes(backup.Size))
			fmt.Printf("   Modified: %s\n", backup.LastModified.Format("2006-01-02 15:04:05"))
			fmt.Printf("   Storage: %s\n", backup.StorageName)
			if i == 0 {
				fmt.Printf("   [LATEST]\n")
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	listCmd.Flags().String("target", "", "target name to list backups for")
}

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run as daemon with scheduler",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize logger with configured level
		logLevel := util.ParseLogLevel(cfg.LogLevel)
		util.InitLogger(logLevel)

		if err := cfg.Validate(); err != nil {
			return err
		}

		// Get HTTP flags
		httpAddr, _ := cmd.Flags().GetString("http")
		authUser, _ := cmd.Flags().GetString("http-user")
		authPass, _ := cmd.Flags().GetString("http-pass")

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
