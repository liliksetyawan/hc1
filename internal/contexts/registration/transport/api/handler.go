package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"salt/hackethon/internal/contexts/registration/application"
	"salt/hackethon/internal/contexts/registration/domain"
)

// Mailer is implemented by internal/platform/mailer.SMTPMailer. Defined here,
// on the consumer side, so the transport layer only depends on the shape it
// needs — it's a plain interface, not a domain/application concern, because
// building the verification link requires the HTTP request's host, which
// the application layer deliberately doesn't know about (see buildRegisURL).
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

type registerRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type availabilityResponse struct {
	Total      int `json:"total"`
	Registered int `json:"registered"`
	Remaining  int `json:"remaining"`
}

type registerResponse struct {
	RegistrationID string               `json:"registrationId"`
	Name           string               `json:"name"`
	Email          string               `json:"email"`
	Status         string               `json:"status"`
	RegisURL       string               `json:"regis_url"`
	Availability   availabilityResponse `json:"availability"`
}

type verifyResponse struct {
	RegistrationID string `json:"registrationId"`
	Status         string `json:"status"`
	Message        string `json:"message"`
}

type errorField struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type errorBody struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Fields  []errorField `json:"fields,omitempty"`
}

type Handler struct {
	register     *application.RegisterHandler
	availability *application.AvailabilityHandler
	verify       *application.VerifyHandler
	mailer       Mailer
}

func NewHandler(register *application.RegisterHandler, availability *application.AvailabilityHandler, verify *application.VerifyHandler, mailer Mailer) *Handler {
	return &Handler{register: register, availability: availability, verify: verify, mailer: mailer}
}

func (h *Handler) RegisterRoutes(router gin.IRouter) {
	router.POST("/registrations", h.handleRegister)
	router.GET("/availability", h.handleAvailability)
	router.GET("/registrations/verify/:code", h.handleVerify)
}

func (h *Handler) handleRegister(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusUnprocessableEntity, errorBody{
			Code:    "VALIDATION_ERROR",
			Message: "Data yang dimasukkan belum valid.",
			Fields:  []errorField{{Field: "body", Message: "Request body tidak valid."}},
		})
		return
	}

	result, err := h.register.Handle(c.Request.Context(), application.RegisterCommand{
		Name:  req.Name,
		Email: req.Email,
	})
	if err != nil {
		writeRegisterError(c, err)
		return
	}

	regisURL := buildRegisURL(c, result.RegisCode)
	h.sendVerificationEmailAsync(result.Name, result.Email, regisURL)

	c.JSON(http.StatusCreated, registerResponse{
		RegistrationID: result.RegistrationID,
		Name:           result.Name,
		Email:          result.Email,
		Status:         result.Status,
		RegisURL:       regisURL,
		Availability: availabilityResponse{
			Total:      result.Availability.Total,
			Registered: result.Availability.Registered,
			Remaining:  result.Availability.Remaining,
		},
	})
}

// sendVerificationEmailAsync fires the send in its own goroutine so a slow or
// failing mail server never delays the register response. Registration has
// already reserved the quota slot by this point; a failed send only means
// the person never gets their link and the reservation expires on its own
// TTL — it does not roll back the registration.
func (h *Handler) sendVerificationEmailAsync(name, email, regisURL string) {
	if h.mailer == nil {
		return
	}
	subject := "Konfirmasi Pendaftaran Workshop AI for Everyone"
	body := fmt.Sprintf(
		"Halo %s,\n\nTerima kasih telah mendaftar workshop \"AI for Everyone\".\n\n"+
			"Klik tautan berikut untuk mengonfirmasi pendaftaran Anda:\n%s\n\n"+
			"Tautan ini akan kedaluwarsa jika tidak segera diverifikasi, dan kuota Anda akan dikembalikan.\n\n"+
			"Terima kasih.",
		name, regisURL,
	)
	go func() {
		if err := h.mailer.Send(context.Background(), email, subject, body); err != nil {
			log.Printf("send verification email to %s: FAILED: %v", email, err)
			return
		}
		log.Printf("send verification email to %s: OK", email)
	}()
}

func buildRegisURL(c *gin.Context, code string) string {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/api/registrations/verify/%s", scheme, c.Request.Host, code)
}

func writeRegisterError(c *gin.Context, err error) {
	var valErr *application.ValidationError
	switch {
	case errors.As(err, &valErr):
		fields := make([]errorField, 0, len(valErr.Fields))
		for _, f := range valErr.Fields {
			fields = append(fields, errorField{Field: f.Field, Message: f.Message})
		}
		respondError(c, http.StatusUnprocessableEntity, errorBody{
			Code:    "VALIDATION_ERROR",
			Message: "Data yang dimasukkan belum valid.",
			Fields:  fields,
		})
	case errors.Is(err, application.ErrEmailAlreadyRegistered):
		respondError(c, http.StatusConflict, errorBody{
			Code:    "EMAIL_ALREADY_REGISTERED",
			Message: "Email ini sudah terdaftar untuk workshop.",
		})
	case errors.Is(err, application.ErrQuotaFull):
		respondError(c, http.StatusConflict, errorBody{
			Code:    "QUOTA_FULL",
			Message: "Mohon maaf, kuota workshop sudah penuh.",
		})
	default:
		respondError(c, http.StatusInternalServerError, errorBody{
			Code:    "INTERNAL_ERROR",
			Message: "Terjadi gangguan sementara. Silakan coba lagi nanti.",
		})
	}
}

func (h *Handler) handleAvailability(c *gin.Context) {
	result, err := h.availability.Handle(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusServiceUnavailable, errorBody{
			Code:    "INTERNAL_ERROR",
			Message: "Layanan sementara tidak tersedia.",
		})
		return
	}

	c.JSON(http.StatusOK, availabilityResponse{
		Total:      result.Total,
		Registered: result.Registered,
		Remaining:  result.Remaining,
	})
}

func (h *Handler) handleVerify(c *gin.Context) {
	code := c.Param("code")

	result, err := h.verify.Handle(c.Request.Context(), application.VerifyCommand{Code: code})
	if err != nil {
		writeVerifyError(c, err)
		return
	}

	c.JSON(http.StatusOK, verifyResponse{
		RegistrationID: result.RegistrationID,
		Status:         result.Status,
		Message:        "Pendaftaran berhasil diverifikasi.",
	})
}

func writeVerifyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, application.ErrInvalidCode):
		respondError(c, http.StatusBadRequest, errorBody{
			Code:    "VALIDATION_ERROR",
			Message: "Kode verifikasi wajib diisi.",
		})
	case errors.Is(err, domain.ErrNotFound):
		respondError(c, http.StatusNotFound, errorBody{
			Code:    "REGISTRATION_NOT_FOUND",
			Message: "Kode registrasi tidak ditemukan.",
		})
	case errors.Is(err, domain.ErrExpired):
		respondError(c, http.StatusGone, errorBody{
			Code:    "REGISTRATION_EXPIRED",
			Message: "Kode registrasi sudah kedaluwarsa. Kuota telah dikembalikan.",
		})
	default:
		respondError(c, http.StatusInternalServerError, errorBody{
			Code:    "INTERNAL_ERROR",
			Message: "Terjadi gangguan sementara. Silakan coba lagi nanti.",
		})
	}
}

func respondError(c *gin.Context, status int, body errorBody) {
	c.JSON(status, gin.H{"error": body})
}
