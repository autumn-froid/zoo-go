package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"zoo-go/internal/domain"
)

func (s *Server) adminOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := s.store.AdminOverview(s.now())
	if err != nil {
		writeError(w, err)
		return
	}
	s.writeData(w, overview)
}

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.AdminUsers(query(r, "q", ""), query(r, "status", "ALL"), limitQuery(r, 100))
	if err != nil {
		writeError(w, err)
		return
	}
	s.writeData(w, users)
}

func (s *Server) adminUpdateUserStatus(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateUserStatusRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	userID := r.PathValue("id")
	if err := s.store.UpdateUserAccountStatus(userID, req.Status, req.Reason, s.now()); err != nil {
		writeError(w, err)
		return
	}
	action := "USER_ENABLED"
	if strings.EqualFold(req.Status, domain.AccountDisabled) {
		action = "USER_DISABLED"
	}
	s.recordAdminAudit(action, "user", userID, req.Reason)
	s.writeData(w, map[string]bool{"ok": true})
}

func (s *Server) adminOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := s.store.AdminOrders(query(r, "status", "ALL"), limitQuery(r, 100))
	if err != nil {
		writeError(w, err)
		return
	}
	s.writeData(w, orders)
}

func (s *Server) adminCommunityPosts(w http.ResponseWriter, r *http.Request) {
	posts, err := s.store.AdminCommunityPosts(query(r, "status", domain.PostPending), limitQuery(r, 100))
	if err != nil {
		writeError(w, err)
		return
	}
	s.writeData(w, posts)
}

func (s *Server) adminApproveCommunityPost(w http.ResponseWriter, r *http.Request) {
	postID := r.PathValue("id")
	if err := s.store.ApproveCommunityPost(postID, s.now()); err != nil {
		writeError(w, err)
		return
	}
	s.recordAdminAudit("COMMUNITY_POST_APPROVED", "community_post", postID, "")
	s.writeData(w, map[string]bool{"ok": true})
}

func (s *Server) adminRejectCommunityPost(w http.ResponseWriter, r *http.Request) {
	var req domain.RejectCommunityPostRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	postID := r.PathValue("id")
	if err := s.store.RejectCommunityPost(postID, req.Reason, s.now()); err != nil {
		writeError(w, err)
		return
	}
	s.recordAdminAudit("COMMUNITY_POST_REJECTED", "community_post", postID, req.Reason)
	s.writeData(w, map[string]bool{"ok": true})
}

func (s *Server) adminAuditLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := s.store.AdminAuditLogs(limitQuery(r, 100))
	if err != nil {
		writeError(w, err)
		return
	}
	s.writeData(w, logs)
}

func (s *Server) recordAdminAudit(action, resourceType, resourceID, detail string) {
	if err := s.store.AddAdminAudit("admin", action, resourceType, resourceID, detail, s.now()); err != nil {
		s.logger.Error("record admin audit", "action", action, "resource", resourceID, "error", err)
	}
}

func limitQuery(r *http.Request, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
