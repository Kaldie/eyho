package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/eyho/eyho/src/api/middleware"
	"github.com/eyho/eyho/src/services"
)

// MeHandler serves the account endpoints of contracts/accounts-api.md. Handlers only map
// requests and responses; the rules live in services.AccountService.
type MeHandler struct {
	svc *services.AccountService
}

// NewMeHandler creates the handler.
func NewMeHandler(svc *services.AccountService) *MeHandler { return &MeHandler{svc: svc} }

type meResponse struct {
	ID            string                    `json:"id"`
	Email         string                    `json:"email"`
	EmailVerified bool                      `json:"email_verified"`
	DisplayName   *string                   `json:"display_name"`
	IsDemo        bool                      `json:"is_demo"`
	CookProfile   *services.CookProfileView `json:"cook_profile"`
}

func (h *MeHandler) respondMe(w http.ResponseWriter, r *http.Request, a *services.Account, status int) {
	cp, err := h.svc.GetCookProfile(r.Context(), a.ID)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, status, meResponse{
		ID: a.ID.String(), Email: a.Email, EmailVerified: a.EmailVerified,
		DisplayName: a.DisplayName, IsDemo: a.IsDemo, CookProfile: cp,
	})
}

// GetMe implements GET /me.
func (h *MeHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	a, _ := middleware.AccountFromContext(r.Context())
	h.respondMe(w, r, a, http.StatusOK)
}

// PatchMe implements PATCH /me (display_name).
func (h *MeHandler) PatchMe(w http.ResponseWriter, r *http.Request) {
	a, _ := middleware.AccountFromContext(r.Context())
	var body struct {
		DisplayName *string `json:"display_name"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.DisplayName == nil {
		writeError(w, http.StatusUnprocessableEntity, "missing_field", "display_name")
		return
	}
	name, err := h.svc.UpdateDisplayName(r.Context(), a.ID, *body.DisplayName)
	if err != nil {
		h.fail(w, err)
		return
	}
	a.DisplayName = name
	h.respondMe(w, r, a, http.StatusOK)
}

type cookProfileBody struct {
	DisplayName *string            `json:"display_name"`
	OrgType     *string            `json:"org_type"`
	Phone       *string            `json:"phone"`
	AddressText *string            `json:"default_pickup_address_text"`
	Location    *services.Location `json:"default_pickup_location"`
	KvkNumber   *string            `json:"kvk_number"`
	NVWA        *struct {
		Accepted bool   `json:"accepted"`
		Version  string `json:"version"`
	} `json:"nvwa_declaration"`
}

func (b cookProfileBody) declaration() *services.NVWADeclaration {
	if b.NVWA == nil {
		return nil
	}
	return &services.NVWADeclaration{Accepted: b.NVWA.Accepted, Version: b.NVWA.Version}
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// PostCookProfile implements POST /me/cook-profile (201, 202 or 422).
func (h *MeHandler) PostCookProfile(w http.ResponseWriter, r *http.Request) {
	a, _ := middleware.AccountFromContext(r.Context())
	var b cookProfileBody
	if !decode(w, r, &b) {
		return
	}
	view, err := h.svc.CreateCookProfile(r.Context(), a.ID, services.CreateCookProfileInput{
		DisplayName: str(b.DisplayName), OrgType: str(b.OrgType), Phone: str(b.Phone),
		AddressText: str(b.AddressText), Location: b.Location, KvkNumber: str(b.KvkNumber),
		NVWA: b.declaration(),
	})
	if err != nil {
		h.fail(w, err)
		return
	}
	status := http.StatusCreated
	if view.RegistrationStatus == "pending" {
		status = http.StatusAccepted // register unreachable: retried in the background
	}
	writeJSON(w, status, view)
}

// PatchCookProfile implements PATCH /me/cook-profile (200, 202 or 422).
func (h *MeHandler) PatchCookProfile(w http.ResponseWriter, r *http.Request) {
	a, _ := middleware.AccountFromContext(r.Context())
	var b cookProfileBody
	if !decode(w, r, &b) {
		return
	}
	view, err := h.svc.UpdateCookProfile(r.Context(), a.ID, services.UpdateCookProfileInput{
		DisplayName: b.DisplayName, OrgType: b.OrgType, Phone: b.Phone, AddressText: b.AddressText,
		Location: b.Location, KvkNumber: b.KvkNumber, NVWA: b.declaration(),
	})
	if err != nil {
		h.fail(w, err)
		return
	}
	status := http.StatusOK
	if b.KvkNumber != nil && view.RegistrationStatus == "pending" {
		status = http.StatusAccepted
	}
	writeJSON(w, status, view)
}

// PostVerificationEmail implements POST /me/verification-email.
func (h *MeHandler) PostVerificationEmail(w http.ResponseWriter, r *http.Request) {
	a, _ := middleware.AccountFromContext(r.Context())
	if err := h.svc.RequestVerificationEmail(r.Context(), a); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// DeleteMe implements DELETE /me.
func (h *MeHandler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	a, _ := middleware.AccountFromContext(r.Context())
	if err := h.svc.DeleteAccount(r.Context(), a); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// fail maps service errors to the contract's status codes and bodies.
func (h *MeHandler) fail(w http.ResponseWriter, err error) {
	var ve *services.ValidationError
	var be *services.BlockedError
	switch {
	case errors.As(err, &ve):
		writeError(w, http.StatusUnprocessableEntity, ve.Code, ve.Field)
	case errors.As(err, &be):
		writeJSON(w, http.StatusConflict, map[string]any{"error": be.Code, "count": be.Count})
	case errors.Is(err, services.ErrDeclarationNeeded):
		writeError(w, http.StatusUnprocessableEntity, "declaration_required", "")
	case errors.Is(err, services.ErrKvkNotFound):
		writeError(w, http.StatusUnprocessableEntity, "kvk_not_found", "")
	case errors.Is(err, services.ErrKvkInactive):
		writeError(w, http.StatusUnprocessableEntity, "kvk_inactive", "")
	case errors.Is(err, services.ErrCookProfileExists):
		writeError(w, http.StatusConflict, "cook_profile_exists", "")
	case errors.Is(err, services.ErrCookProfileMissing):
		writeError(w, http.StatusForbidden, "cook_profile_required", "")
	case errors.Is(err, services.ErrAlreadyVerified):
		writeError(w, http.StatusConflict, "already_verified", "")
	case errors.Is(err, services.ErrRateLimited):
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate_limited", "")
	case errors.Is(err, services.ErrIdpUnavailable):
		writeError(w, http.StatusBadGateway, "identity_provider_unavailable", "")
	case errors.Is(err, services.ErrAccountDeleted):
		writeError(w, http.StatusForbidden, "account_deleted", "")
	case errors.Is(err, services.ErrDemoAccount):
		writeError(w, http.StatusForbidden, "demo_account", "")
	default:
		log.Printf("me handler: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "")
	}
}

// decode reads a JSON body (max 64 KiB); a malformed body answers 422 invalid_field.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		field := ""
		var ute *json.UnmarshalTypeError
		if errors.As(err, &ute) {
			field = ute.Field
		}
		writeError(w, http.StatusUnprocessableEntity, "invalid_field", field)
		return false
	}
	return true
}
