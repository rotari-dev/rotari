package webui

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/rotari-dev/rotari/internal/project"
	stateinternal "github.com/rotari-dev/rotari/internal/state"
)

// webUnlockResult is what `rotari unlock` decides, as the CLI and MCP get
// it from project.Unlock, with the words the Recover dialog shows.
type webUnlockResult struct {
	project.UnlockResult
	// Refusal explains why recovery is not allowed, as the CLI prints it.
	Refusal string `json:"refusal,omitempty"`
	// RerunCommand reruns a released run's failed and unfinished jobs.
	RerunCommand string `json:"rerun_command,omitempty"`
	// Error is why an apply failed.
	Error string `json:"error,omitempty"`
}

type webUnlockRequest struct {
	ProjectName   string `json:"project_name"`
	RunID         string `json:"run_id"`
	IfRevision    string `json:"if_revision"`
	AssumeStopped bool   `json:"assume_stopped"`
}

func newWebUnlockResult(paths stateinternal.ProjectPaths, result project.UnlockResult) webUnlockResult {
	response := webUnlockResult{UnlockResult: result}
	if result.RunID != "" && !result.Allowed {
		response.Refusal = (&project.UnlockRefusedError{Result: result}).Error()
	}
	if result.RunID != "" && result.Action == "release" {
		response.RerunCommand = project.RerunCommand(paths, result.RunID)
	}
	return response
}

// unlockProjectPaths resolves an existing project of baseDir for unlock.
func unlockProjectPaths(baseDir, projectName, runID string) (stateinternal.ProjectPaths, error) {
	if !stateinternal.IsValidPathElement(projectName) {
		return stateinternal.ProjectPaths{}, fmt.Errorf("invalid project_name %q", projectName)
	}
	if runID != "" && !stateinternal.IsValidPathElement(runID) {
		return stateinternal.ProjectPaths{}, fmt.Errorf("invalid run_id %q", runID)
	}
	paths, err := stateinternal.ResolveProjectPaths(baseDir, projectName)
	if err != nil {
		return stateinternal.ProjectPaths{}, err
	}
	if info, err := os.Stat(paths.ProjectDir); err != nil || !info.IsDir() {
		return stateinternal.ProjectPaths{}, fmt.Errorf("project %q does not exist", projectName)
	}
	return paths, nil
}

// handleUnlock previews (GET) or applies (POST) `rotari unlock` for one
// project of baseDir through project.Unlock, as the MCP preview and apply
// tools do. An apply needs control and the preview's revision; a refusal or
// a changed project answers 409.
func (s site) handleUnlock(baseDir string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodGet:
			query := request.URL.Query()
			assumeStopped := false
			switch query.Get("assume_stopped") {
			case "", "false":
			case "true":
				assumeStopped = true
			default:
				writeWebError(writer, fmt.Errorf("assume_stopped must be true or false"))
				return
			}
			paths, err := unlockProjectPaths(baseDir, query.Get("project_name"), query.Get("run_id"))
			if err != nil {
				writeWebError(writer, err)
				return
			}
			result, err := project.Unlock(paths, query.Get("run_id"), project.Guard{DryRun: true}, assumeStopped)
			if err != nil {
				writeWebError(writer, err)
				return
			}
			writeWebJSON(writer, newWebUnlockResult(paths, result))
		case http.MethodPost:
			if !s.AllowControl {
				forbiddenReadOnly(writer)
				return
			}
			var unlock webUnlockRequest
			if err := json.NewDecoder(request.Body).Decode(&unlock); err != nil {
				writeWebError(writer, err)
				return
			}
			if unlock.IfRevision == "" {
				writeWebError(writer, errors.New("if_revision is required; take it from the preview"))
				return
			}
			paths, err := unlockProjectPaths(baseDir, unlock.ProjectName, unlock.RunID)
			if err != nil {
				writeWebError(writer, err)
				return
			}
			result, err := project.Unlock(paths, unlock.RunID, project.Guard{IfRevision: unlock.IfRevision}, unlock.AssumeStopped)
			response := newWebUnlockResult(paths, result)
			var refusal *project.UnlockRefusedError
			switch {
			case err == nil:
				writeWebJSON(writer, response)
			case errors.As(err, &refusal), errors.Is(err, project.ErrRevisionChanged):
				response.Error = err.Error()
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(writer).Encode(response)
			default:
				writeWebError(writer, err)
			}
		default:
			methodNotAllowed(writer)
		}
	}
}
