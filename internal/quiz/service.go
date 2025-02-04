// backend/internal/quiz/service.go
package quiz

import (
	"errors"
	"log"
	"math/rand"
	"quiz-system/internal/models"
	"quiz-system/pkg/cache"
	"quiz-system/pkg/websocket"
)

type Service struct {
	repo  *Repository
	cache *cache.RedisCache
	wsHub *websocket.Hub
}

func NewService(repo *Repository, cache *cache.RedisCache, wsHub *websocket.Hub) *Service {
	return &Service{
		repo:  repo,
		cache: cache,
		wsHub: wsHub,
	}
}

// backend/internal/quiz/service.go
type LeaderboardEntry struct {
    Username string `json:"username"`
    Score    int    `json:"score"`
}


var userQuizProgress = map[uint]map[string]int{} 


func (s *Service) GetLeaderboard(quizCode string) ([]models.LeaderboardEntry, error) {
    quiz, err := s.GetQuizByCode(quizCode)
    if err != nil {
        return nil, err
    }

    entries, err := s.repo.GetLeaderboard(quiz.ID)
    if err != nil {
        return nil, err
    }

    return entries, nil
}

func (s *Service) StartQuiz(quizCode string, userID uint) error {
	log.Printf("StartQuiz called for quiz %s by user %d", quizCode, userID)

	quiz, err := s.GetQuizByCode(quizCode)
	if err != nil {
		log.Printf("Error getting quiz: %v", err)
		return err
	}

            // Reset progress for all participants for this quiz.
            if err := s.repo.ResetQuizProgress(quiz.ID); err != nil {
                return err
            }

	questions, err := s.repo.GetQuizQuestions(quiz.ID)
	if err != nil {
		log.Printf("Error getting questions: %v", err)
		return err
	}

	if len(questions) == 0 {
		log.Printf("No questions found for quiz %s", quizCode)
		return errors.New("no questions found for quiz")
	}



	// Set quiz as active
	quiz.IsActive = true
	if err := s.repo.UpdateQuiz(quiz); err != nil {
		log.Printf("Error updating quiz status: %v", err)
		return err
	}

    firstQuestionDTO := questions[0].ToDTO(true)
    messageData := map[string]interface{}{
		"question": firstQuestionDTO,
		"index":    0,
		"total":    len(questions),
		"quizId":   quiz.ID,
	}

	log.Printf("Broadcasting first question data: %+v", messageData)
	s.wsHub.BroadcastMessage(quizCode, "question", messageData)

	return nil
}


func (s *Service) RemoveParticipant(quizCode string, userID uint) error {
    quiz, err := s.GetQuizByCode(quizCode)
    if err != nil {
        log.Printf("Error getting quiz by code %s: %v", quizCode, err)
        return err
    }

    // Check if the user is the host
    if quiz.CreatorID == userID {
        log.Printf("User %d is the host of quiz %s, ignoring removal", userID, quizCode)
        return nil
    }

    // Remove from database
    err = s.repo.RemoveParticipant(quiz.ID, userID)
    if err != nil {
        log.Printf("Error removing participant %d from quiz %s in database: %v", userID, quizCode, err)
        return err
    }

    // Clear user's progress
    err = s.repo.ClearUserProgress(quiz.ID, userID)
    if err != nil {
        log.Printf("Error clearing progress for user %d in quiz %s: %v", userID, quizCode, err)
        // Continue execution even if clearing progress fails
    }

    // Remove any cached data for this user
    err = s.cache.RemoveUserQuizData(quizCode, userID)
    if err != nil {
        log.Printf("Error clearing cached data for user %d in quiz %s: %v", userID, quizCode, err)
        // Continue execution even if cache clearing fails
    }

    // Update participant count and notify all clients
    if s.wsHub != nil {
        s.wsHub.SendParticipantList(quizCode)
    }

    log.Printf("Successfully removed participant %d from quiz %s", userID, quizCode)
    return nil
}

func (s *Service) GetQuizzesByCreator(userID uint) ([]models.Quiz, error) {
	return s.repo.GetQuizzesByCreator(userID)
}

// In service.go
func (s *Service) HandleNextQuestion(quizCode string, currentIndex int) error {
    log.Printf("Handling next question for quiz %s, current index: %d", quizCode, currentIndex)
    
    quiz, err := s.GetQuizByCode(quizCode)
    if err != nil {
        log.Printf("Error getting quiz: %v", err)
        return err
    }

    questions, err := s.repo.GetQuizQuestions(quiz.ID)
    if err != nil {
        log.Printf("Error getting questions: %v", err)
        return err
    }

    nextIndex := currentIndex + 1
    log.Printf("Next index will be: %d, total questions: %d", nextIndex, len(questions))

    if nextIndex >= len(questions) {
        log.Printf("Quiz %s finished, broadcasting quiz_end", quizCode)
        s.wsHub.BroadcastMessage(quizCode, "quiz_end", nil)
        return nil
    }

    nextQuestion := questions[nextIndex]
    nextQuestionDTO := nextQuestion.ToDTO(true)

    messageData := map[string]interface{}{
        "question": nextQuestionDTO,
        "index":    nextIndex,
        "total":    len(questions),
        "quizId":   quiz.ID,
    }

    log.Printf("Broadcasting next question for quiz %s: %+v", quizCode, messageData)
    s.wsHub.BroadcastMessage(quizCode, "question", messageData)
    
    return nil
}


func (s *Service) CreateQuiz(quiz *models.Quiz) error {
	// Generate unique quiz code
	quiz.QuizCode = generateQuizCode()
	quiz.IsActive = false

	if err := s.repo.CreateQuiz(quiz); err != nil {
		return err
	}

	// Cache the quiz
	return s.cache.SetQuiz(quiz)
}

func (s *Service) GetQuizByCode(code string) (*models.Quiz, error) {
	// Try to get from cache first
	quiz, err := s.cache.GetQuiz(code)
	if err == nil {
		return quiz, nil
	}

	// If not in cache, get from database
	quiz, err = s.repo.GetQuizByCode(code)
	log.Printf("Quiz: %v", quiz)
	if err != nil {
		return nil, err
	}

	// Update cache
	s.cache.SetQuiz(quiz)
	return quiz, nil
}

func (s *Service) JoinQuiz(quizCode string, userID uint) error {
    quiz, err := s.GetQuizByCode(quizCode)
    if err != nil {
        return err
    }

    if userID == quiz.CreatorID {
        log.Printf("User %d is the host for quiz %s", userID, quizCode)
        return nil
    }

    err = s.repo.AddParticipant(quiz.ID, userID)
    if err != nil {
        return err
    }

    // Notify WebSocket hub of the new participant
    if s.wsHub != nil { // Assuming you have a reference to the WebSocket hub
        s.wsHub.SendParticipantList(quizCode)
    }

    return nil
}


func (s *Service) ProcessAnswer(response *models.UserQuizResponse) (int, error) {
    // Retrieve the quiz details first.
    quiz, err := s.repo.GetQuizByID(response.QuizID)
    if err != nil {
        return 0, err
    }
    // If the answer comes from the host, ignore it.
    if response.UserID == quiz.CreatorID {
        log.Printf("User %d is host; skipping answer processing.", response.UserID)
        return 0, nil
    }

    // Retrieve the question details
    question, err := s.repo.GetQuestion(response.QuestionID)
    if err != nil {
        return 0, err
    }


    // Calculate score based on answer, correct answer, and time spent
    score := calculateScore(response.Answer, question.CorrectAnswer, response.TimeSpent)
    response.Score = score

    // Save the user's response to the database
    if err := s.repo.SaveResponse(response); err != nil {
        return 0, err
    }

    // Get the user's current progress (next question index)
    currentIndex, err := s.repo.GetUserQuestionIndex(response.UserID, quiz.ID)
    if err != nil {
        currentIndex = 0
    }
    log.Printf("User %d is at question index %d", response.UserID, currentIndex)

    // Increment progress
    newIndex := currentIndex + 1
    if err := s.repo.UpdateUserQuestionIndex(response.UserID, quiz.ID, newIndex); err != nil {
        log.Printf("Error updating question index for user %d: %v", response.UserID, err)
    }

    // Trigger sending next question only to this participant
    go func(userID uint, quizCode string, nextIndex int) {
        if err := s.HandleNextQuestionForUser(userID, quizCode, nextIndex); err != nil {
            log.Printf("Error sending next question to user %d: %v", userID, err)
        }
    }(response.UserID, quiz.QuizCode, newIndex)

    return score, nil
}

func (s *Service) HandleNextQuestionForUser(userID uint, quizCode string, nextIndex int) error {
    log.Printf("Handling next question for user %d in quiz %s, next index: %d", userID, quizCode, nextIndex)
    
    quiz, err := s.GetQuizByCode(quizCode)
    if err != nil {
        log.Printf("Error getting quiz: %v", err)
        return err
    }

    questions, err := s.repo.GetQuizQuestions(quiz.ID)
    if err != nil {
        log.Printf("Error getting questions: %v", err)
        return err
    }
    totalQuestions := len(questions)
    log.Printf("Total questions for quiz %s: %d", quizCode, totalQuestions)

    // Check if the user is host; if so, skip sending question.
    isHost, err := s.repo.IsUserHost(quiz.ID, userID)
    if err != nil {
        log.Printf("Error checking host status: %v", err)
        isHost = false
    }
    if isHost {
        log.Printf("User %d is host; skipping sending question.", userID)
        return nil
    }

    if nextIndex >= totalQuestions {
        log.Printf("User %d has finished quiz %s", userID, quizCode)

        finishedCount, err := s.repo.GetFinishedPlayersCount(quiz.ID, totalQuestions)
        if err != nil {
            return err
        }
        totalParticipants, err := s.repo.GetUniqueParticipantsForQuiz(quiz.ID)
        if err != nil {
            return err
        }
        log.Printf("Finished count: %d, Total participants: %d", finishedCount, totalParticipants)

        if finishedCount >= totalParticipants {
            log.Printf("All participants finished quiz %s. Broadcasting final leaderboard.", quizCode)
            if err := s.updateLeaderboard(quiz.ID); err != nil {
                log.Printf("Error updating leaderboard: %v", err)
            }
            leaderboard, err := s.cache.GetLeaderboard(quiz.QuizCode)
            if err != nil {
                log.Printf("Error retrieving leaderboard from cache: %v", err)
            }
            s.wsHub.BroadcastMessage(quizCode, "final_leaderboard", leaderboard)
        } else {
            log.Printf("User %d finished, waiting for others in quiz %s", userID, quizCode)
            s.wsHub.SendMessageToUser(userID, "quiz_end_wait", map[string]string{
                "message": "You have finished the quiz. Please wait for other players to finish.",
            })
        }
        return nil
    }

    // Otherwise, send the next question only to this participant.
    nextQuestion := questions[nextIndex]
    messageData := map[string]interface{}{
        "question": nextQuestion.ToDTO(false),
        "index":    nextIndex,
        "total":    totalQuestions,
        "quizId":   quiz.ID,
    }

    s.wsHub.SendMessageToUser(userID, "question", messageData)
    return nil
}







func (s *Service) updateLeaderboard(quizID uint) error {
    entries, err := s.repo.GetLeaderboard(quizID)
    if err != nil {
        return err
    }

    quiz, err := s.repo.GetQuizByID(quizID)
    if err != nil {
        return err
    }

    // Convert entries to map[string]int format for cache
    scores := make(map[string]int)
    for _, entry := range entries {
        scores[entry.Username] = entry.TotalScore
    }

    log.Printf("%v scores of the players", scores)

    return s.cache.SetLeaderboard(quiz.QuizCode, scores)
}

func generateQuizCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, 6)
	for i := range code {
		code[i] = charset[rand.Intn(len(charset))]
	}
	return string(code)
}

func calculateScore(answer, correctAnswer string, timeSpent int) int {
    log.Printf("Calculating score. Answer: %q, Correct: %q, Time Spent: %d", answer, correctAnswer, timeSpent)
    if answer != correctAnswer {
        return 0
    }
    score := 1000
    timeDeduction := timeSpent * 10
    score -= timeDeduction
    if score < 0 {
        score = 0
    }
    return score
}

