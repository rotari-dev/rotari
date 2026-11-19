package webui

import (
	"net/http"
	"time"
)

// The panel's Commands dropdown asks terminalCommandsName under a shell's path for the
// commands that the shells of a group ran, as the Keeper heard them from the shells'
// marks. The dropdown lists the focused pane's own blocks for the shell's own project;
// this is what it offers for another one.

// terminalCommandsName follows a shell's path for the commands of its group, or with
// basedir and project parameters, of another basedir's or project.
const terminalCommandsName = "commands"

// terminalCommandEntry is a command that ended. Exit is absent when the shell gave none.
type terminalCommandEntry struct {
	Command   string    `json:"command"`
	Exit      *int      `json:"exit,omitempty"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}

// terminalCommandsResponse is what the dropdown offers; Basedir, Basedirs, Project and
// Projects are as in terminalDirsResponse.
type terminalCommandsResponse struct {
	Commands []terminalCommandEntry `json:"commands"`
	Basedir  string                 `json:"basedir"`
	Basedirs []webBaseDir           `json:"basedirs"`
	Project  string                 `json:"project"`
	Projects []string               `json:"projects"`
}

// serveTerminalCommands answers GET with the commands of a shell's group, newest first.
// It attaches to nothing, so it takes the shell from no page.
func (s site) serveTerminalCommands(writer http.ResponseWriter, request *http.Request, id string) {
	writer.Header().Set("Cache-Control", "no-store")
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	choice, ok := s.terminalChosenGroup(writer, request, id)
	if !ok {
		return
	}
	response := terminalCommandsResponse{Commands: []terminalCommandEntry{}, Basedir: choice.Basedir, Basedirs: choice.Basedirs, Project: choice.Project, Projects: choice.Projects}
	for _, entry := range s.Terminal.Commands(choice.Group) {
		response.Commands = append(response.Commands, terminalCommandEntry{Command: entry.Command, Exit: entry.Exit, StartedAt: entry.StartedAt, EndedAt: entry.EndedAt})
	}
	writeTerminalJSON(writer, http.StatusOK, response)
}
