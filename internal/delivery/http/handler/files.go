package handler

import (
	"net/http"
	"strconv"

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
	result, err := h.svc.Browse(r.Context(), path)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Read handles GET /files/content.
func (h *FilesHandler) Read(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	content, err := h.svc.Read(r.Context(), path)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, content)
}

// Raw handles GET /files/raw.
func (h *FilesHandler) Raw(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	resolved, entry, err := h.svc.ResolveFile(r.Context(), path)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("X-GoSite-File-Kind", entry.Kind)
	w.Header().Set("X-GoSite-File-Size", strconv.FormatInt(entry.Size, 10))
	http.ServeFile(w, r, resolved)
}

// Save handles PUT /files/content.
func (h *FilesHandler) Save(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.Save(r.Context(), body.Path, body.Content); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "saved"})
}

// Create handles POST /files for directory/file creation and upload.
func (h *FilesHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(256 << 20); err == nil && r.MultipartForm != nil {
		path := r.FormValue("path")
		uploaded := 0
		for _, key := range []string{"files", "file"} {
			for _, header := range r.MultipartForm.File[key] {
				file, ferr := header.Open()
				if ferr != nil {
					writeError(w, ferr)
					return
				}
				if err := h.svc.Upload(r.Context(), path, header.Filename, file); err != nil {
					_ = file.Close()
					writeError(w, err)
					return
				}
				_ = file.Close()
				uploaded++
			}
		}
		if uploaded > 0 {
			writeJSON(w, http.StatusCreated, map[string]interface{}{"message": "uploaded", "count": uploaded})
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

// BatchSave handles POST /files/batch-save.
func (h *FilesHandler) BatchSave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Files []files.SaveInput `json:"files"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.BatchSave(r.Context(), body.Files); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "saved"})
}

// BatchDelete handles POST /files/batch-delete.
func (h *FilesHandler) BatchDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Paths []string `json:"paths"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.BatchDelete(r.Context(), body.Paths); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
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
