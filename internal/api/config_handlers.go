package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"bared/internal/config"
	"bared/internal/configservice"
)

// Storage handlers

func (s *Server) handleListStorages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	storages, err := s.configService.ListStorages(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list storages: %v", err))
		return
	}

	// Get config source
	source, _ := s.configLoader.GetConfigSource(ctx) //nolint:errcheck // default to empty string if fails

	// Convert to response format with secret filtering
	var responses []StorageResponse
	for _, storage := range storages {
		responses = append(responses, s.storageToResponse(storage))
	}

	respondJSON(w, http.StatusOK, ListStoragesResponse{
		Storages: responses,
		Total:    len(responses),
		Source:   string(source),
	})
}

func (s *Server) handleGetStorage(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "Storage name is required")
		return
	}

	storage, err := s.configService.GetStorage(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get storage: %v", err))
		}
		return
	}

	respondJSON(w, http.StatusOK, s.storageToResponse(storage))
}

func (s *Server) handleCreateStorage(w http.ResponseWriter, r *http.Request) {
	var req StorageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	// Convert request to config.Storage
	storage := s.requestToStorage(&req)

	// Validate
	if err := configservice.ValidateStorage(storage); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Validation failed: %v", err))
		return
	}

	// Create
	if err := s.configService.CreateStorage(r.Context(), storage); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			respondError(w, http.StatusConflict, "Storage with this name already exists")
		} else {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create storage: %v", err))
		}
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"message": "Storage created successfully",
		"name":    storage.Name,
	})
}

func (s *Server) handleUpdateStorage(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "Storage name is required")
		return
	}

	var req StorageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	// Ensure name matches URL
	req.Name = name

	storage := s.requestToStorage(&req)

	if err := configservice.ValidateStorage(storage); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Validation failed: %v", err))
		return
	}

	if err := s.configService.UpdateStorage(r.Context(), name, storage); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update storage: %v", err))
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Storage updated successfully",
		"name":    name,
	})
}

func (s *Server) handleDeleteStorage(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "Storage name is required")
		return
	}

	if err := s.configService.DeleteStorage(r.Context(), name); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete storage: %v", err))
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Storage deleted successfully",
		"name":    name,
	})
}

// Notifier handlers

func (s *Server) handleListNotifiers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	notifiers, err := s.configService.ListNotifiers(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list notifiers: %v", err))
		return
	}

	source, _ := s.configLoader.GetConfigSource(ctx) //nolint:errcheck // default to empty string if fails

	var responses []NotifierResponse
	for name, notifier := range notifiers {
		resp := s.notifierToResponse(notifier)
		resp.Name = name // Set the name from the map key
		responses = append(responses, resp)
	}

	respondJSON(w, http.StatusOK, ListNotifiersResponse{
		Notifiers: responses,
		Total:     len(responses),
		Source:    string(source),
	})
}

func (s *Server) handleCreateNotifier(w http.ResponseWriter, r *http.Request) {
	var req NotifierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	notifier := s.requestToNotifier(&req)

	if err := configservice.ValidateNotifier(notifier); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Validation failed: %v", err))
		return
	}

	if err := s.configService.CreateNotifier(r.Context(), req.Name, notifier); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			respondError(w, http.StatusConflict, "Notifier with this name already exists")
		} else {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create notifier: %v", err))
		}
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"message": "Notifier created successfully",
		"name":    req.Name,
	})
}

func (s *Server) handleUpdateNotifier(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "Notifier name is required")
		return
	}

	var req NotifierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	req.Name = name
	notifier := s.requestToNotifier(&req)

	if err := configservice.ValidateNotifier(notifier); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Validation failed: %v", err))
		return
	}

	if err := s.configService.UpdateNotifier(r.Context(), name, notifier); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update notifier: %v", err))
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Notifier updated successfully",
		"name":    name,
	})
}

func (s *Server) handleDeleteNotifier(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "Notifier name is required")
		return
	}

	if err := s.configService.DeleteNotifier(r.Context(), name); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete notifier: %v", err))
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Notifier deleted successfully",
		"name":    name,
	})
}

// Target handlers

func (s *Server) handleListTargetsConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	targets, err := s.configService.ListTargets(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list targets: %v", err))
		return
	}

	source, _ := s.configLoader.GetConfigSource(ctx) //nolint:errcheck // default to empty string if fails

	var responses []TargetResponse
	for _, target := range targets {
		responses = append(responses, s.targetToResponse(target))
	}

	respondJSON(w, http.StatusOK, ListTargetsConfigResponse{
		Targets: responses,
		Total:   len(responses),
		Source:  string(source),
	})
}

func (s *Server) handleCreateTarget(w http.ResponseWriter, r *http.Request) {
	var req TargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	target := s.requestToTarget(&req)

	// Get storages for validation
	storages, _ := s.configService.ListStorages(r.Context()) //nolint:errcheck // validation will catch missing storage
	if err := configservice.ValidateTarget(target, storages); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Validation failed: %v", err))
		return
	}

	if err := s.configService.CreateTarget(r.Context(), target); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			respondError(w, http.StatusConflict, "Target with this name already exists")
		} else {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create target: %v", err))
		}
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"message": "Target created successfully",
		"name":    target.Name,
	})
}

func (s *Server) handleUpdateTarget(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "Target name is required")
		return
	}

	var req TargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	req.Name = name
	target := s.requestToTarget(&req)

	storages, _ := s.configService.ListStorages(r.Context()) //nolint:errcheck // validation will catch missing storage
	if err := configservice.ValidateTarget(target, storages); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Validation failed: %v", err))
		return
	}

	if err := s.configService.UpdateTarget(r.Context(), name, target); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update target: %v", err))
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Target updated successfully",
		"name":    name,
	})
}

func (s *Server) handleUpdateTargetSchedule(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "Target name is required")
		return
	}

	var req UpdateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	if err := s.configService.UpdateTargetSchedule(r.Context(), name, req.Schedule); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update schedule: %v", err))
		}
		return
	}

	// Trigger hot reload so the scheduler picks up the new schedule
	s.triggerReload()

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Target schedule updated successfully",
		"name":    name,
	})
}

func (s *Server) handleDeleteTarget(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "Target name is required")
		return
	}

	if err := s.configService.DeleteTarget(r.Context(), name); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete target: %v", err))
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Target deleted successfully",
		"name":    name,
	})
}

// RestoreTarget handlers

func (s *Server) handleListRestoreTargetsConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	restoreTargets, err := s.configService.ListRestoreTargets(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list restore targets: %v", err))
		return
	}

	source, _ := s.configLoader.GetConfigSource(ctx) //nolint:errcheck // default to empty string if fails

	var responses []RestoreTargetResponse
	for _, rt := range restoreTargets {
		responses = append(responses, s.restoreTargetToResponse(rt))
	}

	respondJSON(w, http.StatusOK, ListRestoreTargetsConfigResponse{
		RestoreTargets: responses,
		Total:          len(responses),
		Source:         string(source),
	})
}

func (s *Server) handleCreateRestoreTarget(w http.ResponseWriter, r *http.Request) {
	var req RestoreTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	rt := s.requestToRestoreTarget(&req)

	storages, _ := s.configService.ListStorages(r.Context()) //nolint:errcheck // validation will catch missing storage
	targets, _ := s.configService.ListTargets(r.Context())   //nolint:errcheck // validation will catch missing target
	targetsMap := make(map[string]*config.Target)
	for _, t := range targets {
		targetsMap[t.Name] = t
	}

	if err := configservice.ValidateRestoreTarget(rt, storages, targetsMap); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Validation failed: %v", err))
		return
	}

	if err := s.configService.CreateRestoreTarget(r.Context(), rt); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			respondError(w, http.StatusConflict, "Restore target with this name already exists")
		} else {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create restore target: %v", err))
		}
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"message": "Restore target created successfully",
		"name":    rt.Name,
	})
}

func (s *Server) handleUpdateRestoreTarget(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "Restore target name is required")
		return
	}

	var req RestoreTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	req.Name = name
	rt := s.requestToRestoreTarget(&req)

	storages, _ := s.configService.ListStorages(r.Context()) //nolint:errcheck // validation will catch missing storage
	targets, _ := s.configService.ListTargets(r.Context())   //nolint:errcheck // validation will catch missing target
	targetsMap := make(map[string]*config.Target)
	for _, t := range targets {
		targetsMap[t.Name] = t
	}

	if err := configservice.ValidateRestoreTarget(rt, storages, targetsMap); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Validation failed: %v", err))
		return
	}

	if err := s.configService.UpdateRestoreTarget(r.Context(), name, rt); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update restore target: %v", err))
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Restore target updated successfully",
		"name":    name,
	})
}

func (s *Server) handleDeleteRestoreTarget(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "Restore target name is required")
		return
	}

	if err := s.configService.DeleteRestoreTarget(r.Context(), name); err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete restore target: %v", err))
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Restore target deleted successfully",
		"name":    name,
	})
}

// Global config handlers

func (s *Server) handleGetGlobalConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	configs, err := s.configService.ListGlobalConfig(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get global config: %v", err))
		return
	}

	source, _ := s.configLoader.GetConfigSource(ctx) //nolint:errcheck // default to empty string if fails

	respondJSON(w, http.StatusOK, GlobalConfigResponse{
		DefaultStorage: configs["default_storage"],
		LogLevel:       configs["log_level"],
		LogFormat:      configs["log_format"],
		Source:         string(source),
	})
}

func (s *Server) handleUpdateGlobalConfig(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		respondError(w, http.StatusBadRequest, "Config key is required")
		return
	}

	var req UpdateGlobalConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	if err := s.configService.SetGlobalConfig(r.Context(), key, req.Value); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update config: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Global config updated successfully",
		"key":     key,
		"value":   req.Value,
	})
}

// Utility handlers

func (s *Server) handleMigrateConfig(w http.ResponseWriter, r *http.Request) {
	if s.cfg == nil {
		respondError(w, http.StatusBadRequest, "No YAML configuration available to migrate")
		return
	}

	if err := s.configService.ImportFromYAML(r.Context(), s.cfg); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Migration failed: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, MigrateConfigResponse{
		Message:        "Configuration migrated successfully from YAML to database",
		StoragesCount:  len(s.cfg.Storages),
		NotifiersCount: len(s.cfg.Notifiers),
		TargetsCount:   len(s.cfg.Targets),
	})
}

func (s *Server) handleReloadConfig(w http.ResponseWriter, r *http.Request) {
	// Trigger reload via channel (if daemon is set up to listen)
	if s.reloadChan != nil {
		select {
		case s.reloadChan <- struct{}{}:
			source, _ := s.configLoader.GetConfigSource(r.Context()) //nolint:errcheck // default to empty string if fails
			respondJSON(w, http.StatusOK, ReloadConfigResponse{
				Message: "Configuration reload triggered successfully",
				Source:  string(source),
			})
		case <-time.After(2 * time.Second):
			respondError(w, http.StatusServiceUnavailable, "Reload request timed out")
		}
	} else {
		respondError(w, http.StatusServiceUnavailable, "Hot reload not available")
	}
}

func (s *Server) handleGetConfigSource(w http.ResponseWriter, r *http.Request) {
	source, err := s.configLoader.GetConfigSource(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get config source: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, ConfigSourceResponse{
		Source: string(source),
	})
}

func (s *Server) handleImportConfig(w http.ResponseWriter, r *http.Request) {
	// Limit body size to 1MB
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req ConfigImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err.Error() == "http: request body too large" {
			respondError(w, http.StatusRequestEntityTooLarge, "Request body too large (max 1MB)")
			return
		}
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	if strings.TrimSpace(req.YAMLContent) == "" {
		respondError(w, http.StatusBadRequest, "YAML content is required")
		return
	}

	if req.ConflictMode != "override" && req.ConflictMode != "skip" {
		respondError(w, http.StatusBadRequest, "conflict_mode must be 'override' or 'skip'")
		return
	}

	// Parse YAML
	cfg, err := config.ParseFromString(req.YAMLContent)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse YAML: %v", err))
		return
	}

	ctx := r.Context()
	resp := ConfigImportResponse{
		DryRun: req.DryRun,
		Storages: ResourceImportSummary{
			Created: []string{}, Updated: []string{}, Skipped: []string{}, Failed: []FailedImportResource{},
		},
		Notifiers: ResourceImportSummary{
			Created: []string{}, Updated: []string{}, Skipped: []string{}, Failed: []FailedImportResource{},
		},
		Targets: ResourceImportSummary{
			Created: []string{}, Updated: []string{}, Skipped: []string{}, Failed: []FailedImportResource{},
		},
		RestoreTargets: ResourceImportSummary{
			Created: []string{}, Updated: []string{}, Skipped: []string{}, Failed: []FailedImportResource{},
		},
		GlobalConfig: GlobalConfigImportSummary{
			Updated: []string{}, Skipped: []string{}, Failed: []FailedImportConfig{},
		},
	}

	// Import storages
	for name, storage := range cfg.Storages {
		storage.Name = name
		if err := configservice.ValidateStorage(storage); err != nil {
			resp.Storages.Failed = append(resp.Storages.Failed, FailedImportResource{Name: name, Error: fmt.Sprintf("validation failed: %v", err)})
			continue
		}
		existing, _ := s.configService.GetStorage(ctx, name)
		if existing != nil {
			if req.ConflictMode == "skip" {
				resp.Storages.Skipped = append(resp.Storages.Skipped, name)
			} else {
				if !req.DryRun {
					if err := s.configService.UpdateStorage(ctx, name, storage); err != nil {
						resp.Storages.Failed = append(resp.Storages.Failed, FailedImportResource{Name: name, Error: err.Error()})
						continue
					}
				}
				resp.Storages.Updated = append(resp.Storages.Updated, name)
			}
		} else {
			if !req.DryRun {
				if err := s.configService.CreateStorage(ctx, storage); err != nil {
					resp.Storages.Failed = append(resp.Storages.Failed, FailedImportResource{Name: name, Error: err.Error()})
					continue
				}
			}
			resp.Storages.Created = append(resp.Storages.Created, name)
		}
	}

	// Import notifiers
	for name, notifier := range cfg.Notifiers {
		if err := configservice.ValidateNotifier(notifier); err != nil {
			resp.Notifiers.Failed = append(resp.Notifiers.Failed, FailedImportResource{Name: name, Error: fmt.Sprintf("validation failed: %v", err)})
			continue
		}
		existing, _ := s.configService.GetNotifier(ctx, name)
		if existing != nil {
			if req.ConflictMode == "skip" {
				resp.Notifiers.Skipped = append(resp.Notifiers.Skipped, name)
			} else {
				if !req.DryRun {
					if err := s.configService.UpdateNotifier(ctx, name, notifier); err != nil {
						resp.Notifiers.Failed = append(resp.Notifiers.Failed, FailedImportResource{Name: name, Error: err.Error()})
						continue
					}
				}
				resp.Notifiers.Updated = append(resp.Notifiers.Updated, name)
			}
		} else {
			if !req.DryRun {
				if err := s.configService.CreateNotifier(ctx, name, notifier); err != nil {
					resp.Notifiers.Failed = append(resp.Notifiers.Failed, FailedImportResource{Name: name, Error: err.Error()})
					continue
				}
			}
			resp.Notifiers.Created = append(resp.Notifiers.Created, name)
		}
	}

	// Import targets (validate against storages from both YAML and existing DB)
	dbStorages, _ := s.configService.ListStorages(ctx) //nolint:errcheck // validation will catch missing storage
	// Merge YAML storages with DB storages for validation
	mergedStorages := make(map[string]*config.Storage)
	for k, v := range dbStorages {
		mergedStorages[k] = v
	}
	for k, v := range cfg.Storages {
		v.Name = k
		mergedStorages[k] = v
	}

	for _, target := range cfg.Targets {
		if err := configservice.ValidateTarget(target, mergedStorages); err != nil {
			resp.Targets.Failed = append(resp.Targets.Failed, FailedImportResource{Name: target.Name, Error: fmt.Sprintf("validation failed: %v", err)})
			continue
		}
		existing, _ := s.configService.GetTarget(ctx, target.Name)
		if existing != nil {
			if req.ConflictMode == "skip" {
				resp.Targets.Skipped = append(resp.Targets.Skipped, target.Name)
			} else {
				if !req.DryRun {
					if err := s.configService.UpdateTarget(ctx, target.Name, target); err != nil {
						resp.Targets.Failed = append(resp.Targets.Failed, FailedImportResource{Name: target.Name, Error: err.Error()})
						continue
					}
				}
				resp.Targets.Updated = append(resp.Targets.Updated, target.Name)
			}
		} else {
			if !req.DryRun {
				if err := s.configService.CreateTarget(ctx, target); err != nil {
					resp.Targets.Failed = append(resp.Targets.Failed, FailedImportResource{Name: target.Name, Error: err.Error()})
					continue
				}
			}
			resp.Targets.Created = append(resp.Targets.Created, target.Name)
		}
	}

	// Import restore targets
	dbTargets, _ := s.configService.ListTargets(ctx) //nolint:errcheck // validation will catch missing target
	targetsMap := make(map[string]*config.Target)
	for _, t := range dbTargets {
		targetsMap[t.Name] = t
	}
	for _, t := range cfg.Targets {
		targetsMap[t.Name] = t
	}

	for _, rt := range cfg.RestoreTargets {
		if err := configservice.ValidateRestoreTarget(rt, mergedStorages, targetsMap); err != nil {
			resp.RestoreTargets.Failed = append(resp.RestoreTargets.Failed, FailedImportResource{Name: rt.Name, Error: fmt.Sprintf("validation failed: %v", err)})
			continue
		}
		existing, _ := s.configService.GetRestoreTarget(ctx, rt.Name)
		if existing != nil {
			if req.ConflictMode == "skip" {
				resp.RestoreTargets.Skipped = append(resp.RestoreTargets.Skipped, rt.Name)
			} else {
				if !req.DryRun {
					if err := s.configService.UpdateRestoreTarget(ctx, rt.Name, rt); err != nil {
						resp.RestoreTargets.Failed = append(resp.RestoreTargets.Failed, FailedImportResource{Name: rt.Name, Error: err.Error()})
						continue
					}
				}
				resp.RestoreTargets.Updated = append(resp.RestoreTargets.Updated, rt.Name)
			}
		} else {
			if !req.DryRun {
				if err := s.configService.CreateRestoreTarget(ctx, rt); err != nil {
					resp.RestoreTargets.Failed = append(resp.RestoreTargets.Failed, FailedImportResource{Name: rt.Name, Error: err.Error()})
					continue
				}
			}
			resp.RestoreTargets.Created = append(resp.RestoreTargets.Created, rt.Name)
		}
	}

	// Import global config
	globalKeys := map[string]string{
		"default_storage": cfg.DefaultStorage,
		"log_level":       cfg.LogLevel,
		"log_format":      cfg.LogFormat,
	}
	for key, value := range globalKeys {
		if value == "" {
			continue
		}
		if !req.DryRun {
			if err := s.configService.SetGlobalConfig(ctx, key, value); err != nil {
				resp.GlobalConfig.Failed = append(resp.GlobalConfig.Failed, FailedImportConfig{Key: key, Error: err.Error()})
				continue
			}
		}
		resp.GlobalConfig.Updated = append(resp.GlobalConfig.Updated, key)
	}

	// Compute totals
	resp.TotalCreated = len(resp.Storages.Created) + len(resp.Notifiers.Created) + len(resp.Targets.Created) + len(resp.RestoreTargets.Created)
	resp.TotalUpdated = len(resp.Storages.Updated) + len(resp.Notifiers.Updated) + len(resp.Targets.Updated) + len(resp.RestoreTargets.Updated) + len(resp.GlobalConfig.Updated)
	resp.TotalSkipped = len(resp.Storages.Skipped) + len(resp.Notifiers.Skipped) + len(resp.Targets.Skipped) + len(resp.RestoreTargets.Skipped) + len(resp.GlobalConfig.Skipped)
	resp.TotalFailed = len(resp.Storages.Failed) + len(resp.Notifiers.Failed) + len(resp.Targets.Failed) + len(resp.RestoreTargets.Failed) + len(resp.GlobalConfig.Failed)
	resp.HasErrors = resp.TotalFailed > 0

	respondJSON(w, http.StatusOK, resp)
}

// triggerReload sends a non-blocking reload signal to the daemon
func (s *Server) triggerReload() {
	if s.reloadChan != nil {
		select {
		case s.reloadChan <- struct{}{}:
		default:
			// Reload already pending
		}
	}
}

// Helper functions for converting between types

func (s *Server) storageToResponse(storage *config.Storage) StorageResponse {
	configMap := make(map[string]interface{})

	switch storage.Type {
	case "local":
		if storage.Path != "" {
			configMap["path"] = storage.Path
		}
	case "s3":
		if storage.Bucket != "" {
			configMap["bucket"] = storage.Bucket
		}
		if storage.Region != "" {
			configMap["region"] = storage.Region
		}
		if storage.AccessKeyID != "" {
			configMap["access_key_id"] = storage.AccessKeyID
		}
		if storage.EndpointURL != "" {
			configMap["endpoint_url"] = storage.EndpointURL
		}
		// secret_access_key filtered out
		if storage.SecretAccessKey != "" {
			configMap["secret_access_key"] = "***REDACTED***"
		}
	case "sftp":
		if storage.Host != "" {
			configMap["host"] = storage.Host
		}
		if storage.Port > 0 {
			configMap["port"] = storage.Port
		}
		if storage.Username != "" {
			configMap["username"] = storage.Username
		}
		// password filtered out
		if storage.Password != "" {
			configMap["password"] = "***REDACTED***"
		}
	}

	return StorageResponse{
		Name:    storage.Name,
		Type:    storage.Type,
		Keep:    storage.Keep,
		Config:  configMap,
		Enabled: true,
	}
}

func (s *Server) requestToStorage(req *StorageRequest) *config.Storage {
	storage := &config.Storage{
		Name: req.Name,
		Type: req.Type,
		Keep: req.Keep,
	}

	// Type-specific fields
	switch req.Type {
	case "local":
		if path, ok := req.Config["path"].(string); ok {
			storage.Path = path
		}
	case "s3":
		if bucket, ok := req.Config["bucket"].(string); ok {
			storage.Bucket = bucket
		}
		if region, ok := req.Config["region"].(string); ok {
			storage.Region = region
		}
		if accessKeyID, ok := req.Config["access_key_id"].(string); ok {
			storage.AccessKeyID = accessKeyID
		}
		if endpointURL, ok := req.Config["endpoint_url"].(string); ok {
			storage.EndpointURL = endpointURL
		}
		storage.SecretAccessKey = req.SecretAccessKey
	case "sftp":
		if host, ok := req.Config["host"].(string); ok {
			storage.Host = host
		}
		if port, ok := req.Config["port"].(float64); ok {
			storage.Port = int(port)
		}
		if username, ok := req.Config["username"].(string); ok {
			storage.Username = username
		}
		storage.Password = req.Password
	}

	return storage
}

func (s *Server) notifierToResponse(notifier *config.Notifier) NotifierResponse {
	configMap := make(map[string]interface{})

	// Common fields
	if notifier.URL != "" {
		configMap["url"] = notifier.URL
	}
	if notifier.Channel != "" {
		configMap["channel"] = notifier.Channel
	}

	// Email fields
	if notifier.SMTPHost != "" {
		configMap["smtp_host"] = notifier.SMTPHost
		configMap["smtp_port"] = notifier.SMTPPort
		configMap["smtp_username"] = notifier.SMTPUsername
		configMap["smtp_from"] = notifier.SMTPFrom
		configMap["smtp_to"] = notifier.SMTPTo
		configMap["smtp_use_tls"] = notifier.SMTPUseTLS
		if notifier.SMTPPassword != "" {
			configMap["smtp_password"] = "***REDACTED***"
		}
	}

	// Webhook fields
	if notifier.WebhookMethod != "" {
		configMap["webhook_method"] = notifier.WebhookMethod
	}
	if len(notifier.WebhookHeaders) > 0 {
		configMap["webhook_headers"] = notifier.WebhookHeaders
	}
	if notifier.WebhookAuth != nil {
		authMap := make(map[string]interface{})
		authMap["type"] = notifier.WebhookAuth.Type
		if notifier.WebhookAuth.Username != "" {
			authMap["username"] = notifier.WebhookAuth.Username
		}
		if notifier.WebhookAuth.HeaderName != "" {
			authMap["header_name"] = notifier.WebhookAuth.HeaderName
		}
		// Secrets filtered
		if notifier.WebhookAuth.Password != "" {
			authMap["password"] = "***REDACTED***"
		}
		if notifier.WebhookAuth.Token != "" {
			authMap["token"] = "***REDACTED***"
		}
		if notifier.WebhookAuth.HeaderValue != "" {
			authMap["header_value"] = "***REDACTED***"
		}
		configMap["webhook_auth"] = authMap
	}

	return NotifierResponse{
		Name:      "",
		Type:      notifier.Type,
		OnSuccess: notifier.OnSuccess,
		Config:    configMap,
		Enabled:   true,
	}
}

func (s *Server) requestToNotifier(req *NotifierRequest) *config.Notifier {
	notifier := &config.Notifier{
		Type:      req.Type,
		OnSuccess: req.OnSuccess,
	}

	// Common fields
	if url, ok := req.Config["url"].(string); ok {
		notifier.URL = url
	}
	if channel, ok := req.Config["channel"].(string); ok {
		notifier.Channel = channel
	}

	// Email fields
	if smtpHost, ok := req.Config["smtp_host"].(string); ok {
		notifier.SMTPHost = smtpHost
	}
	if smtpPort, ok := req.Config["smtp_port"].(float64); ok {
		notifier.SMTPPort = int(smtpPort)
	}
	if smtpUsername, ok := req.Config["smtp_username"].(string); ok {
		notifier.SMTPUsername = smtpUsername
	}
	if smtpFrom, ok := req.Config["smtp_from"].(string); ok {
		notifier.SMTPFrom = smtpFrom
	}
	if smtpTo, ok := req.Config["smtp_to"].([]interface{}); ok {
		for _, to := range smtpTo {
			if toStr, ok := to.(string); ok {
				notifier.SMTPTo = append(notifier.SMTPTo, toStr)
			}
		}
	}
	if smtpUseTLS, ok := req.Config["smtp_use_tls"].(bool); ok {
		notifier.SMTPUseTLS = smtpUseTLS
	}
	notifier.SMTPPassword = req.SMTPPassword

	// Webhook fields
	if webhookMethod, ok := req.Config["webhook_method"].(string); ok {
		notifier.WebhookMethod = webhookMethod
	}
	if webhookHeaders, ok := req.Config["webhook_headers"].(map[string]interface{}); ok {
		notifier.WebhookHeaders = make(map[string]string)
		for k, v := range webhookHeaders {
			if vStr, ok := v.(string); ok {
				notifier.WebhookHeaders[k] = vStr
			}
		}
	}
	if webhookAuth, ok := req.Config["webhook_auth"].(map[string]interface{}); ok {
		notifier.WebhookAuth = &config.WebhookAuth{}
		if authType, ok := webhookAuth["type"].(string); ok {
			notifier.WebhookAuth.Type = authType
		}
		if username, ok := webhookAuth["username"].(string); ok {
			notifier.WebhookAuth.Username = username
		}
		if headerName, ok := webhookAuth["header_name"].(string); ok {
			notifier.WebhookAuth.HeaderName = headerName
		}
		notifier.WebhookAuth.Password = req.WebhookAuthPassword
		notifier.WebhookAuth.Token = req.WebhookAuthToken
		notifier.WebhookAuth.HeaderValue = req.WebhookAuthHeaderVal
	}

	return notifier
}

func (s *Server) targetToResponse(target *config.Target) TargetResponse {
	connMap := make(map[string]interface{})
	if target.Conn != nil {
		connMap["type"] = target.Conn.Type
		if target.Conn.User != "" {
			connMap["user"] = target.Conn.User
		}
		if target.Conn.Database != "" {
			connMap["database"] = target.Conn.Database
		}
		if target.Conn.Host != "" {
			connMap["host"] = target.Conn.Host
		}
		if target.Conn.Port > 0 {
			connMap["port"] = target.Conn.Port
		}
		// password filtered
		if target.Conn.Password != "" {
			connMap["password"] = "***REDACTED***"
		}
	}

	var storageName string
	if target.Storage != nil && target.Storage.Enabled {
		storageName = target.Storage.Name
	}

	var compress *CompressionConfig
	if target.Compress != nil {
		compress = &CompressionConfig{
			Enabled: target.Compress.Enabled,
			Type:    target.Compress.Type,
		}
	}

	return TargetResponse{
		Name:           target.Name,
		Connection:     connMap,
		StorageName:    storageName,
		Schedule:       target.Schedule,
		Compress:       compress,
		ExcludeTables:  target.ExcludeTables,
		AdditionalArgs: target.AdditionalArgs,
		Enabled:        true,
	}
}

func (s *Server) requestToTarget(req *TargetRequest) *config.Target {
	target := &config.Target{
		Name: req.Name,
		Conn: &config.Connection{
			Type:     req.Connection.Type,
			User:     req.Connection.User,
			Password: req.Connection.Password,
			Database: req.Connection.Database,
			Host:     req.Connection.Host,
			Port:     req.Connection.Port,
		},
		ExcludeTables:  req.ExcludeTables,
		AdditionalArgs: req.AdditionalArgs,
		Schedule:       req.Schedule,
	}

	if req.StorageName != "" {
		target.Storage = &config.TargetStorage{
			Enabled: true,
			Name:    req.StorageName,
		}
	}

	if req.Compress != nil {
		target.Compress = &config.CompressionOpts{
			Enabled: req.Compress.Enabled,
			Type:    req.Compress.Type,
		}
	}

	return target
}

func (s *Server) restoreTargetToResponse(rt *config.RestoreTarget) RestoreTargetResponse {
	connMap := make(map[string]interface{})
	if rt.Conn != nil {
		connMap["type"] = rt.Conn.Type
		if rt.Conn.User != "" {
			connMap["user"] = rt.Conn.User
		}
		if rt.Conn.Database != "" {
			connMap["database"] = rt.Conn.Database
		}
		if rt.Conn.Host != "" {
			connMap["host"] = rt.Conn.Host
		}
		if rt.Conn.Port > 0 {
			connMap["port"] = rt.Conn.Port
		}
		if rt.Conn.Password != "" {
			connMap["password"] = "***REDACTED***"
		}
	}

	var storageName string
	if rt.Storage != nil && rt.Storage.Enabled {
		storageName = rt.Storage.Name
	}

	return RestoreTargetResponse{
		Name:         rt.Name,
		Connection:   connMap,
		StorageName:  storageName,
		SourceTarget: rt.SourceTarget,
		Description:  rt.Description,
		Enabled:      true,
	}
}

func (s *Server) requestToRestoreTarget(req *RestoreTargetRequest) *config.RestoreTarget {
	rt := &config.RestoreTarget{
		Name: req.Name,
		Conn: &config.Connection{
			Type:     req.Connection.Type,
			User:     req.Connection.User,
			Password: req.Connection.Password,
			Database: req.Connection.Database,
			Host:     req.Connection.Host,
			Port:     req.Connection.Port,
		},
		SourceTarget: req.SourceTarget,
		Description:  req.Description,
	}

	if req.StorageName != "" {
		rt.Storage = &config.TargetStorage{
			Enabled: true,
			Name:    req.StorageName,
		}
	}

	return rt
}
