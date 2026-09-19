package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/pkg/config"
)

type AuthService struct {
	repo *UserRepository
	rdb  *redis.Client
	cfg  *config.Config
}

func NewAuthService(repo *UserRepository, rdb *redis.Client, cfg *config.Config) *AuthService {
	return &AuthService{
		repo: repo,
		rdb:  rdb,
		cfg:  cfg,
	}
}

func (s *AuthService) Register(ctx context.Context, req RegisterRequest) (*TokenResponse, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user := &User{
		ID:        uuid.New().String(),
		Email:     req.Email,
		Username:  req.Username,
		Password:  string(hashedPassword),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return s.generateTokenPair(ctx, user.ID, user.Email)
}

func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*TokenResponse, error) {
	user, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	_ = s.repo.UpdateLastLogin(ctx, user.ID)

	return s.generateTokenPair(ctx, user.ID, user.Email)
}

func (s *AuthService) RefreshToken(ctx context.Context, req RefreshRequest) (*TokenResponse, error) {
	token, err := jwt.Parse(req.RefreshToken, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.cfg.JWT.Secret), nil
	})

	if err != nil || !token.Valid {
		return nil, errors.New("invalid refresh token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	userID := claims["user_id"].(string)

	// Validate in Redis
	redisKey := fmt.Sprintf("refresh_token:%s", userID)
	storedToken, err := s.rdb.Get(ctx, redisKey).Result()
	if err != nil || storedToken != req.RefreshToken {
		return nil, errors.New("refresh token expired or revoked")
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	return s.generateTokenPair(ctx, user.ID, user.Email)
}

func (s *AuthService) GetCurrentUser(ctx context.Context, userID string) (*UserResponse, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Username:  user.Username,
		AvatarURL: user.AvatarURL,
		CreatedAt: user.CreatedAt,
	}, nil
}

func (s *AuthService) generateTokenPair(ctx context.Context, userID, email string) (*TokenResponse, error) {
	// Access Token
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     time.Now().Add(s.cfg.JWT.AccessExpiry).Unix(),
	})
	accessTokenString, err := accessToken.SignedString([]byte(s.cfg.JWT.Secret))
	if err != nil {
		return nil, err
	}

	// Refresh Token
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(s.cfg.JWT.RefreshExpiry).Unix(),
	})
	refreshTokenString, err := refreshToken.SignedString([]byte(s.cfg.JWT.Secret))
	if err != nil {
		return nil, err
	}

	// Store Refresh Token in Redis
	redisKey := fmt.Sprintf("refresh_token:%s", userID)
	err = s.rdb.Set(ctx, redisKey, refreshTokenString, s.cfg.JWT.RefreshExpiry).Err()
	if err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresIn:    int64(s.cfg.JWT.AccessExpiry.Seconds()),
	}, nil
}
