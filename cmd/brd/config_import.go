package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"bared/internal/client"
	"bared/internal/config"
	"bared/internal/configservice"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management commands",
	Long:  `Manage BareD configurations through the API`,
}

var configImportCmd = &cobra.Command{
	Use:   "import [config-file]",
	Short: "Import configuration from YAML file to database via API",
	Long: `Import configuration from a YAML file into the database via the HTTP API.

Requires a running daemon with HTTP API enabled. Imports storages, notifiers,
targets, restore targets, and global configuration settings.

Conflict Resolution Modes:
  interactive (default): Prompt for each existing resource
  override: Update all existing resources with new values
  skip: Only create new resources, skip existing ones

Examples:
  # Interactive mode - prompts for conflicts
  brd config import bared.yml --user admin --pass secret

  # Override all existing configs
  brd config import bared.yml --user admin --pass secret --mode override

  # Only create new configs
  brd config import bared.yml --user admin --pass secret --mode skip

  # Dry run - validate without importing
  brd config import bared.yml --user admin --pass secret --dry-run

  # Remote daemon
  brd config import bared.yml --api-url https://backup.example.com --user admin --pass secret`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigImport,
}

func init() {
	configImportCmd.Flags().String("api-url", "http://localhost:8080", "Daemon API URL")
	configImportCmd.Flags().String("user", "", "HTTP basic auth username (required)")
	configImportCmd.Flags().String("pass", "", "HTTP basic auth password (required)")
	configImportCmd.Flags().String("mode", "interactive", "Conflict resolution mode: interactive, override, skip")
	configImportCmd.Flags().Bool("dry-run", false, "Validate without importing")
	configImportCmd.Flags().Duration("timeout", 30*time.Second, "HTTP request timeout")
	configImportCmd.Flags().BoolP("yes", "y", false, "Auto-confirm all prompts (equivalent to --mode override)")

	configCmd.AddCommand(configImportCmd)
}

func runConfigImport(cmd *cobra.Command, args []string) error {
	configFile := args[0]

	// Get flags
	apiURL, err := cmd.Flags().GetString("api-url")
	if err != nil {
		return fmt.Errorf("failed to get api-url flag: %w", err)
	}
	user, err := cmd.Flags().GetString("user")
	if err != nil {
		return fmt.Errorf("failed to get user flag: %w", err)
	}
	pass, err := cmd.Flags().GetString("pass")
	if err != nil {
		return fmt.Errorf("failed to get pass flag: %w", err)
	}
	modeStr, err := cmd.Flags().GetString("mode")
	if err != nil {
		return fmt.Errorf("failed to get mode flag: %w", err)
	}
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return fmt.Errorf("failed to get dry-run flag: %w", err)
	}
	timeout, err := cmd.Flags().GetDuration("timeout")
	if err != nil {
		return fmt.Errorf("failed to get timeout flag: %w", err)
	}
	autoYes, err := cmd.Flags().GetBool("yes")
	if err != nil {
		return fmt.Errorf("failed to get yes flag: %w", err)
	}

	// Validate required flags
	if user == "" {
		return fmt.Errorf("--user flag is required")
	}
	if pass == "" {
		return fmt.Errorf("--pass flag is required")
	}

	// Parse mode
	mode := client.ConflictMode(modeStr)
	if autoYes {
		mode = client.ModeOverride
	}

	// Validate mode
	switch mode {
	case client.ModeInteractive, client.ModeOverride, client.ModeSkip:
		// Valid
	default:
		return fmt.Errorf("invalid mode: %s (must be: interactive, override, or skip)", modeStr)
	}

	// Load and validate config file
	fmt.Printf("Loading configuration from %s...\n", configFile)
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	// Validate config
	fmt.Println("Validating configuration...")
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	fmt.Println("✓ Configuration is valid")

	// Create API client
	importClient := client.NewImportClient(apiURL, user, pass, timeout)

	// Test connection
	fmt.Printf("\nConnecting to daemon at %s...\n", apiURL)
	ctx := context.Background()
	if err := importClient.Ping(ctx); err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	fmt.Println("✓ Connected successfully")

	// Perform import
	if dryRun {
		fmt.Println("\n=== DRY RUN MODE ===")
		fmt.Println("Checking for conflicts (no changes will be made)...")
	} else {
		fmt.Printf("\nImporting configuration (mode: %s)...\n", mode)
	}

	summary := importConfig(ctx, importClient, cfg, mode, dryRun)

	// Print summary
	printImportSummary(summary)

	if summary.HasErrors() {
		// Build a more specific error message
		failCount := summary.TotalFailed()
		return fmt.Errorf("%d resource(s) failed to import - see details above", failCount)
	}

	return nil
}

func importConfig(ctx context.Context, apiClient *client.ImportClient, cfg *config.Config, mode client.ConflictMode, dryRun bool) *client.ImportSummary {
	summary := &client.ImportSummary{
		DryRun: dryRun,
	}

	// Import storages first (no dependencies)
	if len(cfg.Storages) > 0 {
		fmt.Printf("\nProcessing %d storage(s)...\n", len(cfg.Storages))
		summary.Storages = importStorages(ctx, apiClient, cfg.Storages, mode, dryRun)
	}

	// Import notifiers (no dependencies)
	if len(cfg.Notifiers) > 0 {
		fmt.Printf("\nProcessing %d notifier(s)...\n", len(cfg.Notifiers))
		summary.Notifiers = importNotifiers(ctx, apiClient, cfg.Notifiers, mode, dryRun)
	}

	// Import targets (depend on storages)
	if len(cfg.Targets) > 0 {
		fmt.Printf("\nProcessing %d target(s)...\n", len(cfg.Targets))
		summary.Targets = importTargets(ctx, apiClient, cfg.Targets, mode, dryRun)
	}

	// Import restore targets (depend on storages and targets)
	if len(cfg.RestoreTargets) > 0 {
		fmt.Printf("\nProcessing %d restore target(s)...\n", len(cfg.RestoreTargets))
		summary.RestoreTargets = importRestoreTargets(ctx, apiClient, cfg.RestoreTargets, mode, dryRun)
	}

	// Import global config
	if cfg.DefaultStorage != "" || cfg.LogLevel != "" || cfg.LogFormat != "" {
		fmt.Println("\nProcessing global configuration...")
		summary.GlobalConfig = importGlobalConfig(ctx, apiClient, cfg, mode, dryRun)
	}

	return summary
}

func importStorages(ctx context.Context, apiClient *client.ImportClient, storages map[string]*config.Storage, mode client.ConflictMode, dryRun bool) client.ResourceSummary {
	summary := client.ResourceSummary{}

	for name, storage := range storages {
		// Set the name from the map key (YAML doesn't populate this automatically)
		storage.Name = name

		// Validate storage
		if err := configservice.ValidateStorage(storage); err != nil {
			summary.Failed = append(summary.Failed, client.FailedResource{
				Name:  name,
				Error: fmt.Sprintf("validation failed: %v", err),
			})
			fmt.Printf("  ✗ %s (validation failed: %v)\n", name, err)
			continue
		}

		// Check if exists
		exists, err := apiClient.StorageExists(ctx, name)
		if err != nil {
			summary.Failed = append(summary.Failed, client.FailedResource{
				Name:  name,
				Error: fmt.Sprintf("failed to check existence: %v", err),
			})
			fmt.Printf("  ✗ %s (failed to check: %v)\n", name, err)
			continue
		}

		// Handle conflict
		action := resolveConflict("storage", name, exists, mode)
		if action == client.ActionAbort {
			fmt.Println("\nImport aborted by user")
			os.Exit(0)
		}

		if dryRun {
			if exists {
				if action == client.ActionUpdate {
					summary.Updated = append(summary.Updated, name)
					fmt.Printf("  ! %s (would update)\n", name)
				} else {
					summary.Skipped = append(summary.Skipped, name)
					fmt.Printf("  ⊘ %s (would skip)\n", name)
				}
			} else {
				summary.Created = append(summary.Created, name)
				fmt.Printf("  ✓ %s (would create)\n", name)
			}
			continue
		}

		// Perform operation
		if exists {
			if action == client.ActionUpdate {
				if err := apiClient.UpdateStorage(ctx, name, storage); err != nil {
					summary.Failed = append(summary.Failed, client.FailedResource{
						Name:  name,
						Error: err.Error(),
					})
					fmt.Printf("  ✗ %s (update failed: %v)\n", name, err)
				} else {
					summary.Updated = append(summary.Updated, name)
					fmt.Printf("  ✓ %s (updated)\n", name)
				}
			} else {
				summary.Skipped = append(summary.Skipped, name)
				fmt.Printf("  ⊘ %s (skipped)\n", name)
			}
		} else {
			if err := apiClient.CreateStorage(ctx, storage); err != nil {
				// If we got a conflict error and mode is override, try updating instead
				if strings.Contains(err.Error(), "409") && mode == client.ModeOverride {
					if updateErr := apiClient.UpdateStorage(ctx, name, storage); updateErr != nil {
						summary.Failed = append(summary.Failed, client.FailedResource{
							Name:  name,
							Error: fmt.Sprintf("create returned 409, update also failed: %v", updateErr),
						})
						fmt.Printf("  ✗ %s (failed: %v)\n", name, updateErr)
					} else {
						summary.Updated = append(summary.Updated, name)
						fmt.Printf("  ✓ %s (updated)\n", name)
					}
				} else {
					summary.Failed = append(summary.Failed, client.FailedResource{
						Name:  name,
						Error: err.Error(),
					})
					fmt.Printf("  ✗ %s (create failed: %v)\n", name, err)
				}
			} else {
				summary.Created = append(summary.Created, name)
				fmt.Printf("  ✓ %s (created)\n", name)
			}
		}
	}

	return summary
}

func importNotifiers(ctx context.Context, apiClient *client.ImportClient, notifiers map[string]*config.Notifier, mode client.ConflictMode, dryRun bool) client.ResourceSummary {
	summary := client.ResourceSummary{}

	for name, notifier := range notifiers {
		// Validate notifier
		if err := configservice.ValidateNotifier(notifier); err != nil {
			summary.Failed = append(summary.Failed, client.FailedResource{
				Name:  name,
				Error: fmt.Sprintf("validation failed: %v", err),
			})
			fmt.Printf("  ✗ %s (validation failed: %v)\n", name, err)
			continue
		}

		// Check if exists
		exists, err := apiClient.NotifierExists(ctx, name)
		if err != nil {
			summary.Failed = append(summary.Failed, client.FailedResource{
				Name:  name,
				Error: fmt.Sprintf("failed to check existence: %v", err),
			})
			fmt.Printf("  ✗ %s (failed to check: %v)\n", name, err)
			continue
		}

		// Handle conflict
		action := resolveConflict("notifier", name, exists, mode)
		if action == client.ActionAbort {
			fmt.Println("\nImport aborted by user")
			os.Exit(0)
		}

		if dryRun {
			if exists {
				if action == client.ActionUpdate {
					summary.Updated = append(summary.Updated, name)
					fmt.Printf("  ! %s (would update)\n", name)
				} else {
					summary.Skipped = append(summary.Skipped, name)
					fmt.Printf("  ⊘ %s (would skip)\n", name)
				}
			} else {
				summary.Created = append(summary.Created, name)
				fmt.Printf("  ✓ %s (would create)\n", name)
			}
			continue
		}

		// Perform operation
		if exists {
			if action == client.ActionUpdate {
				if err := apiClient.UpdateNotifier(ctx, name, notifier); err != nil {
					summary.Failed = append(summary.Failed, client.FailedResource{
						Name:  name,
						Error: err.Error(),
					})
					fmt.Printf("  ✗ %s (update failed: %v)\n", name, err)
				} else {
					summary.Updated = append(summary.Updated, name)
					fmt.Printf("  ✓ %s (updated)\n", name)
				}
			} else {
				summary.Skipped = append(summary.Skipped, name)
				fmt.Printf("  ⊘ %s (skipped)\n", name)
			}
		} else {
			if err := apiClient.CreateNotifier(ctx, name, notifier); err != nil {
				// If we got a conflict error and mode is override, try updating instead
				if strings.Contains(err.Error(), "409") && mode == client.ModeOverride {
					if updateErr := apiClient.UpdateNotifier(ctx, name, notifier); updateErr != nil {
						summary.Failed = append(summary.Failed, client.FailedResource{
							Name:  name,
							Error: fmt.Sprintf("create returned 409, update also failed: %v", updateErr),
						})
						fmt.Printf("  ✗ %s (failed: %v)\n", name, updateErr)
					} else {
						summary.Updated = append(summary.Updated, name)
						fmt.Printf("  ✓ %s (updated)\n", name)
					}
				} else {
					summary.Failed = append(summary.Failed, client.FailedResource{
						Name:  name,
						Error: err.Error(),
					})
					fmt.Printf("  ✗ %s (create failed: %v)\n", name, err)
				}
			} else {
				summary.Created = append(summary.Created, name)
				fmt.Printf("  ✓ %s (created)\n", name)
			}
		}
	}

	return summary
}

func importTargets(ctx context.Context, apiClient *client.ImportClient, targets []*config.Target, mode client.ConflictMode, dryRun bool) client.ResourceSummary {
	summary := client.ResourceSummary{}

	for _, target := range targets {
		// Get existing storages for validation
		// In a real scenario, we'd cache this
		// For now, we skip this validation as the API will handle it

		// Check if exists
		exists, err := apiClient.TargetExists(ctx, target.Name)
		if err != nil {
			summary.Failed = append(summary.Failed, client.FailedResource{
				Name:  target.Name,
				Error: fmt.Sprintf("failed to check existence: %v", err),
			})
			fmt.Printf("  ✗ %s (failed to check: %v)\n", target.Name, err)
			continue
		}

		// Handle conflict
		action := resolveConflict("target", target.Name, exists, mode)
		if action == client.ActionAbort {
			fmt.Println("\nImport aborted by user")
			os.Exit(0)
		}

		if dryRun {
			if exists {
				if action == client.ActionUpdate {
					summary.Updated = append(summary.Updated, target.Name)
					fmt.Printf("  ! %s (would update)\n", target.Name)
				} else {
					summary.Skipped = append(summary.Skipped, target.Name)
					fmt.Printf("  ⊘ %s (would skip)\n", target.Name)
				}
			} else {
				summary.Created = append(summary.Created, target.Name)
				fmt.Printf("  ✓ %s (would create)\n", target.Name)
			}
			continue
		}

		// Perform operation
		if exists {
			if action == client.ActionUpdate {
				if err := apiClient.UpdateTarget(ctx, target.Name, target); err != nil {
					summary.Failed = append(summary.Failed, client.FailedResource{
						Name:  target.Name,
						Error: err.Error(),
					})
					fmt.Printf("  ✗ %s (update failed: %v)\n", target.Name, err)
				} else {
					summary.Updated = append(summary.Updated, target.Name)
					fmt.Printf("  ✓ %s (updated)\n", target.Name)
				}
			} else {
				summary.Skipped = append(summary.Skipped, target.Name)
				fmt.Printf("  ⊘ %s (skipped)\n", target.Name)
			}
		} else {
			if err := apiClient.CreateTarget(ctx, target); err != nil {
				// If we got a conflict error and mode is override, try updating instead
				if strings.Contains(err.Error(), "409") && mode == client.ModeOverride {
					if updateErr := apiClient.UpdateTarget(ctx, target.Name, target); updateErr != nil {
						summary.Failed = append(summary.Failed, client.FailedResource{
							Name:  target.Name,
							Error: fmt.Sprintf("create returned 409, update also failed: %v", updateErr),
						})
						fmt.Printf("  ✗ %s (failed: %v)\n", target.Name, updateErr)
					} else {
						summary.Updated = append(summary.Updated, target.Name)
						fmt.Printf("  ✓ %s (updated)\n", target.Name)
					}
				} else {
					summary.Failed = append(summary.Failed, client.FailedResource{
						Name:  target.Name,
						Error: err.Error(),
					})
					fmt.Printf("  ✗ %s (create failed: %v)\n", target.Name, err)
				}
			} else {
				summary.Created = append(summary.Created, target.Name)
				fmt.Printf("  ✓ %s (created)\n", target.Name)
			}
		}
	}

	return summary
}

func importRestoreTargets(ctx context.Context, apiClient *client.ImportClient, restoreTargets []*config.RestoreTarget, mode client.ConflictMode, dryRun bool) client.ResourceSummary {
	summary := client.ResourceSummary{}

	for _, rt := range restoreTargets {
		// Check if exists
		exists, err := apiClient.RestoreTargetExists(ctx, rt.Name)
		if err != nil {
			summary.Failed = append(summary.Failed, client.FailedResource{
				Name:  rt.Name,
				Error: fmt.Sprintf("failed to check existence: %v", err),
			})
			fmt.Printf("  ✗ %s (failed to check: %v)\n", rt.Name, err)
			continue
		}

		// Handle conflict
		action := resolveConflict("restore target", rt.Name, exists, mode)
		if action == client.ActionAbort {
			fmt.Println("\nImport aborted by user")
			os.Exit(0)
		}

		if dryRun {
			if exists {
				if action == client.ActionUpdate {
					summary.Updated = append(summary.Updated, rt.Name)
					fmt.Printf("  ! %s (would update)\n", rt.Name)
				} else {
					summary.Skipped = append(summary.Skipped, rt.Name)
					fmt.Printf("  ⊘ %s (would skip)\n", rt.Name)
				}
			} else {
				summary.Created = append(summary.Created, rt.Name)
				fmt.Printf("  ✓ %s (would create)\n", rt.Name)
			}
			continue
		}

		// Perform operation
		if exists {
			if action == client.ActionUpdate {
				if err := apiClient.UpdateRestoreTarget(ctx, rt.Name, rt); err != nil {
					summary.Failed = append(summary.Failed, client.FailedResource{
						Name:  rt.Name,
						Error: err.Error(),
					})
					fmt.Printf("  ✗ %s (update failed: %v)\n", rt.Name, err)
				} else {
					summary.Updated = append(summary.Updated, rt.Name)
					fmt.Printf("  ✓ %s (updated)\n", rt.Name)
				}
			} else {
				summary.Skipped = append(summary.Skipped, rt.Name)
				fmt.Printf("  ⊘ %s (skipped)\n", rt.Name)
			}
		} else {
			if err := apiClient.CreateRestoreTarget(ctx, rt); err != nil {
				// If we got a conflict error and mode is override, try updating instead
				if strings.Contains(err.Error(), "409") && mode == client.ModeOverride {
					if updateErr := apiClient.UpdateRestoreTarget(ctx, rt.Name, rt); updateErr != nil {
						summary.Failed = append(summary.Failed, client.FailedResource{
							Name:  rt.Name,
							Error: fmt.Sprintf("create returned 409, update also failed: %v", updateErr),
						})
						fmt.Printf("  ✗ %s (failed: %v)\n", rt.Name, updateErr)
					} else {
						summary.Updated = append(summary.Updated, rt.Name)
						fmt.Printf("  ✓ %s (updated)\n", rt.Name)
					}
				} else {
					summary.Failed = append(summary.Failed, client.FailedResource{
						Name:  rt.Name,
						Error: err.Error(),
					})
					fmt.Printf("  ✗ %s (create failed: %v)\n", rt.Name, err)
				}
			} else {
				summary.Created = append(summary.Created, rt.Name)
				fmt.Printf("  ✓ %s (created)\n", rt.Name)
			}
		}
	}

	return summary
}

func importGlobalConfig(ctx context.Context, apiClient *client.ImportClient, cfg *config.Config, mode client.ConflictMode, dryRun bool) client.GlobalConfigSummary {
	summary := client.GlobalConfigSummary{}

	// Get existing global config
	existing, err := apiClient.GetGlobalConfig(ctx)
	if err != nil {
		// If we can't get existing config, just try to set new values
		existing = make(map[string]string)
	}

	// Helper function to import a single config value
	importConfigValue := func(key, value string) {
		if value == "" {
			return
		}

		_, exists := existing[key]
		action := resolveConflict("global config", key, exists, mode)
		if action == client.ActionAbort {
			fmt.Println("\nImport aborted by user")
			os.Exit(0)
		}

		if dryRun {
			if exists {
				if action == client.ActionUpdate {
					summary.Updated = append(summary.Updated, key)
					fmt.Printf("  ! %s (would update)\n", key)
				} else {
					summary.Skipped = append(summary.Skipped, key)
					fmt.Printf("  ⊘ %s (would skip)\n", key)
				}
			} else {
				summary.Updated = append(summary.Updated, key)
				fmt.Printf("  ✓ %s (would set)\n", key)
			}
			return
		}

		// Perform operation
		if !exists || action == client.ActionUpdate {
			if err := apiClient.SetGlobalConfig(ctx, key, value); err != nil {
				summary.Failed = append(summary.Failed, client.FailedConfig{
					Key:   key,
					Error: err.Error(),
				})
				fmt.Printf("  ✗ %s (failed: %v)\n", key, err)
			} else {
				summary.Updated = append(summary.Updated, key)
				fmt.Printf("  ✓ %s (set)\n", key)
			}
		} else {
			summary.Skipped = append(summary.Skipped, key)
			fmt.Printf("  ⊘ %s (skipped)\n", key)
		}
	}

	importConfigValue("default_storage", cfg.DefaultStorage)
	importConfigValue("log_level", cfg.LogLevel)
	importConfigValue("log_format", cfg.LogFormat)

	return summary
}

func resolveConflict(resourceType, name string, exists bool, mode client.ConflictMode) client.ConflictAction {
	if !exists {
		return client.ActionUpdate // Will create
	}

	switch mode {
	case client.ModeOverride:
		return client.ActionUpdate
	case client.ModeSkip:
		return client.ActionSkip
	case client.ModeInteractive:
		return promptConflictResolution(resourceType, name)
	default:
		return client.ActionSkip
	}
}

func promptConflictResolution(resourceType, name string) client.ConflictAction {
	// Capitalize first letter
	capitalized := resourceType
	if len(resourceType) > 0 {
		capitalized = strings.ToUpper(resourceType[:1]) + resourceType[1:]
	}
	fmt.Printf("\n%s \"%s\" already exists. What would you like to do?\n", capitalized, name)
	fmt.Println("  [u] Update with new configuration")
	fmt.Println("  [s] Skip (keep existing)")
	fmt.Println("  [a] Abort import")
	fmt.Print("Choice: ")

	reader := bufio.NewReader(os.Stdin)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			os.Exit(1)
		}

		choice := strings.ToLower(strings.TrimSpace(input))
		switch choice {
		case "u", "update":
			return client.ActionUpdate
		case "s", "skip":
			return client.ActionSkip
		case "a", "abort":
			return client.ActionAbort
		default:
			fmt.Print("Invalid choice. Please enter 'u', 's', or 'a': ")
		}
	}
}

func printImportSummary(summary *client.ImportSummary) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	if summary.DryRun {
		fmt.Println("DRY RUN SUMMARY")
	} else {
		fmt.Println("IMPORT SUMMARY")
	}
	fmt.Println(strings.Repeat("=", 60))

	printResourceSummary("Storages", summary.Storages, summary.DryRun)
	printResourceSummary("Notifiers", summary.Notifiers, summary.DryRun)
	printResourceSummary("Targets", summary.Targets, summary.DryRun)
	printResourceSummary("Restore Targets", summary.RestoreTargets, summary.DryRun)
	printGlobalConfigSummary(summary.GlobalConfig, summary.DryRun)

	fmt.Println(strings.Repeat("-", 60))
	if summary.DryRun {
		fmt.Printf("Would create: %d  |  Would update: %d  |  Would skip: %d\n",
			summary.TotalCreated(), summary.TotalUpdated(), summary.TotalSkipped())
	} else {
		fmt.Printf("Created: %d  |  Updated: %d  |  Skipped: %d\n",
			summary.TotalCreated(), summary.TotalUpdated(), summary.TotalSkipped())
	}

	if summary.HasErrors() {
		fmt.Printf("Failed: %d\n", summary.TotalFailed())
		fmt.Println("\n⚠ Import completed with errors")
		fmt.Println("  Review the failed resources above for details")
	} else {
		if summary.DryRun {
			fmt.Println("\n✓ Dry run completed successfully")
			fmt.Println("  Run without --dry-run to perform the import")
		} else {
			fmt.Println("\n✓ Import completed successfully")
		}
	}
	fmt.Println(strings.Repeat("=", 60))
}

func printResourceSummary(title string, summary client.ResourceSummary, dryRun bool) {
	if len(summary.Created) == 0 && len(summary.Updated) == 0 &&
		len(summary.Skipped) == 0 && len(summary.Failed) == 0 {
		return
	}

	fmt.Printf("\n%s:\n", title)

	if len(summary.Created) > 0 {
		verb := "Created"
		if dryRun {
			verb = "Would create"
		}
		fmt.Printf("  %s: %s\n", verb, strings.Join(summary.Created, ", "))
	}

	if len(summary.Updated) > 0 {
		verb := "Updated"
		if dryRun {
			verb = "Would update"
		}
		fmt.Printf("  %s: %s\n", verb, strings.Join(summary.Updated, ", "))
	}

	if len(summary.Skipped) > 0 {
		verb := "Skipped"
		if dryRun {
			verb = "Would skip"
		}
		fmt.Printf("  %s: %s\n", verb, strings.Join(summary.Skipped, ", "))
	}

	if len(summary.Failed) > 0 {
		fmt.Println("  Failed:")
		for _, failed := range summary.Failed {
			fmt.Printf("    - %s: %s\n", failed.Name, failed.Error)
		}
	}
}

func printGlobalConfigSummary(summary client.GlobalConfigSummary, dryRun bool) {
	if len(summary.Updated) == 0 && len(summary.Skipped) == 0 && len(summary.Failed) == 0 {
		return
	}

	fmt.Println("\nGlobal Configuration:")

	if len(summary.Updated) > 0 {
		verb := "Set"
		if dryRun {
			verb = "Would set"
		}
		fmt.Printf("  %s: %s\n", verb, strings.Join(summary.Updated, ", "))
	}

	if len(summary.Skipped) > 0 {
		verb := "Skipped"
		if dryRun {
			verb = "Would skip"
		}
		fmt.Printf("  %s: %s\n", verb, strings.Join(summary.Skipped, ", "))
	}

	if len(summary.Failed) > 0 {
		fmt.Println("  Failed:")
		for _, failed := range summary.Failed {
			fmt.Printf("    - %s: %s\n", failed.Key, failed.Error)
		}
	}
}
