package handler

import (
	"net/http"

	"github.com/jahrulnr/gosite/internal/service/files"
)

// FilesHandler serves file manager endpoints.
type FilesHandler struct {
	svc *files.Service
}

// NewFilesHandler returns a files handler.
func NewFilesHandler(svc *files.Service) *FilesHandler {
	return &FilesHandler{svc: svc}
}

// Browse handles GET /files.
func (h *FilesHandler) Browse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	entries, err := h.svc.Browse(r.Context(), path)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"entries": entries})
}

// Read handles GET /files/content.
func (h *FilesHandler) Read(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	content, err := h.svc.Read(r.Context(), path)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"content": content})
}

// Create handles POST /files for directory/file creation and upload.
func (h *FilesHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err == nil && r.MultipartForm != nil {
		if file, header, ferr := r.FormFile("file"); ferr == nil {
			defer file.Close()
			path := r.FormValue("path")
			if err := h.svc.Upload(r.Context(), path, header.Filename, file); err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]string{"message": "uploaded"})
			return
		}
	}

	var body struct {
		Type    string `json:"type"`
		Name    string `json:"name"`
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.Create(r.Context(), files.CreateInput{
		Type:    body.Type,
		Name:    body.Name,
		Path:    body.Path,
		Content: body.Content,
	}); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"message": "created"})
}

// Action handles POST /files/actions.
func (h *FilesHandler) Action(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Action string `json:"action"`
		Path   string `json:"path"`
		Mode   string `json:"mode"`
		ToPath string `json:"to_path"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.Action(r.Context(), files.ActionInput{
		Action: body.Action,
		Path:   body.Path,
		Mode:   body.Mode,
		ToPath: body.ToPath,
	}); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "ok"})
}

// Delete handles DELETE /files.
func (h *FilesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if err := h.svc.Delete(r.Context(), path); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}
