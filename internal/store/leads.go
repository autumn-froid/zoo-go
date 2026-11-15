package store

import (
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"zoo-go/internal/domain"
)

var chinaMobileRE = regexp.MustCompile(`^1[3-9]\d{9}$`)

const leadResubmitWindow = 24 * time.Hour

func (s *Store) CreatePilotLead(req domain.CreatePilotLeadRequest, now time.Time) (domain.PilotLead, error) {
	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role != domain.LeadRoleOwner && role != domain.LeadRoleStudent {
		return domain.PilotLead{}, validation("请选择身份：宠物主人或在校学生")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > 40 {
		return domain.PilotLead{}, validation("请填写姓名（40字以内）")
	}
	phone := normalizePhone(req.Phone)
	if !chinaMobileRE.MatchString(phone) {
		return domain.PilotLead{}, validation("请填写有效的中国大陆手机号")
	}
	community := strings.TrimSpace(req.Community)
	if utf8.RuneCountInString(community) > 80 {
		return domain.PilotLead{}, validation("小区或学校名称过长")
	}
	note := strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(note) > 500 {
		return domain.PilotLead{}, validation("备注请控制在 500 字以内")
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "website"
	}
	if utf8.RuneCountInString(source) > 40 {
		return domain.PilotLead{}, validation("来源标识无效")
	}

	cutoff := timestamp(now.Add(-leadResubmitWindow))
	var recentID string
	err := s.db.QueryRow(`
		SELECT id FROM pilot_leads
		WHERE phone = ? AND created_at >= ?
		ORDER BY created_at DESC LIMIT 1
	`, phone, cutoff).Scan(&recentID)
	if err == nil {
		return domain.PilotLead{}, rateLimited("该手机号近期已提交过申请，请等待运营联系")
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.PilotLead{}, err
	}

	lead := domain.PilotLead{
		ID:        newID("lead"),
		Role:      role,
		Name:      name,
		Phone:     phone,
		Community: community,
		Note:      note,
		Source:    source,
		Status:    domain.LeadStatusNew,
		CreatedAt: now.UTC(),
	}
	_, err = s.db.Exec(`
		INSERT INTO pilot_leads (id, role, name, phone, community, note, source, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, lead.ID, lead.Role, lead.Name, lead.Phone, lead.Community, lead.Note, lead.Source, lead.Status, timestamp(lead.CreatedAt))
	if err != nil {
		return domain.PilotLead{}, err
	}
	return lead, nil
}

func (s *Store) AdminPilotLeads(role, status string, limit int) ([]domain.PilotLead, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	args := make([]any, 0, 3)
	where := "WHERE 1=1"
	if role = strings.ToLower(strings.TrimSpace(role)); role == domain.LeadRoleOwner || role == domain.LeadRoleStudent {
		where += " AND role = ?"
		args = append(args, role)
	}
	if status = strings.ToUpper(strings.TrimSpace(status)); status != "" && status != "ALL" {
		where += " AND status = ?"
		args = append(args, status)
	}
	args = append(args, limit)
	rows, err := s.db.Query(`
		SELECT id, role, name, phone, community, note, source, status, created_at
		FROM pilot_leads
		`+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	leads := []domain.PilotLead{}
	for rows.Next() {
		var lead domain.PilotLead
		var createdAt string
		if err := rows.Scan(
			&lead.ID, &lead.Role, &lead.Name, &lead.Phone, &lead.Community,
			&lead.Note, &lead.Source, &lead.Status, &createdAt,
		); err != nil {
			return nil, err
		}
		lead.CreatedAt = parseTimestamp(createdAt)
		leads = append(leads, lead)
	}
	return leads, rows.Err()
}

func normalizePhone(value string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(value) {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
