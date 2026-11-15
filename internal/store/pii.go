package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"

	"zoo-go/internal/security"
)

const (
	piiScopeStudentName   = "student_verifications.real_name"
	piiScopeStudentNo     = "student_verifications.student_no"
	piiScopeStudentMajor  = "student_verifications.major"
	piiScopeCHSICode      = "student_verifications.chsi_code"
	piiScopeIdentityName  = "identity_verifications.real_name"
	piiScopeIdentityID    = "identity_verifications.id_number"
	piiScopeAddressDetail = "owner_addresses.detail"
	piiScopeAddressName   = "owner_addresses.location_name"
	piiScopeTaskAddress   = "tasks.address_detail"
	piiScopeTaskLocation  = "tasks.location_name"
)

func (s *Store) encryptPII(scope, value string) (string, error) {
	if s.pii == nil || value == "" {
		return value, nil
	}
	return s.pii.Encrypt(scope, value)
}

func (s *Store) decryptPII(scope, value string) (string, error) {
	if s.pii == nil || value == "" {
		return value, nil
	}
	return s.pii.Decrypt(scope, value)
}

func (s *Store) lookupPII(scope, normalized string) string {
	if s.pii != nil {
		return s.pii.Lookup(scope, normalized)
	}
	sum := sha256.Sum256([]byte(scope + "\x00" + normalized))
	return hex.EncodeToString(sum[:])
}

func (s *Store) migratePIIData() error {
	if s.pii == nil {
		return nil
	}
	updates, err := s.encryptedColumnUpdates("student_verifications", "id", []string{
		"real_name", "student_no", "major", "chsi_code",
	}, map[string]string{
		"real_name": piiScopeStudentName, "student_no": piiScopeStudentNo,
		"major": piiScopeStudentMajor, "chsi_code": piiScopeCHSICode,
	})
	if err != nil {
		return err
	}
	if err := s.applyPIIUpdates(updates); err != nil {
		return err
	}
	if err := s.migrateStudentHashes(); err != nil {
		return err
	}
	updates, err = s.encryptedColumnUpdates("owner_addresses", "owner_id", []string{"detail", "location_name"}, map[string]string{
		"detail": piiScopeAddressDetail, "location_name": piiScopeAddressName,
	})
	if err != nil {
		return err
	}
	if err := s.applyPIIUpdates(updates); err != nil {
		return err
	}
	updates, err = s.encryptedColumnUpdates("tasks", "id", []string{"address_detail", "location_name"}, map[string]string{
		"address_detail": piiScopeTaskAddress, "location_name": piiScopeTaskLocation,
	})
	if err != nil {
		return err
	}
	return s.applyPIIUpdates(updates)
}

type piiColumnUpdate struct {
	table    string
	idColumn string
	id       string
	columns  []string
	values   []string
}

func (s *Store) encryptedColumnUpdates(table, idColumn string, columns []string, scopes map[string]string) ([]piiColumnUpdate, error) {
	selectColumns := idColumn
	notEmpty := make([]string, 0, len(columns))
	for _, column := range columns {
		selectColumns += ", " + column
		notEmpty = append(notEmpty, column+` <> ''`)
	}
	query := `SELECT ` + selectColumns + ` FROM ` + table + ` WHERE ` + joinStrings(notEmpty, " OR ")
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("read %s for PII migration: %w", table, err)
	}
	defer rows.Close()
	updates := []piiColumnUpdate{}
	for rows.Next() {
		values := make([]sql.NullString, len(columns))
		scan := make([]any, 0, len(columns)+1)
		var id string
		scan = append(scan, &id)
		for index := range values {
			scan = append(scan, &values[index])
		}
		if err := rows.Scan(scan...); err != nil {
			return nil, err
		}
		update := piiColumnUpdate{table: table, idColumn: idColumn, id: id}
		for index, column := range columns {
			if !values[index].Valid || values[index].String == "" {
				continue
			}
			value := values[index].String
			if security.IsEncrypted(value) {
				if _, err := s.pii.Decrypt(scopes[column], value); err != nil {
					return nil, fmt.Errorf("verify encrypted %s.%s: %w", table, column, err)
				}
				continue
			}
			encrypted, err := s.pii.Encrypt(scopes[column], value)
			if err != nil {
				return nil, err
			}
			update.columns = append(update.columns, column)
			update.values = append(update.values, encrypted)
		}
		if len(update.columns) > 0 {
			updates = append(updates, update)
		}
	}
	return updates, rows.Err()
}

func (s *Store) applyPIIUpdates(updates []piiColumnUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, update := range updates {
		assignments := ""
		args := make([]any, 0, len(update.values)+1)
		for index, column := range update.columns {
			if index > 0 {
				assignments += ", "
			}
			assignments += column + " = ?"
			args = append(args, update.values[index])
		}
		args = append(args, update.id)
		if _, err := tx.Exec(`UPDATE `+update.table+` SET `+assignments+` WHERE `+update.idColumn+` = ?`, args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) migrateStudentHashes() error {
	rows, err := s.db.Query(`SELECT id, chsi_code, chsi_code_hash FROM student_verifications`)
	if err != nil {
		return err
	}
	type hashUpdate struct {
		id   string
		hash string
	}
	updates := []hashUpdate{}
	for rows.Next() {
		var id, code, currentHash string
		if err := rows.Scan(&id, &code, &currentHash); err != nil {
			rows.Close()
			return err
		}
		plain, err := s.pii.Decrypt(piiScopeCHSICode, code)
		if err != nil {
			rows.Close()
			return err
		}
		hash := s.lookupPII(piiScopeCHSICode, plain)
		if hash != currentHash {
			updates = append(updates, hashUpdate{id: id, hash: hash})
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(updates) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, update := range updates {
		if _, err := tx.Exec(`UPDATE student_verifications SET chsi_code_hash = ? WHERE id = ?`, update.hash, update.id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func joinStrings(values []string, separator string) string {
	result := ""
	for index, value := range values {
		if index > 0 {
			result += separator
		}
		result += value
	}
	return result
}
