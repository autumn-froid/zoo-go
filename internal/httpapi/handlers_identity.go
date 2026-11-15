package httpapi

import (
	"net/http"
	"strings"

	"zoo-go/internal/agreement"
	"zoo-go/internal/domain"
)

func (s *Server) identityVerification(w http.ResponseWriter, r *http.Request) {
	verification, err := s.store.IdentityVerificationForUser(currentUser(r).ID)
	if err != nil {
		writeError(w, err)
		return
	}
	s.writeData(w, map[string]any{
		"verification":       verification,
		"consentVersion":     agreement.IdentityConsentVersion,
		"consentContent":     agreement.IdentityConsentContent,
		"consentContentHash": agreement.IdentityConsentHash(),
	})
}

func (s *Server) submitIdentityVerification(w http.ResponseWriter, r *http.Request) {
	var req domain.SubmitIdentityVerificationRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	req.ClientIP, req.UserAgent = requestMetadata(r)
	verification, err := s.store.SubmitIdentityVerification(currentUser(r).ID, req, s.now())
	if err != nil {
		writeError(w, err)
		return
	}
	s.writeDataStatus(w, http.StatusCreated, verification)
}

func (s *Server) adminIdentityVerifications(w http.ResponseWriter, r *http.Request) {
	verifications, err := s.store.IdentityVerificationApplications(query(r, "status", domain.IdentityPending))
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	s.writeData(w, verifications)
}

func (s *Server) adminApproveIdentityVerification(w http.ResponseWriter, r *http.Request) {
	if err := s.store.ApproveIdentityVerification(r.PathValue("id"), "admin", s.now()); err != nil {
		writeError(w, err)
		return
	}
	s.recordAdminAudit("IDENTITY_VERIFICATION_APPROVED", "identity_verification", r.PathValue("id"), "人工核验通过")
	s.writeData(w, map[string]bool{"ok": true})
}

func (s *Server) adminRejectIdentityVerification(w http.ResponseWriter, r *http.Request) {
	var req domain.RejectIdentityVerificationRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	if err := s.store.RejectIdentityVerification(r.PathValue("id"), "admin", req.Reason, s.now()); err != nil {
		writeError(w, err)
		return
	}
	s.recordAdminAudit("IDENTITY_VERIFICATION_REJECTED", "identity_verification", r.PathValue("id"), req.Reason)
	s.writeData(w, map[string]bool{"ok": true})
}

func requestMetadata(r *http.Request) (string, string) {
	ip := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		ip = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	if ip == "" {
		ip = r.RemoteAddr
	}
	return ip, r.UserAgent()
}
