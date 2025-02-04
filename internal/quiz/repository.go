// backend/internal/quiz/repository.go
package quiz

import (
	"errors"
	"fmt"
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

func (r *Repository) CreateQuiz(quiz *models.Quiz) error {
    err := r.db.Create(quiz).Error
    if err != nil {
        log.Printf("Error creating quiz: %v", err)
        return err
    }
    log.Printf("Created quiz with ID: %d", quiz.ID)
    return nil
}


// repository.go
func (r *Repository) GetUserByID(userID uint) (*models.User, error) {
    var user models.User
    err := r.db.First(&user, userID).Error
    if err != nil {
        return nil, err
    }
    return &user, nil
}

func (r *Repository) UpdateQuiz(quiz *models.Quiz) error {
    err := r.db.Save(quiz).Error
    if err != nil {
        log.Printf("Error updating quiz: %v", err)
        return err
    }
    log.Printf("Updated quiz with ID: %d", quiz.ID)
    return nil
}

func (r *Repository) GetUserQuestionIndex(userID, quizID uint) (int, error) {
	var progress models.UserQuizProgress
	err := r.db.Where("user_id = ? AND quiz_id = ?", userID, quizID).First(&progress).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Record not found; create a new one with NextIndex 0
			progress = models.UserQuizProgress{
				UserID:    userID,
				QuizID:    quizID,
				NextIndex: 0,
			}
			if createErr := r.db.Create(&progress).Error; createErr != nil {
				return 0, createErr
			}
			return 0, nil
		}
		return 0, err
	}
	return progress.NextIndex, nil
}

// UpdateUserQuestionIndex updates the next question index for a given user and quiz.
func (r *Repository) UpdateUserQuestionIndex(userID, quizID uint, newIndex int) error {
	var progress models.UserQuizProgress
	err := r.db.Where("user_id = ? AND quiz_id = ?", userID, quizID).First(&progress).Error
	if err != nil {
		return err
	}
	progress.NextIndex = newIndex
	return r.db.Save(&progress).Error
}

// In repository.go
func (r *Repository) GetQuizQuestions(quizID uint) ([]models.Question, error) {
    var questions []models.Question
    
    err := r.db.Where("quiz_id = ? AND deleted_at IS NULL", quizID).
        Preload("Options", "deleted_at IS NULL").
        Find(&questions).Error
    
    if err != nil {
        log.Printf("Error getting questions: %v", err)
        return nil, err
    }

    log.Printf("Found %d questions for quiz %d", len(questions), quizID)
    
    return questions, nil
}

func (r *Repository) VerifyQuizData(quizID uint) error {
    // Check questions
    var questionCount int64
    if err := r.db.Model(&models.Question{}).
        Where("quiz_id = ?", quizID).
        Count(&questionCount).Error; err != nil {
        return err
    }
    log.Printf("Found %d questions for quiz %d", questionCount, quizID)

    // Check options for each question
    var questions []models.Question
    err := r.db.Where("quiz_id = ?", quizID).Find(&questions).Error
    if err != nil {
        return err
    }

    for _, q := range questions {
        var optionCount int64
        if err := r.db.Model(&models.Option{}).
            Where("question_id = ?", q.ID).
            Count(&optionCount).Error; err != nil {
            return err
        }
        log.Printf("Question %d has %d options", q.ID, optionCount)
    }

    return nil
}

func (r *Repository) GetQuizzesByCreator(userID uint) ([]models.Quiz, error) {
    var quizzes []models.Quiz
    err := r.db.Where("creator_id = ?", userID).Find(&quizzes).Error
    if err != nil {
        log.Printf("Error getting quizzes for creator %d: %v", userID, err)
        return nil, err
    }
    return quizzes, nil
}

func (r *Repository) GetQuizByCode(code string) (*models.Quiz, error) {
    var quiz models.Quiz
    err := r.db.Preload("Questions.Options").
        Where("quiz_code = ?", code).
        First(&quiz).Error

    if err != nil {
        log.Printf("Error getting quiz by code %s: %v", code, err)
        return nil, err
    }
    log.Printf("Found quiz %d with code %s", quiz.ID, code)
    return &quiz, nil
}

func (r *Repository) GetQuestion(questionID uint) (*models.Question, error) {
    var question models.Question
    err := r.db.Preload("Options").First(&question, questionID).Error
    if err != nil {
        log.Printf("Error getting question %d: %v", questionID, err)
        return nil, err
    }
    return &question, nil
}

func (r *Repository) SaveResponse(response *models.UserQuizResponse) error {
    return r.db.Create(response).Error
}

func (r *Repository) AddParticipant(quizID, userID uint) error {
    participant := &models.QuizParticipant{
        QuizID: quizID,
        UserID: userID,
    }
    err := r.db.Create(participant).Error
    if err != nil {
        log.Printf("Error adding participant %d to quiz %d: %v", userID, quizID, err)
        return err
    }
    log.Printf("Added participant %d to quiz %d", userID, quizID)
    return nil
}


func (r *Repository) RemoveParticipant(quizID, userID uint) error {
    result := r.db.Where("quiz_id = ? AND user_id = ?", quizID, userID).
        Delete(&models.QuizParticipant{})
    
    if result.Error != nil {
        return result.Error
    }
    
    return nil
}

func (r *Repository) ClearUserProgress(quizID, userID uint) error {
    result := r.db.Where("quiz_id = ? AND user_id = ?", quizID, userID).
        Delete(&models.UserQuizResponse{})
    
    if result.Error != nil {
        return result.Error
    }
    
    return nil
}

// repository.go
func (r *Repository) GetLeaderboard(quizID uint) ([]models.LeaderboardEntry, error) {
    var entries []models.LeaderboardEntry
    
    err := r.db.Raw(`
        SELECT u.username, SUM(uqr.score) as total_score
        FROM users u
        JOIN user_quiz_responses uqr ON u.id = uqr.user_id
        WHERE uqr.quiz_id = ? AND uqr.deleted_at IS NULL
        GROUP BY u.username
        ORDER BY total_score DESC
    `, quizID).Scan(&entries).Error

    if err != nil {
        log.Printf("Error getting leaderboard: %v", err)
        return nil, err
    }

    return entries, nil
}

// repository.go
func (r *Repository) GetQuestionIndex(quizID uint, questionID uint) (int, error) {
    var questions []models.Question
    err := r.db.Where("quiz_id = ? AND deleted_at IS NULL", quizID).
        Order("created_at asc").
        Find(&questions).Error
    if err != nil {
        return 0, err
    }

    for i, q := range questions {
        if q.ID == questionID {
            return i, nil
        }
    }
    return 0, fmt.Errorf("question not found")
}

func (r *Repository) GetQuizByID(quizID uint) (*models.Quiz, error) {
    var quiz models.Quiz
    err := r.db.First(&quiz, quizID).Error
    if err != nil {
        log.Printf("Error getting quiz %d: %v", quizID, err)
        return nil, err
    }
    return &quiz, nil
}



func (r *Repository) GetUniqueResponseCountForQuestion(questionID uint) (int64, error) {
    var count int64
    err := r.db.Model(&models.UserQuizResponse{}).
        Where("question_id = ? AND deleted_at IS NULL", questionID).
        Distinct("user_id").
        Count(&count).Error
    return count, err
}

func (r *Repository) GetUniqueParticipantsForQuiz(quizID uint) (int64, error) {
    var count int64
    err := r.db.Model(&models.UserQuizResponse{}).
        Where("quiz_id = ? AND deleted_at IS NULL", quizID).
        Distinct("user_id").
        Count(&count).Error
    return count, err
}
func (r *Repository) IsUserHost(quizID, userID uint) (bool, error) {
    var quiz models.Quiz
    err := r.db.Select("creator_id").Where("id = ?", quizID).First(&quiz).Error
    if err != nil {
        return false, err
    }
    return quiz.CreatorID == userID, nil
}



// repository/user_quiz_progress.go
func (r *Repository) GetFinishedCount(quizID uint, totalQuestions int) (int64, error) {
	var count int64
	err := r.db.Model(&models.UserQuizProgress{}).
		Where("quiz_id = ? AND next_index >= ?", quizID, totalQuestions).
		Count(&count).Error
	return count, err
}



func (r *Repository) GetFinishedPlayersCount(quizID uint, totalQuestions int) (int64, error) {
    var count int64
    err := r.db.Model(&models.UserQuizProgress{}).
        Where("quiz_id = ? AND next_index >= ?", quizID, totalQuestions).
        Count(&count).Error
    if err != nil {
        log.Printf("Error counting finished players: %v", err)
        return 0, err
    }
    return count, nil
}


// repository.go
func (r *Repository) ResetQuizProgress(quizID uint) error {
    // Reset nextIndex to 0 for all users who participated in the quiz
    return r.db.Model(&models.UserQuizProgress{}).
        Where("quiz_id = ?", quizID).
        Update("next_index", 0).Error
}
