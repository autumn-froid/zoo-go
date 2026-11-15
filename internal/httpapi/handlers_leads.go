package httpapi

import (
	"net/http"

	"zoo-go/internal/domain"
)

func (s *Server) createPilotLead(w http.ResponseWriter, r *http.Request) {
	var req domain.CreatePilotLeadRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	lead, err := s.store.CreatePilotLead(req, s.now())
	if err != nil {
		writeError(w, err)
		return
	}
	s.writeDataStatus(w, http.StatusCreated, map[string]any{
		"id":      lead.ID,
		"status":  lead.Status,
		"message": "已收到申请，运营将尽快与您联系",
	})
}

func (s *Server) adminPilotLeads(w http.ResponseWriter, r *http.Request) {
	leads, err := s.store.AdminPilotLeads(query(r, "role", "ALL"), query(r, "status", "NEW"), limitQuery(r, 100))
	if err != nil {
		writeError(w, err)
		return
	}
	s.writeData(w, leads)
}
