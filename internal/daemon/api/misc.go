package api

import "net/http"

func (ro *Router) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("health ok"))

}
