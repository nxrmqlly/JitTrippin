package httpx

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// BindJSON reads r.Body and parses it into v, sends ErrorJSON if error occours.
// Returns a bool (ok)
func BindJSON[T any](w http.ResponseWriter, r *http.Request, v *T) bool {
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		ErrorJSON(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return false
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(v); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "bad JSON: "+err.Error())
		return false
	}

	if dec.Decode(&struct{}{}) != io.EOF {
		ErrorJSON(w, http.StatusBadRequest, "JSON must contain only one object")
		return false
	}

	return true
}

func WriteJSON[T any](w http.ResponseWriter, status int, body T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type errBody struct {
	Error string `json:"error"`
}

func ErrorJSON(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, errBody{Error: msg})
}

func InternalServerError(w http.ResponseWriter) {
	ErrorJSON(w, http.StatusInternalServerError, "internal server error")
}
