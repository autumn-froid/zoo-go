package store

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"zoo-go/internal/agreement"
	"zoo-go/internal/domain"
	"zoo-go/internal/security"
)

func newEncryptedTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "encrypted.db")
	cipher, err := security.NewCipher(strings.Repeat("1", 64), strings.Repeat("2", 64))
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	st, err := NewWithOptions(path, Options{PII: cipher})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st, path
}

func assertEncryptedNotPlaintext(t *testing.T, label, stored, plaintext string) {
	t.Helper()
	if !security.IsEncrypted(stored) {
		t.Fatalf("%s is not encrypted: %q", label, stored)
	}
	if strings.Contains(stored, plaintext) {
		t.Fatalf("%s contains plaintext", label)
	}
}

func TestStudentVerificationPIIIsEncryptedAndReadableByAdmin(t *testing.T) {
	st, path := newEncryptedTestStore(t)
	session, err := st.Login("wx_encrypted_student", "加密学生", "", "", time.Now())
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	code := "ENC20261115A1"
	if _, err := st.SubmitStudentVerification(session.Profile.ID, domain.SubmitStudentVerificationRequest{
		RealName: "加密学生", School: "中国农业大学", StudentNo: "20249999", Major: "动物医学",
		EnrollmentYear: 2024, ChsiCode: code,
	}, time.Now()); err != nil {
		t.Fatalf("SubmitStudentVerification() error = %v", err)
	}
	var realName, studentNo, major, chsiCode, chsiHash string
	if err := st.db.QueryRow(`
		SELECT real_name, student_no, major, chsi_code, chsi_code_hash
		FROM student_verifications WHERE user_id = ?
	`, session.Profile.ID).Scan(&realName, &studentNo, &major, &chsiCode, &chsiHash); err != nil {
		t.Fatalf("read encrypted row: %v", err)
	}
	assertEncryptedNotPlaintext(t, "real_name", realName, "加密学生")
	assertEncryptedNotPlaintext(t, "student_no", studentNo, "20249999")
	assertEncryptedNotPlaintext(t, "major", major, "动物医学")
	assertEncryptedNotPlaintext(t, "chsi_code", chsiCode, code)
	if chsiHash == hashToken(code) {
		t.Fatal("CHSI lookup hash still uses an unkeyed SHA-256")
	}
	applications, err := st.StudentVerificationApplications(domain.StudentPending)
	if err != nil || len(applications) != 1 {
		t.Fatalf("StudentVerificationApplications() = %+v, %v", applications, err)
	}
	if applications[0].RealName != "加密学生" || applications[0].StudentNo != "20249999" ||
		applications[0].Major != "动物医学" || applications[0].ChsiCode != code {
		t.Fatalf("admin application was not decrypted: %+v", applications[0])
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	cipher, _ := security.NewCipher(strings.Repeat("1", 64), strings.Repeat("2", 64))
	reopened, err := NewWithOptions(path, Options{PII: cipher})
	if err != nil {
		t.Fatalf("reopen NewWithOptions() error = %v", err)
	}
	defer reopened.Close()
	applications, err = reopened.StudentVerificationApplications(domain.StudentPending)
	if err != nil || len(applications) != 1 || applications[0].RealName != "加密学生" {
		t.Fatalf("reopened applications = %+v, %v", applications, err)
	}
}

func TestLegacyPlaintextPIIIsMigrated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-pii.db")
	st, err := New(path)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	session, err := st.Login("wx_legacy_pii", "旧学生", "", "", time.Now())
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if _, err := st.SubmitStudentVerification(session.Profile.ID, domain.SubmitStudentVerificationRequest{
		RealName: "旧学生", School: "中国农业大学", StudentNo: "20247777", Major: "动物科学",
		EnrollmentYear: 2024, ChsiCode: "LEGACY20261115",
	}, time.Now()); err != nil {
		t.Fatalf("SubmitStudentVerification() error = %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	cipher, _ := security.NewCipher(strings.Repeat("3", 64), strings.Repeat("4", 64))
	migrated, err := NewWithOptions(path, Options{PII: cipher})
	if err != nil {
		t.Fatalf("migrated NewWithOptions() error = %v", err)
	}
	defer migrated.Close()
	var realName, chsiCode string
	if err := migrated.db.QueryRow(`
		SELECT real_name, chsi_code FROM student_verifications WHERE user_id = ?
	`, session.Profile.ID).Scan(&realName, &chsiCode); err != nil {
		t.Fatalf("read migrated row: %v", err)
	}
	assertEncryptedNotPlaintext(t, "migrated real_name", realName, "旧学生")
	assertEncryptedNotPlaintext(t, "migrated chsi_code", chsiCode, "LEGACY20261115")
	applications, err := migrated.StudentVerificationApplications(domain.StudentPending)
	if err != nil || len(applications) != 1 || applications[0].RealName != "旧学生" {
		t.Fatalf("migrated applications = %+v, %v", applications, err)
	}
}

func TestIdentityVerificationStoresEncryptedIDAndMasksResponses(t *testing.T) {
	st, _ := newEncryptedTestStore(t)
	session, err := st.Login("wx_identity", "实名用户", "", "", time.Now())
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	idNumber := "11010519491231002X"
	verification, err := st.SubmitIdentityVerification(session.Profile.ID, domain.SubmitIdentityVerificationRequest{
		RealName: "张三", IDNumber: idNumber, Consent: true, ConsentVersion: agreement.IdentityConsentVersion,
		ClientIP: "127.0.0.1", UserAgent: "test-agent",
	}, time.Now())
	if err != nil {
		t.Fatalf("SubmitIdentityVerification() error = %v", err)
	}
	if verification.IDNumber != "" || verification.IDNumberMasked != "110***********002X" {
		t.Fatalf("user-facing verification = %+v", verification)
	}
	var encryptedName, encryptedID, hash, mask string
	if err := st.db.QueryRow(`
		SELECT real_name_enc, id_number_enc, id_number_hash, id_number_mask
		FROM identity_verifications WHERE id = ?
	`, verification.ID).Scan(&encryptedName, &encryptedID, &hash, &mask); err != nil {
		t.Fatalf("read identity row: %v", err)
	}
	assertEncryptedNotPlaintext(t, "identity real_name", encryptedName, "张三")
	assertEncryptedNotPlaintext(t, "identity id_number", encryptedID, idNumber)
	if hash == hashToken(idNumber) || mask != "110***********002X" {
		t.Fatalf("identity hash/mask = %q / %q", hash, mask)
	}
	applications, err := st.IdentityVerificationApplications(domain.IdentityPending)
	if err != nil || len(applications) != 1 {
		t.Fatalf("IdentityVerificationApplications() = %+v, %v", applications, err)
	}
	if applications[0].IDNumber != idNumber || applications[0].RealName != "张三" {
		t.Fatalf("admin identity application = %+v", applications[0])
	}
	if err := st.ApproveIdentityVerification(verification.ID, "admin", time.Now()); err != nil {
		t.Fatalf("ApproveIdentityVerification() error = %v", err)
	}
	profile, err := st.Profile(session.Profile.ID)
	if err != nil {
		t.Fatalf("Profile() error = %v", err)
	}
	if !profile.Verified || profile.IdentityStatus != domain.IdentityVerified || profile.IdentityMasked == "" {
		t.Fatalf("verified profile = %+v", profile)
	}
}

func TestIdentityVerificationRequiresConsentAndRejectsDuplicateNumber(t *testing.T) {
	st := newTestStore(t)
	first := createRawTestUser(t, st, "实名甲")
	second := createRawTestUser(t, st, "实名乙")
	idNumber := "11010519491231002X"
	if _, err := st.SubmitIdentityVerification(first.ID, domain.SubmitIdentityVerificationRequest{
		RealName: "实名甲", IDNumber: idNumber, Consent: false, ConsentVersion: agreement.IdentityConsentVersion,
	}, time.Now()); err == nil {
		t.Fatal("identity submission without consent was accepted")
	}
	if _, err := st.SubmitIdentityVerification(first.ID, domain.SubmitIdentityVerificationRequest{
		RealName: "实名甲", IDNumber: idNumber, Consent: true, ConsentVersion: agreement.IdentityConsentVersion,
	}, time.Now()); err != nil {
		t.Fatalf("first SubmitIdentityVerification() error = %v", err)
	}
	_, err := st.SubmitIdentityVerification(second.ID, domain.SubmitIdentityVerificationRequest{
		RealName: "实名乙", IDNumber: idNumber, Consent: true, ConsentVersion: agreement.IdentityConsentVersion,
	}, time.Now())
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Code != "STATE_CONFLICT" {
		t.Fatalf("duplicate identity error = %v", err)
	}
}

func createRawTestUser(t *testing.T, st *Store, name string) domain.Profile {
	t.Helper()
	session, err := st.Login("raw:"+name+"-"+newID("openid"), name, "", "", time.Now())
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	return session.Profile
}

func TestAddressAndTaskLocationAreEncryptedAtRest(t *testing.T) {
	st, _ := newEncryptedTestStore(t)
	owner := createTestUser(t, st, "地址主人")
	pet := createTestPet(t, st, owner.ID)
	task := publishTestTask(t, st, owner.ID, pet.ID)
	var locationName, addressDetail string
	if err := st.db.QueryRow(`SELECT location_name, address_detail FROM tasks WHERE id = ?`, task.ID).Scan(&locationName, &addressDetail); err != nil {
		t.Fatalf("read task PII: %v", err)
	}
	assertEncryptedNotPlaintext(t, "task location_name", locationName, "朗润园东门")
	assertEncryptedNotPlaintext(t, "task address_detail", addressDetail, "3号楼2单元401")
	loaded, err := st.Task(task.ID, owner.ID, "owner")
	if err != nil {
		t.Fatalf("Task() error = %v", err)
	}
	if loaded.LocationName != "朗润园东门" || loaded.AddressDetail != "3号楼2单元401" {
		t.Fatalf("decrypted task = %+v", loaded)
	}
	home, err := st.OwnerHome(owner.ID)
	if err != nil || home.Address == nil || home.Address.Detail != "3号楼2单元401" {
		t.Fatalf("OwnerHome() address = %+v, %v", home.Address, err)
	}
}

func TestAgreementSnapshotsGateAndFollowOrder(t *testing.T) {
	st := newTestStore(t)
	owner := createTestUser(t, st, "协议主人")
	student := createVerifiedStudent(t, st, "协议学生")
	otherStudent := createVerifiedStudent(t, st, "未中标学生")
	pet := createTestPet(t, st, owner.ID)
	now := time.Now()
	ownerVersion, ownerItems := testOwnerAgreement()
	task, err := st.PublishTask(domain.PublishTaskRequest{
		OwnerID: owner.ID, PetID: pet.ID, Services: []string{"喂食", "换水"},
		ServiceDate: now.AddDate(0, 0, 2).Format("2006-01-02"), TimeWindow: "09:00-10:00",
		Mode: domain.ModeBid, BudgetFen: 6000, BidDurationMin: 60,
		Community: "马连洼·朗润园附近", LocationName: "朗润园东门",
		Latitude: 39.9998, Longitude: 116.3269, AddressDetail: "3号楼2单元401",
		SafetyConfirmed: true, AgreementAccepted: true,
		AgreementVersion: ownerVersion, AgreementItems: ownerItems,
		ClientIP: "127.0.0.1", UserAgent: "owner-test",
	}, now)
	if err != nil {
		t.Fatalf("PublishTask() error = %v", err)
	}
	bidRequest := testBidRequest(student.ID, 4500)
	bidRequest.ClientIP = "127.0.0.2"
	bidRequest.UserAgent = "sitter-test"
	bid, err := st.SubmitBid(task.ID, bidRequest, now)
	if err != nil {
		t.Fatalf("SubmitBid() error = %v", err)
	}
	otherBidRequest := testBidRequest(otherStudent.ID, 4400)
	otherBidRequest.ClientIP = "127.0.0.3"
	otherBidRequest.UserAgent = "other-sitter-test"
	if _, err := st.SubmitBid(task.ID, otherBidRequest, now); err != nil {
		t.Fatalf("SubmitBid(other) error = %v", err)
	}
	awarded, err := st.AwardTask(task.ID, domain.AwardRequest{OwnerID: owner.ID, BidID: bid.MyBid.ID}, now)
	if err != nil {
		t.Fatalf("AwardTask() error = %v", err)
	}
	snapshot, err := st.OrderAgreement(awarded.OrderID, owner.ID)
	if err != nil {
		t.Fatalf("OrderAgreement() error = %v", err)
	}
	if snapshot.Version == "" || snapshot.Owner == nil || snapshot.Sitter == nil ||
		snapshot.Owner.ClientIP != "127.0.0.1" || snapshot.Sitter.ClientIP != "127.0.0.2" ||
		snapshot.Sitter.UserID != student.ID {
		t.Fatalf("agreement snapshot = %+v", snapshot)
	}
	var otherOrderID string
	if err := st.db.QueryRow(`
		SELECT order_id FROM agreement_acceptances WHERE task_id = ? AND role = ? AND user_id = ?
	`, task.ID, agreement.RoleSitter, otherStudent.ID).Scan(&otherOrderID); err != nil {
		t.Fatalf("read losing bidder agreement: %v", err)
	}
	if otherOrderID != "" {
		t.Fatalf("losing bidder agreement linked to order %s", otherOrderID)
	}
	if _, err := st.OrderAgreement(awarded.OrderID, createRawTestUser(t, st, "旁观者").ID); err == nil {
		t.Fatal("unrelated user could read the order agreement")
	}
}

func TestAgreementVersionMismatchIsRejected(t *testing.T) {
	st := newTestStore(t)
	owner := createTestUser(t, st, "旧协议主人")
	pet := createTestPet(t, st, owner.ID)
	now := time.Now()
	_, ownerItems := testOwnerAgreement()
	_, err := st.PublishTask(domain.PublishTaskRequest{
		OwnerID: owner.ID, PetID: pet.ID, Services: []string{"喂食"},
		ServiceDate: now.AddDate(0, 0, 2).Format("2006-01-02"), TimeWindow: "09:00-10:00",
		Mode: domain.ModeBid, BudgetFen: 6000, BidDurationMin: 60,
		Community: "马连洼·朗润园附近", LocationName: "朗润园东门",
		Latitude: 39.9998, Longitude: 116.3269, AddressDetail: "3号楼2单元401",
		SafetyConfirmed: true, AgreementAccepted: true,
		AgreementVersion: "outdated-version", AgreementItems: ownerItems,
	}, now)
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("outdated agreement error = %v", err)
	}
}
