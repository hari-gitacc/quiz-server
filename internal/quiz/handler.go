// backend/internal/quiz/handler.go
package quiz

import (
	"encoding/json"
	"log"
	"net/http"
	"quiz-system/internal/models"

	"github.com/gorilla/mux"
)

type Handler struct {
    service *Service
}

func NewHandler(service *Service) *Handler {
    return &Handler{service: service}
}

func (h *Handler) CreateQuiz(w http.ResponseWriter, r *http.Request) {
    userID, ok := r.Context().Value("user_id").(uint)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    var quiz models.Quiz
    if err := json.NewDecoder(r.Body).Decode(&quiz); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    quiz.CreatorID = userID

    if err := h.service.CreateQuiz(&quiz); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(quiz)
}

func (h *Handler) StartQuiz(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    quizCode := vars["quizCode"]
    userID := r.Context().Value("user_id").(uint)

    log.Printf("Starting quiz %s for user %d", quizCode, userID)

    if err := h.service.StartQuiz(quizCode, userID); err != nil {
        log.Printf("Error starting quiz: %v", err)
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    log.Printf("Quiz %s started successfully", quizCode)
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "Quiz started"})
}

func (h *Handler) GetQuiz(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    quizCode := vars["quizCode"]

    quiz, err := h.service.GetQuizByCode(quizCode)
    if err != nil {
        http.Error(w, "Quiz not found", http.StatusNotFound)
        return
    }

    json.NewEncoder(w).Encode(quiz)
}



func (h *Handler) JoinQuiz(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    quizCode := vars["quizCode"]
    userID, ok := r.Context().Value("user_id").(uint)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    if err := h.service.JoinQuiz(quizCode, userID); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    w.WriteHeader(http.StatusOK)
}

func (h *Handler) SubmitAnswer(w http.ResponseWriter, r *http.Request) {
    var response models.UserQuizResponse
  
    if err := json.NewDecoder(r.Body).Decode(&response); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    response.UserID = r.Context().Value("user_id").(uint)

    score, err := h.service.ProcessAnswer(&response)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(map[string]int{"score": score})
}

func (h *Handler) GetMyQuizzes(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("user_id").(uint)
    
    quizzes, err := h.service.GetQuizzesByCreator(userID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(quizzes)
}


// backend/internal/quiz/handler.go
func (h *Handler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    quizCode := vars["quizCode"]

    leaderboard, err := h.service.GetLeaderboard(quizCode)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(leaderboard)
}