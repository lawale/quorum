package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/service"
)

// ConsoleHandler handles admin console API endpoints for operator management.
type ConsoleHandler struct {
	operatorService *service.OperatorService
}

func NewConsoleHandler(os *service.OperatorService) *ConsoleHandler {
	return &ConsoleHandler{operatorService: os}
}

type setupBody struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type loginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type changePasswordBody struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type createOperatorBody struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

// Setup creates the first operator. Only works when no operators exist.
func (h *ConsoleHandler) Setup(w http.ResponseWriter, r *http.Request) {
	var body setupBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	op, token, err := h.operatorService.Setup(r.Context(), body.Username, body.Password, body.DisplayName)
	if err != nil {
		if errors.Is(err, service.ErrSetupAlreadyDone) {
			writeError(w, http.StatusConflict, "setup has already been completed")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to complete setup")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"operator": op,
		"token":    token,
	})
}

// Login authenticates an operator and returns a JWT.
func (h *ConsoleHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	op, token, err := h.operatorService.Login(r.Context(), body.Username, body.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid username or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to login")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"operator": op,
		"token":    token,
	})
}

// NeedsSetup returns whether the system needs initial setup.
func (h *ConsoleHandler) NeedsSetup(w http.ResponseWriter, r *http.Request) {
	needs, err := h.operatorService.NeedsSetup(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check setup status")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"needs_setup": needs,
	})
}

// Me returns the current authenticated operator.
func (h *ConsoleHandler) Me(w http.ResponseWriter, r *http.Request) {
	opID := operatorIDFromContext(r.Context())
	if opID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	op, err := h.operatorService.GetCurrentOperator(r.Context(), opID)
	if err != nil {
		if errors.Is(err, service.ErrOperatorNotFound) {
			writeError(w, http.StatusNotFound, "operator not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get operator")
		return
	}

	writeJSON(w, http.StatusOK, op)
}

// ChangePassword changes the current operator's password.
func (h *ConsoleHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	opID := operatorIDFromContext(r.Context())
	if opID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var body changePasswordBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.CurrentPassword == "" || body.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "current_password and new_password are required")
		return
	}

	err := h.operatorService.ChangePassword(r.Context(), opID, body.CurrentPassword, body.NewPassword)
	if err != nil {
		if errors.Is(err, service.ErrIncorrectPassword) {
			writeError(w, http.StatusBadRequest, "current password is incorrect")
			return
		}
		if errors.Is(err, service.ErrOperatorNotFound) {
			writeError(w, http.StatusNotFound, "operator not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to change password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "password changed successfully"})
}

// ListOperators returns all operators.
func (h *ConsoleHandler) ListOperators(w http.ResponseWriter, r *http.Request) {
	operators, err := h.operatorService.ListOperators(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list operators")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": operators})
}

// CreateOperator creates a new operator.
func (h *ConsoleHandler) CreateOperator(w http.ResponseWriter, r *http.Request) {
	var body createOperatorBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	op, err := h.operatorService.CreateOperator(r.Context(), body.Username, body.Password, body.DisplayName)
	if err != nil {
		if errors.Is(err, service.ErrUsernameExists) {
			writeError(w, http.StatusConflict, "username already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create operator")
		return
	}

	writeJSON(w, http.StatusCreated, op)
}

// DeleteOperator deletes an operator by ID.
func (h *ConsoleHandler) DeleteOperator(w http.ResponseWriter, r *http.Request) {
	callerID := operatorIDFromContext(r.Context())
	if callerID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid operator ID")
		return
	}

	if err := h.operatorService.DeleteOperator(r.Context(), callerID, targetID); err != nil {
		if errors.Is(err, service.ErrCannotDeleteSelf) {
			writeError(w, http.StatusBadRequest, "cannot delete yourself")
			return
		}
		if errors.Is(err, service.ErrLastOperator) {
			writeError(w, http.StatusBadRequest, "cannot delete the last operator")
			return
		}
		if errors.Is(err, service.ErrOperatorNotFound) {
			writeError(w, http.StatusNotFound, "operator not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete operator")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
