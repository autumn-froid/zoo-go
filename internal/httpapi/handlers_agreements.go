package httpapi

import (
	"net/http"
	"strings"

	"zoo-go/internal/agreement"
)

func (s *Server) currentAgreement(w http.ResponseWriter, r *http.Request) {
	var role string
	switch strings.ToLower(strings.TrimSpace(query(r, "role", "owner"))) {
	case "owner":
		role = agreement.RoleOwner
	case "sitter", "student":
		role = agreement.RoleSitter
	default:
		writeJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "VALIDATION_ERROR", Message: "协议角色无效"}})
		return
	}
	definition, _ := agreement.Current(role)
	s.writeData(w, definition)
}

func (s *Server) orderAgreement(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.store.OrderAgreement(r.PathValue("id"), currentUser(r).ID)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	s.writeData(w, snapshot)
}
