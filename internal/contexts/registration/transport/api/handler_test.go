package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"salt/hackethon/internal/contexts/registration/application"
	"salt/hackethon/internal/contexts/registration/domain"
	"salt/hackethon/internal/contexts/registration/transport/api"
)

const longTTL = time.Hour

func init() {
	gin.SetMode(gin.TestMode)
}

// fakeRepository mirrors the application package's test fake: dataMu guards the
// in-memory rows, lockMu is held for the whole WithTransaction call whenever the
// handler takes the advisory lock, so concurrent HTTP requests are serialized
// exactly as concurrent DB transactions would be.
type fakeRepository struct {
	lockMu sync.Mutex
	dataMu sync.Mutex
	byID   map[int64]*domain.Registration
	byCode map[string]int64
	nextID int64
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		byID:   make(map[int64]*domain.Registration),
		byCode: make(map[string]int64),
	}
}

func (f *fakeRepository) WithTransaction(ctx context.Context, fn func(tx domain.Repository) error) error {
	tx := &fakeTx{repo: f}
	err := fn(tx)
	if tx.locked {
		f.lockMu.Unlock()
	}
	return err
}

func (f *fakeRepository) Lock(ctx context.Context) error {
	f.lockMu.Lock()
	return nil
}

func (f *fakeRepository) ExpirePending(ctx context.Context, now time.Time) error {
	f.dataMu.Lock()
	defer f.dataMu.Unlock()
	for _, r := range f.byID {
		if r.Status == domain.StatusPending && !now.Before(r.ExpiresAt) {
			r.Status = domain.StatusExpired
		}
	}
	return nil
}

func (f *fakeRepository) CountActive(ctx context.Context, now time.Time) (int, error) {
	f.dataMu.Lock()
	defer f.dataMu.Unlock()
	count := 0
	for _, r := range f.byID {
		if r.IsActive(now) {
			count++
		}
	}
	return count, nil
}

func (f *fakeRepository) ExistsActiveByEmail(ctx context.Context, email string, now time.Time) (bool, error) {
	f.dataMu.Lock()
	defer f.dataMu.Unlock()
	for _, r := range f.byID {
		if r.Email == email && r.IsActive(now) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeRepository) Create(ctx context.Context, reg *domain.Registration) error {
	f.dataMu.Lock()
	defer f.dataMu.Unlock()
	f.nextID++
	reg.ID = f.nextID
	cp := *reg
	f.byID[reg.ID] = &cp
	f.byCode[reg.RegisCode] = reg.ID
	return nil
}

func (f *fakeRepository) FindByCode(ctx context.Context, code string) (*domain.Registration, error) {
	f.dataMu.Lock()
	defer f.dataMu.Unlock()
	id, ok := f.byCode[code]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *f.byID[id]
	return &cp, nil
}

func (f *fakeRepository) Save(ctx context.Context, reg *domain.Registration) error {
	f.dataMu.Lock()
	defer f.dataMu.Unlock()
	cp := *reg
	f.byID[reg.ID] = &cp
	return nil
}

func (f *fakeRepository) count() int {
	f.dataMu.Lock()
	defer f.dataMu.Unlock()
	return len(f.byID)
}

type fakeTx struct {
	repo   *fakeRepository
	locked bool
}

func (t *fakeTx) WithTransaction(ctx context.Context, fn func(tx domain.Repository) error) error {
	return fn(t)
}
func (t *fakeTx) Lock(ctx context.Context) error {
	t.repo.lockMu.Lock()
	t.locked = true
	return nil
}
func (t *fakeTx) ExpirePending(ctx context.Context, now time.Time) error {
	return t.repo.ExpirePending(ctx, now)
}
func (t *fakeTx) CountActive(ctx context.Context, now time.Time) (int, error) {
	return t.repo.CountActive(ctx, now)
}
func (t *fakeTx) ExistsActiveByEmail(ctx context.Context, email string, now time.Time) (bool, error) {
	return t.repo.ExistsActiveByEmail(ctx, email, now)
}
func (t *fakeTx) Create(ctx context.Context, reg *domain.Registration) error {
	return t.repo.Create(ctx, reg)
}
func (t *fakeTx) FindByCode(ctx context.Context, code string) (*domain.Registration, error) {
	return t.repo.FindByCode(ctx, code)
}
func (t *fakeTx) Save(ctx context.Context, reg *domain.Registration) error {
	return t.repo.Save(ctx, reg)
}

func newTestRouter(quotaTotal int, ttl time.Duration) (*gin.Engine, *fakeRepository, *fakeMailer) {
	repo := newFakeRepository()
	mail := newFakeMailer()
	registerHandler := application.NewRegisterHandler(repo, quotaTotal, ttl)
	availabilityHandler := application.NewAvailabilityHandler(repo, quotaTotal)
	verifyHandler := application.NewVerifyHandler(repo)

	router := gin.New()
	api.NewHandler(registerHandler, availabilityHandler, verifyHandler, mail).RegisterRoutes(router.Group("/api"))
	return router, repo, mail
}

// fakeMailer records Send calls on a buffered channel so tests can assert on
// them without racing the async goroutine that triggers the send.
type fakeMailer struct {
	calls chan mailCall
}

type mailCall struct {
	To, Subject, Body string
}

func newFakeMailer() *fakeMailer {
	return &fakeMailer{calls: make(chan mailCall, 10)}
}

func (m *fakeMailer) Send(ctx context.Context, to, subject, body string) error {
	m.calls <- mailCall{To: to, Subject: subject, Body: body}
	return nil
}

func doRequest(router *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestRegister_SendsVerificationEmail(t *testing.T) {
	router, _, mail := newTestRouter(30, longTTL)

	rec := doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
		"name": "Karyawan A", "email": "a@corp.example",
	})
	var body struct {
		RegisURL string `json:"regis_url"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)

	select {
	case call := <-mail.calls:
		if call.To != "a@corp.example" {
			t.Errorf("To = %q, want a@corp.example", call.To)
		}
		if !bytesContains(call.Body, body.RegisURL) {
			t.Errorf("email body does not contain regis_url %q:\n%s", body.RegisURL, call.Body)
		}
	case <-time.After(time.Second):
		t.Fatal("mailer.Send was not called within 1s of a successful registration")
	}
}

func TestRegister_NilMailerDoesNotPanic(t *testing.T) {
	repo := newFakeRepository()
	router := gin.New()
	api.NewHandler(
		application.NewRegisterHandler(repo, 30, longTTL),
		application.NewAvailabilityHandler(repo, 30),
		application.NewVerifyHandler(repo),
		nil,
	).RegisterRoutes(router.Group("/api"))

	rec := doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
		"name": "Karyawan A", "email": "a@corp.example",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

// failingMailer always fails, to exercise the log-and-continue path:
// registration must still succeed even if the email can't be delivered.
type failingMailer struct{ done chan struct{} }

func (m *failingMailer) Send(ctx context.Context, to, subject, body string) error {
	defer close(m.done)
	return fmt.Errorf("smtp unavailable")
}

func TestRegister_SucceedsEvenIfEmailSendFails(t *testing.T) {
	repo := newFakeRepository()
	mail := &failingMailer{done: make(chan struct{})}
	router := gin.New()
	api.NewHandler(
		application.NewRegisterHandler(repo, 30, longTTL),
		application.NewAvailabilityHandler(repo, 30),
		application.NewVerifyHandler(repo),
		mail,
	).RegisterRoutes(router.Group("/api"))

	rec := doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
		"name": "Karyawan A", "email": "a@corp.example",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	select {
	case <-mail.done:
	case <-time.After(time.Second):
		t.Fatal("mailer.Send was not called")
	}
}

func TestRegister_DoesNotSendEmailOnFailure(t *testing.T) {
	router, _, mail := newTestRouter(1, longTTL)
	doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
		"name": "Karyawan C", "email": "c@corp.example",
	})
	<-mail.calls // drain the successful registration's send

	doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
		"name": "Karyawan D", "email": "d@corp.example",
	})

	select {
	case call := <-mail.calls:
		t.Fatalf("unexpected Send call for a rejected registration: %+v", call)
	case <-time.After(100 * time.Millisecond):
		// expected: no second call
	}
}

func TestRegister_Success(t *testing.T) {
	router, _, _ := newTestRouter(30, longTTL)
	for i := 0; i < 28; i++ {
		rec := doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
			"name": "Seed", "email": seedEmail(i),
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("seed %d: status = %d, body = %s", i, rec.Code, rec.Body.String())
		}
	}

	rec := doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
		"name": "Karyawan A", "email": "a@corp.example",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["registrationId"] != "REG-0029" {
		t.Errorf("registrationId = %v, want REG-0029", body["registrationId"])
	}
	if body["status"] != "pending" {
		t.Errorf("status = %v, want pending", body["status"])
	}
	regisURL, _ := body["regis_url"].(string)
	if regisURL == "" || !bytesContains(regisURL, "/api/registrations/verify/") {
		t.Errorf("regis_url = %q, want it to contain /api/registrations/verify/", regisURL)
	}
	availability := body["availability"].(map[string]any)
	if availability["total"].(float64) != 30 || availability["registered"].(float64) != 29 || availability["remaining"].(float64) != 1 {
		t.Errorf("availability = %+v, want {30 29 1}", availability)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	router, _, _ := newTestRouter(30, longTTL)
	doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
		"name": "Budi", "email": "budi@corp.example",
	})

	rec := doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
		"name": "Budi", "email": " budi@corp.example ",
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"]["code"] != "EMAIL_ALREADY_REGISTERED" {
		t.Errorf("code = %v, want EMAIL_ALREADY_REGISTERED", body["error"]["code"])
	}
}

func TestRegister_QuotaFull(t *testing.T) {
	router, _, _ := newTestRouter(1, longTTL)
	doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
		"name": "Karyawan C", "email": "c@corp.example",
	})

	rec := doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
		"name": "Karyawan D", "email": "d@corp.example",
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"]["code"] != "QUOTA_FULL" {
		t.Errorf("code = %v, want QUOTA_FULL", body["error"]["code"])
	}
}

func TestRegister_ValidationError(t *testing.T) {
	router, _, _ := newTestRouter(30, longTTL)
	rec := doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
		"name": "", "email": "abc",
	})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Fields  []struct {
				Field   string `json:"field"`
				Message string `json:"message"`
			} `json:"fields"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("code = %v, want VALIDATION_ERROR", body.Error.Code)
	}
	if len(body.Error.Fields) != 2 || body.Error.Fields[0].Field != "name" || body.Error.Fields[1].Field != "email" {
		t.Errorf("fields = %+v, want name then email", body.Error.Fields)
	}
}

func TestAvailability_Success(t *testing.T) {
	router, _, _ := newTestRouter(30, longTTL)
	for i := 0; i < 12; i++ {
		doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
			"name": "Seed", "email": seedEmail(i),
		})
	}

	rec := doRequest(router, http.MethodGet, "/api/availability", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]float64
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["total"] != 30 || body["registered"] != 12 || body["remaining"] != 18 {
		t.Errorf("body = %+v, want {30 12 18}", body)
	}
}

func TestVerify_Success(t *testing.T) {
	router, _, _ := newTestRouter(30, longTTL)
	rec := doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
		"name": "Karyawan A", "email": "a@corp.example",
	})
	var regBody struct {
		RegisURL string `json:"regis_url"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &regBody)
	code := regBody.RegisURL[len(regBody.RegisURL)-32:]

	verifyRec := doRequest(router, http.MethodGet, "/api/registrations/verify/"+code, nil)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", verifyRec.Code, verifyRec.Body.String())
	}
	var body map[string]string
	_ = json.Unmarshal(verifyRec.Body.Bytes(), &body)
	if body["status"] != "success" {
		t.Errorf("status = %v, want success", body["status"])
	}
}

func TestVerify_NotFound(t *testing.T) {
	router, _, _ := newTestRouter(30, longTTL)
	rec := doRequest(router, http.MethodGet, "/api/registrations/verify/does-not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"]["code"] != "REGISTRATION_NOT_FOUND" {
		t.Errorf("code = %v, want REGISTRATION_NOT_FOUND", body["error"]["code"])
	}
}

func TestVerify_Expired_ReleasesQuota(t *testing.T) {
	router, _, _ := newTestRouter(1, 30*time.Millisecond)
	rec := doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
		"name": "Karyawan A", "email": "a@corp.example",
	})
	var regBody struct {
		RegisURL string `json:"regis_url"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &regBody)
	code := regBody.RegisURL[len(regBody.RegisURL)-32:]

	time.Sleep(80 * time.Millisecond)

	verifyRec := doRequest(router, http.MethodGet, "/api/registrations/verify/"+code, nil)
	if verifyRec.Code != http.StatusGone {
		t.Fatalf("status = %d, body = %s", verifyRec.Code, verifyRec.Body.String())
	}
	var body map[string]map[string]any
	_ = json.Unmarshal(verifyRec.Body.Bytes(), &body)
	if body["error"]["code"] != "REGISTRATION_EXPIRED" {
		t.Errorf("code = %v, want REGISTRATION_EXPIRED", body["error"]["code"])
	}

	availRec := doRequest(router, http.MethodGet, "/api/availability", nil)
	var avail map[string]float64
	_ = json.Unmarshal(availRec.Body.Bytes(), &avail)
	if avail["remaining"] != 1 {
		t.Errorf("remaining = %v, want 1 (quota must be returned)", avail["remaining"])
	}

	// The freed slot must be takeable by a new registration.
	rec2 := doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
		"name": "Karyawan B", "email": "b@corp.example",
	})
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected the reclaimed slot to be takeable, status = %d, body = %s", rec2.Code, rec2.Body.String())
	}
}

// TestRegister_ConcurrentLastSlot_OnlyOneSucceeds drives the brief's race
// scenario through the real HTTP handlers end to end.
func TestRegister_ConcurrentLastSlot_OnlyOneSucceeds(t *testing.T) {
	router, repo, _ := newTestRouter(1, longTTL)

	var wg sync.WaitGroup
	codes := make([]int, 2)
	emails := []string{"first@corp.example", "second@corp.example"}
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			defer wg.Done()
			rec := doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
				"name": "Concurrent", "email": emails[i],
			})
			codes[i] = rec.Code
		}(i)
	}
	wg.Wait()

	created, conflicts := 0, 0
	for _, code := range codes {
		switch code {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		}
	}
	if created != 1 || conflicts != 1 {
		t.Fatalf("created=%d conflicts=%d codes=%v, want exactly one of each", created, conflicts, codes)
	}
	if count := repo.count(); count != 1 {
		t.Fatalf("final row count = %d, want 1", count)
	}
}

func seedEmail(i int) string {
	return fmt.Sprintf("seed%d@corp.example", i)
}

func bytesContains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}

// erroringRepository simulates an infrastructure failure (e.g. DB unreachable)
// so the generic INTERNAL_ERROR / 503 branches can be exercised without a real DB.
type erroringRepository struct{ err error }

func (r *erroringRepository) WithTransaction(ctx context.Context, fn func(tx domain.Repository) error) error {
	return fn(r)
}
func (r *erroringRepository) Lock(ctx context.Context) error { return r.err }
func (r *erroringRepository) ExpirePending(ctx context.Context, now time.Time) error {
	return r.err
}
func (r *erroringRepository) CountActive(ctx context.Context, now time.Time) (int, error) {
	return 0, r.err
}
func (r *erroringRepository) ExistsActiveByEmail(ctx context.Context, email string, now time.Time) (bool, error) {
	return false, r.err
}
func (r *erroringRepository) Create(ctx context.Context, reg *domain.Registration) error {
	return r.err
}
func (r *erroringRepository) FindByCode(ctx context.Context, code string) (*domain.Registration, error) {
	return nil, r.err
}
func (r *erroringRepository) Save(ctx context.Context, reg *domain.Registration) error {
	return r.err
}

func TestRegister_MalformedJSON(t *testing.T) {
	router, _, _ := newTestRouter(30, longTTL)
	req := httptest.NewRequest(http.MethodPost, "/api/registrations", bytes.NewReader([]byte("{not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestRegister_InternalError(t *testing.T) {
	repo := &erroringRepository{err: fmt.Errorf("db unavailable")}
	router := gin.New()
	api.NewHandler(
		application.NewRegisterHandler(repo, 30, longTTL),
		application.NewAvailabilityHandler(repo, 30),
		application.NewVerifyHandler(repo),
		newFakeMailer(),
	).RegisterRoutes(router.Group("/api"))

	rec := doRequest(router, http.MethodPost, "/api/registrations", map[string]string{
		"name": "Karyawan A", "email": "a@corp.example",
	})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"]["code"] != "INTERNAL_ERROR" {
		t.Errorf("code = %v, want INTERNAL_ERROR", body["error"]["code"])
	}
}

func TestAvailability_InternalError(t *testing.T) {
	repo := &erroringRepository{err: fmt.Errorf("db unavailable")}
	router := gin.New()
	api.NewHandler(
		application.NewRegisterHandler(repo, 30, longTTL),
		application.NewAvailabilityHandler(repo, 30),
		application.NewVerifyHandler(repo),
		newFakeMailer(),
	).RegisterRoutes(router.Group("/api"))

	rec := doRequest(router, http.MethodGet, "/api/availability", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"]["code"] != "INTERNAL_ERROR" {
		t.Errorf("code = %v, want INTERNAL_ERROR", body["error"]["code"])
	}
}

func TestVerify_InternalError(t *testing.T) {
	repo := &erroringRepository{err: fmt.Errorf("db unavailable")}
	router := gin.New()
	api.NewHandler(
		application.NewRegisterHandler(repo, 30, longTTL),
		application.NewAvailabilityHandler(repo, 30),
		application.NewVerifyHandler(repo),
		newFakeMailer(),
	).RegisterRoutes(router.Group("/api"))

	rec := doRequest(router, http.MethodGet, "/api/registrations/verify/some-code", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}
