// backend/internal/auth/repository.go
package auth

import (
	"log"
	"quiz-system/internal/models"

	"gorm.io/gorm"
)

type Repository struct {
    db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
    return &Repository{db: db}
}
// backend/internal/auth/repository.go
func (r *Repository) GetUserByUsername(username string) (*models.User, error) {
    var user models.User
    // Let's print the query we're making
    log.Printf("Attempting to find user with username: %s", username)
    
    result := r.db.Where("username = ?", username).First(&user)
    if result.Error != nil {
        log.Printf("Error finding user: %v", result.Error)
        return nil, result.Error
    }
    return &user, nil
}

func (r *Repository) CreateUser(user *models.User) error {
    return r.db.Create(user).Error
}

