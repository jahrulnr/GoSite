package handler

import (
	"net/http"
	"strings"

	"github.com/jahrulnr/gosite/pkg/apperror"
)

func requestID(r *http.Request) (int64, error) {
	return resourceID(r, "websites")
}

func resourceID(r *http.Request, resource string) (int64, error) {
	if id := r.PathValue("id"); id != "" {
		return parseID(id)
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, p := range parts {
		if p == resource && i+1 < len(parts) {
			next := parts[i+1]
			if resource == "websites" && next == "validate" {
				continue
			}
			return parseID(next)
		}
	}
	return 0, apperror.New(apperror.CodeInvalidInput, "missing id")
}

func containerID(r *http.Request) (string, error) {
	if id := r.PathValue("id"); id != "" {
		return id, nil
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, p := range parts {
		if p == "containers" && i+1 < len(parts) {
			return parts[i+1], nil
		}
	}
	return "", apperror.New(apperror.CodeInvalidInput, "missing container id")
}
