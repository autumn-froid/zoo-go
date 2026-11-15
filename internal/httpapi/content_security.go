package httpapi

import (
	"errors"
	"io"
	"net/http"

	"zoo-go/internal/store"
)

func (s *Server) wechatContentSecurityVerify(w http.ResponseWriter, r *http.Request) {
	if s.callbackVerifier == nil {
		http.NotFound(w, r)
		return
	}
	echo, err := s.callbackVerifier.VerifyURL(r)
	if err != nil {
		s.logger.WarnContext(r.Context(), "reject wechat content security URL verification", "error", err)
		http.Error(w, "fail", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(echo))
}

func (s *Server) wechatContentSecurityNotify(w http.ResponseWriter, r *http.Request) {
	if s.callbackVerifier == nil {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "fail", http.StatusBadRequest)
		return
	}
	notification, err := s.callbackVerifier.Verify(r, body)
	if err != nil {
		s.logger.WarnContext(r.Context(), "reject wechat content security callback", "error", err)
		http.Error(w, "fail", http.StatusForbidden)
		return
	}
	err = s.store.HandleMachineMediaResult(
		notification.TraceID, machineVerdict(notification.Suggest),
		notification.Label, notification.ErrCode, notification.ErrMsg, s.now(),
	)
	if errors.Is(err, store.ErrMediaReviewNotFound) {
		s.logger.WarnContext(r.Context(), "wechat content security trace not found", "trace_id", notification.TraceID)
		http.Error(w, "fail", http.StatusInternalServerError)
		return
	} else if err != nil {
		s.logger.ErrorContext(r.Context(), "apply wechat content security callback", "trace_id", notification.TraceID, "error", err)
		http.Error(w, "fail", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("success"))
}
