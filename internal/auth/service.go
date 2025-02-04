// backend/internal/auth/service.go
package auth

import (
    "errors"
    "quiz-system/internal/models"
    "time"

    "github.com/dgrijalva/jwt-go"
    "golang.org/x/crypto/bcrypt"
)

type Service struct {
    repo      *Repository
    jwtSecret []byte
}

func NewService(repo *Repository, jwtSecret string) *Service {
    return &Service{
        repo:      repo,
        jwtSecret: []byte(jwtSecret),
    }
}

func (s *Service) Login(username, password string) (string, error) {
    user, err := s.repo.GetUserByUsername(username)
    if err != nil {
        return "", errors.New("user not found")
    }

    if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
        return "", errors.New("invalid password")
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "user_id":  user.ID,
        "username": user.Username,
        "exp":      time.Now().Add(time.Hour * 24).Unix(),
    })

    tokenString, err := token.SignedString(s.jwtSecret)
    if err != nil {
        return "", err
    }

    return tokenString, nil
}

func (s *Service) Register(user *models.User) error {
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
    if err != nil {
        return err
    }

    user.Password = string(hashedPassword)
    return s.repo.CreateUser(user)
}