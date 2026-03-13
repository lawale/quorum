package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/testutil"
	"golang.org/x/crypto/bcrypt"
)

func newMockOperatorStore() *testutil.MockOperatorStore {
	return &testutil.MockOperatorStore{}
}

func TestOperatorService_Setup_Success(t *testing.T) {
	store := newMockOperatorStore()
	store.CountFunc = func(ctx context.Context) (int, error) { return 0, nil }
	store.CreateFunc = func(ctx context.Context, op *model.Operator) error {
		op.ID = uuid.New()
		return nil
	}

	svc := NewOperatorService(store, "test-secret")

	op, token, err := svc.Setup(context.Background(), "admin", "password123", "Admin User")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op == nil {
		t.Fatal("expected non-nil operator")
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if op.Username != "admin" {
		t.Errorf("Username = %q, want %q", op.Username, "admin")
	}
	if op.DisplayName != "Admin User" {
		t.Errorf("DisplayName = %q, want %q", op.DisplayName, "Admin User")
	}
	if op.MustChangePassword {
		t.Error("expected MustChangePassword=false for setup operator")
	}

	// Verify password was hashed
	if err := bcrypt.CompareHashAndPassword([]byte(op.PasswordHash), []byte("password123")); err != nil {
		t.Error("password hash doesn't match original password")
	}
}

func TestOperatorService_Setup_AlreadyDone(t *testing.T) {
	store := newMockOperatorStore()
	store.CountFunc = func(ctx context.Context) (int, error) { return 1, nil }

	svc := NewOperatorService(store, "test-secret")

	_, _, err := svc.Setup(context.Background(), "admin", "password123", "Admin")
	if !errors.Is(err, ErrSetupAlreadyDone) {
		t.Fatalf("expected ErrSetupAlreadyDone, got: %v", err)
	}
}

func TestOperatorService_Login_Success(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	store := newMockOperatorStore()
	store.GetByUsernameFunc = func(ctx context.Context, username string) (*model.Operator, error) {
		return &model.Operator{
			ID:           uuid.New(),
			Username:     "admin",
			PasswordHash: string(hash),
			DisplayName:  "Admin",
		}, nil
	}

	svc := NewOperatorService(store, "test-secret")

	op, token, err := svc.Login(context.Background(), "admin", "password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op == nil {
		t.Fatal("expected non-nil operator")
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// Validate the issued token
	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("token validation failed: %v", err)
	}
	if claims.Username != "admin" {
		t.Errorf("claims.Username = %q, want %q", claims.Username, "admin")
	}
	if claims.Subject != op.ID.String() {
		t.Errorf("claims.Subject = %q, want %q", claims.Subject, op.ID.String())
	}
}

func TestOperatorService_Login_UserNotFound(t *testing.T) {
	store := newMockOperatorStore()
	store.GetByUsernameFunc = func(ctx context.Context, username string) (*model.Operator, error) {
		return nil, nil
	}

	svc := NewOperatorService(store, "test-secret")

	_, _, err := svc.Login(context.Background(), "nobody", "password123")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestOperatorService_Login_WrongPassword(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)

	store := newMockOperatorStore()
	store.GetByUsernameFunc = func(ctx context.Context, username string) (*model.Operator, error) {
		return &model.Operator{
			ID:           uuid.New(),
			Username:     "admin",
			PasswordHash: string(hash),
		}, nil
	}

	svc := NewOperatorService(store, "test-secret")

	_, _, err := svc.Login(context.Background(), "admin", "wrong-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestOperatorService_CreateOperator_Success(t *testing.T) {
	store := newMockOperatorStore()
	store.GetByUsernameFunc = func(ctx context.Context, username string) (*model.Operator, error) {
		return nil, nil // no conflict
	}
	store.CreateFunc = func(ctx context.Context, op *model.Operator) error {
		op.ID = uuid.New()
		return nil
	}

	svc := NewOperatorService(store, "test-secret")

	op, err := svc.CreateOperator(context.Background(), "newuser", "pass123", "New User")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op.Username != "newuser" {
		t.Errorf("Username = %q, want %q", op.Username, "newuser")
	}
	if !op.MustChangePassword {
		t.Error("expected MustChangePassword=true for created operator")
	}
}

func TestOperatorService_CreateOperator_UsernameExists(t *testing.T) {
	store := newMockOperatorStore()
	store.GetByUsernameFunc = func(ctx context.Context, username string) (*model.Operator, error) {
		return &model.Operator{Username: "existing"}, nil
	}

	svc := NewOperatorService(store, "test-secret")

	_, err := svc.CreateOperator(context.Background(), "existing", "pass123", "Existing")
	if !errors.Is(err, ErrUsernameExists) {
		t.Fatalf("expected ErrUsernameExists, got: %v", err)
	}
}

func TestOperatorService_ChangePassword_Success(t *testing.T) {
	currentHash, _ := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.DefaultCost)
	opID := uuid.New()

	store := newMockOperatorStore()
	store.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*model.Operator, error) {
		return &model.Operator{
			ID:                 opID,
			Username:           "admin",
			PasswordHash:       string(currentHash),
			MustChangePassword: true,
		}, nil
	}
	store.UpdateFunc = func(ctx context.Context, op *model.Operator) error {
		// Verify the new hash is set
		if err := bcrypt.CompareHashAndPassword([]byte(op.PasswordHash), []byte("new-password")); err != nil {
			t.Error("new password hash doesn't match")
		}
		if op.MustChangePassword {
			t.Error("expected MustChangePassword=false after change")
		}
		return nil
	}

	svc := NewOperatorService(store, "test-secret")

	err := svc.ChangePassword(context.Background(), opID, "old-password", "new-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOperatorService_ChangePassword_WrongCurrent(t *testing.T) {
	currentHash, _ := bcrypt.GenerateFromPassword([]byte("actual-password"), bcrypt.DefaultCost)
	opID := uuid.New()

	store := newMockOperatorStore()
	store.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*model.Operator, error) {
		return &model.Operator{
			ID:           opID,
			PasswordHash: string(currentHash),
		}, nil
	}

	svc := NewOperatorService(store, "test-secret")

	err := svc.ChangePassword(context.Background(), opID, "wrong-current", "new-password")
	if !errors.Is(err, ErrIncorrectPassword) {
		t.Fatalf("expected ErrIncorrectPassword, got: %v", err)
	}
}

func TestOperatorService_DeleteOperator_Success(t *testing.T) {
	callerID := uuid.New()
	targetID := uuid.New()

	store := newMockOperatorStore()
	store.CountFunc = func(ctx context.Context) (int, error) { return 2, nil }
	store.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*model.Operator, error) {
		return &model.Operator{ID: targetID, Username: "target"}, nil
	}
	store.DeleteFunc = func(ctx context.Context, id uuid.UUID) error {
		if id != targetID {
			t.Errorf("delete called with wrong ID: %v", id)
		}
		return nil
	}

	svc := NewOperatorService(store, "test-secret")

	err := svc.DeleteOperator(context.Background(), callerID, targetID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOperatorService_DeleteOperator_CannotDeleteSelf(t *testing.T) {
	selfID := uuid.New()

	store := newMockOperatorStore()
	svc := NewOperatorService(store, "test-secret")

	err := svc.DeleteOperator(context.Background(), selfID, selfID)
	if !errors.Is(err, ErrCannotDeleteSelf) {
		t.Fatalf("expected ErrCannotDeleteSelf, got: %v", err)
	}
}

func TestOperatorService_DeleteOperator_LastOperator(t *testing.T) {
	callerID := uuid.New()
	targetID := uuid.New()

	store := newMockOperatorStore()
	store.CountFunc = func(ctx context.Context) (int, error) { return 1, nil }

	svc := NewOperatorService(store, "test-secret")

	err := svc.DeleteOperator(context.Background(), callerID, targetID)
	if !errors.Is(err, ErrLastOperator) {
		t.Fatalf("expected ErrLastOperator, got: %v", err)
	}
}

func TestOperatorService_NeedsSetup(t *testing.T) {
	store := newMockOperatorStore()

	t.Run("true when no operators", func(t *testing.T) {
		store.CountFunc = func(ctx context.Context) (int, error) { return 0, nil }
		svc := NewOperatorService(store, "test-secret")

		needs, err := svc.NeedsSetup(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !needs {
			t.Error("expected NeedsSetup=true")
		}
	})

	t.Run("false when operators exist", func(t *testing.T) {
		store.CountFunc = func(ctx context.Context) (int, error) { return 1, nil }
		svc := NewOperatorService(store, "test-secret")

		needs, err := svc.NeedsSetup(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if needs {
			t.Error("expected NeedsSetup=false")
		}
	})
}

func TestOperatorService_ValidateToken_Invalid(t *testing.T) {
	svc := NewOperatorService(newMockOperatorStore(), "test-secret")

	_, err := svc.ValidateToken("invalid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestOperatorService_ValidateToken_WrongSecret(t *testing.T) {
	store := newMockOperatorStore()
	store.CountFunc = func(ctx context.Context) (int, error) { return 0, nil }
	store.CreateFunc = func(ctx context.Context, op *model.Operator) error {
		op.ID = uuid.New()
		return nil
	}

	svc1 := NewOperatorService(store, "secret-1")
	svc2 := NewOperatorService(store, "secret-2")

	// Issue token with svc1
	_, token, err := svc1.Setup(context.Background(), "admin", "pass", "Admin")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Validate with different secret should fail
	_, err = svc2.ValidateToken(token)
	if err == nil {
		t.Fatal("expected error validating token with wrong secret")
	}
}

func TestOperatorService_AutoGeneratedSecret(t *testing.T) {
	store := newMockOperatorStore()
	store.CountFunc = func(ctx context.Context) (int, error) { return 0, nil }
	store.CreateFunc = func(ctx context.Context, op *model.Operator) error {
		op.ID = uuid.New()
		return nil
	}

	// Empty secret should auto-generate
	svc := NewOperatorService(store, "")

	op, token, err := svc.Setup(context.Background(), "admin", "pass", "Admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op == nil || token == "" {
		t.Fatal("expected valid operator and token with auto-generated secret")
	}

	// Token should validate with same service instance
	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("token validation failed: %v", err)
	}
	if claims.Username != "admin" {
		t.Errorf("claims.Username = %q, want %q", claims.Username, "admin")
	}
}
