// backend/internal/models/quiz.go
package models




import (
    "time"
    "gorm.io/gorm"
)

type Quiz struct {
    ID          uint      `json:"id" gorm:"primaryKey"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    DeletedAt   gorm.DeletedAt `json:"deleted_at" gorm:"index"`
    Title       string    `json:"title" gorm:"not null"`
    Description string    `json:"description"`
    CreatorID   uint      `json:"creator_id"`
    TimeLimit   uint      `json:"time_limit"`
    QuizCode    string    `json:"quiz_code" gorm:"unique"`
    IsActive    bool      `json:"is_active" gorm:"default:false"`
    Questions   []Question `json:"questions,omitempty" gorm:"foreignKey:QuizID"`
}

type Question struct {
    ID            uint      `json:"id" gorm:"primaryKey"`
    CreatedAt     time.Time `json:"created_at"`
    UpdatedAt     time.Time `json:"updated_at"`
    DeletedAt     gorm.DeletedAt `json:"deleted_at" gorm:"index"`
    QuizID        uint      `json:"quiz_id"`
    Text          string    `json:"text" gorm:"not null"`
    Options       []Option  `json:"options,omitempty" gorm:"foreignKey:QuestionID"`
    CorrectAnswer string    `json:"correct_answer" gorm:"not null"`
    TimeLimit     int       `json:"time_limit"`
}

type Option struct {
    ID          uint      `json:"id" gorm:"primaryKey"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    DeletedAt   gorm.DeletedAt `json:"deleted_at" gorm:"index"`
    QuestionID  uint      `json:"question_id"`
    Text        string    `json:"text" gorm:"not null"`
}

type UserQuizResponse struct {
    ID          uint      `json:"id" gorm:"primaryKey"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    DeletedAt   gorm.DeletedAt `json:"deleted_at" gorm:"index"`
    UserID      uint      `json:"user_id"`
    QuizID      uint      `json:"quiz_id"`
    QuestionID  uint      `json:"question_id"`
    Answer      string    `json:"answer"`
    Score       int       `json:"score"`
    TimeSpent   int       `json:"time_spent"`
}

type ParticipantInfo struct {
    UserID   uint   `json:"userId"`
    Username string `json:"username"`
    Status   string `json:"status"`
}

type QuizParticipant struct {
    ID          uint      `json:"id" gorm:"primaryKey"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    DeletedAt   gorm.DeletedAt `json:"deleted_at" gorm:"index"`
    QuizID      uint      `json:"quiz_id"`
    UserID      uint      `json:"user_id"`
}

// models/quiz.go
type LeaderboardEntry struct {
    Username    string `json:"username"`
    TotalScore int    `json:"score"` // Changed to TotalScore to match the SQL query
}


type UserQuizProgress struct {
    ID        uint      `gorm:"primaryKey"`
    UserID    uint      `gorm:"not null"`
    QuizID    uint      `gorm:"not null"`
    NextIndex int       `gorm:"not null"` // The index of the next question to serve
    CreatedAt time.Time
    UpdatedAt time.Time
}

// Optionally override the default table name
func (UserQuizProgress) TableName() string {
    return "user_quiz_progress"
}
