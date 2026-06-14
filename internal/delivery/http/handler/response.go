package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jahrulnr/gosite/pkg/apperror"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	appErr := apperror.From(err)
	if appErr == nil {
		return
	}
	writeJSON(w, appErr.HTTPStatus, appErr.Body())
}

func decodeJSON(r *http.Request, dst interface{}) error {
	if r.Body == nil {
		return apperror.New(apperror.CodeInvalidInput, "empty body")
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return apperror.Wrap(apperror.CodeInvalidInput, "invalid json", err)
	}
	return nil
}

func parseID(s string) (int64, error) {
	if s == "" {
		return 0, apperror.New(apperror.CodeInvalidInput, "missing id")
	}
	var id int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, apperror.New(apperror.CodeInvalidInput, "invalid id")
		}
		id = id*10 + int64(c-'0')
	}
	return id, nil
}
