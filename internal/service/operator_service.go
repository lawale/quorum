package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrOperatorNotFound   = errors.New("operator not found")
	ErrUsernameExists     = errors.New("username already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrSetupAlreadyDone   = errors.New("setup has already been completed")
	ErrNoOperatorsExist   = errors.New("no operators exist; run setup first")
	ErrIncorrectPassword  = errors.New("current password is incorrect")
	ErrCannotDeleteSelf   = errors.New("cannot delete yourself")
	ErrLastOperator       = errors.New("cannot delete the last operator")
)

// OperatorTokenClaims are the JWT claims for console operator sessions.
type OperatorTokenClaims struct {
	jwt.RegisteredClaims
	Username string `json:"username"`
}

// OperatorService handles operator management and authentication for the admin console.
type OperatorService struct {
	operators store.OperatorStore
	jwtSecret []byte
	jwtExpiry time.Duration
}

// NewOperatorService creates a new OperatorService.
// If jwtSecret is empty, a random 32-byte secret is generated (sessions won't survive restarts).
func NewOperatorService(operators store.OperatorStore, jwtSecret string) *OperatorService {
	secret := []byte(jwtSecret)
	if len(secret) == 0 {
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			panic("failed to generate random JWT secret: " + err.Error())
		}
		slog.Warn("no JWT secret configured, using random secret (sessions will not survive restarts)")
	} else if len(secret) < 32 {
		panic("JWT secret must be at least 32 bytes long")
	}
	return &OperatorService{
		operators: operators,
		jwtSecret: secret,
		jwtExpiry: 24 * time.Hour,
	}
}

// Setup creates the first operator. Returns ErrSetupAlreadyDone if operators already exist.
func (s *OperatorService) Setup(ctx context.Context, username, password, displayName string) (*model.Operator, string, error) {
	count, err := s.operators.Count(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("checking operator count: %w", err)
	}
	if count > 0 {
		return nil, "", ErrSetupAlreadyDone
	}

	return s.createOperator(ctx, username, password, displayName, false)
}

// Login authenticates an operator and returns a JWT token.
func (s *OperatorService) Login(ctx context.Context, username, password string) (*model.Operator, string, error) {
	op, err := s.operators.GetByUsername(ctx, username)
	if err != nil {
		return nil, "", fmt.Errorf("looking up operator: %w", err)
	}
	if op == nil {
		return nil, "", ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(op.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	token, err := s.issueToken(op)
	if err != nil {
		return nil, "", err
	}

	return op, token, nil
}

// CreateOperator creates a new operator (called by an existing operator).
func (s *OperatorService) CreateOperator(ctx context.Context, username, password, displayName string) (*model.Operator, error) {
	existing, err := s.operators.GetByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("checking existing username: %w", err)
	}
	if existing != nil {
		return nil, ErrUsernameExists
	}

	op, _, err := s.createOperator(ctx, username, password, displayName, true)
	if err != nil {
		return nil, err
	}
	return op, nil
}

// GetCurrentOperator returns the operator for the given ID.
func (s *OperatorService) GetCurrentOperator(ctx context.Context, id uuid.UUID) (*model.Operator, error) {
	op, err := s.operators.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting operator: %w", err)
	}
	if op == nil {
		return nil, ErrOperatorNotFound
	}
	return op, nil
}

// ChangePassword changes an operator's password. Requires the current password for verification.
func (s *OperatorService) ChangePassword(ctx context.Context, operatorID uuid.UUID, currentPassword, newPassword string) error {
	op, err := s.operators.GetByID(ctx, operatorID)
	if err != nil {
		return fmt.Errorf("getting operator: %w", err)
	}
	if op == nil {
		return ErrOperatorNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(op.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrIncorrectPassword
	}

	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	op.PasswordHash = hash
	op.MustChangePassword = false
	if err := s.operators.Update(ctx, op); err != nil {
		return fmt.Errorf("updating operator password: %w", err)
	}

	return nil
}

// ListOperators returns all operators.
func (s *OperatorService) ListOperators(ctx context.Context) ([]model.Operator, error) {
	return s.operators.List(ctx)
}

// DeleteOperator deletes an operator. Cannot delete yourself or the last operator.
func (s *OperatorService) DeleteOperator(ctx context.Context, callerID, targetID uuid.UUID) error {
	if callerID == targetID {
		return ErrCannotDeleteSelf
	}

	count, err := s.operators.Count(ctx)
	if err != nil {
		return fmt.Errorf("counting operators: %w", err)
	}
	if count <= 1 {
		return ErrLastOperator
	}

	target, err := s.operators.GetByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("getting operator: %w", err)
	}
	if target == nil {
		return ErrOperatorNotFound
	}

	return s.operators.Delete(ctx, targetID)
}

// NeedsSetup returns true if no operators exist yet.
func (s *OperatorService) NeedsSetup(ctx context.Context) (bool, error) {
	count, err := s.operators.Count(ctx)
	if err != nil {
		return false, fmt.Errorf("counting operators: %w", err)
	}
	return count == 0, nil
}

// ValidateToken parses and validates a JWT token, returning the claims.
func (s *OperatorService) ValidateToken(tokenString string) (*OperatorTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &OperatorTokenClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}

	claims, ok := token.Claims.(*OperatorTokenClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

func (s *OperatorService) createOperator(ctx context.Context, username, password, displayName string, mustChange bool) (*model.Operator, string, error) {
	hash, err := hashPassword(password)
	if err != nil {
		return nil, "", err
	}

	op := &model.Operator{
		Username:           username,
		PasswordHash:       hash,
		DisplayName:        displayName,
		MustChangePassword: mustChange,
	}

	if err := s.operators.Create(ctx, op); err != nil {
		return nil, "", fmt.Errorf("creating operator: %w", err)
	}

	token, err := s.issueToken(op)
	if err != nil {
		return nil, "", err
	}

	return op, token, nil
}

func (s *OperatorService) issueToken(op *model.Operator) (string, error) {
	now := time.Now()
	claims := OperatorTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   op.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.jwtExpiry)),
		},
		Username: op.Username,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}

	return signed, nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return string(hash), nil
}
