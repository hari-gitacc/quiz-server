package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/cors"

	"quiz-system/internal/auth"
	"quiz-system/internal/models"
	"quiz-system/internal/quiz"
	"quiz-system/pkg/cache"
	"quiz-system/pkg/database"
	"quiz-system/pkg/websocket"

	"github.com/gorilla/mux"
)

func main() {
    // Load environment variables
    if err := godotenv.Load(); err != nil {
        log.Printf("Warning: .env file not found")
    }

    // Initialize database
    dbConfig := &database.Config{
        Host:     os.Getenv("DB_HOST"),
        Port:     os.Getenv("DB_PORT"),
        User:     os.Getenv("DB_USER"),
        Password: os.Getenv("DB_PASSWORD"),
        DBName:   os.Getenv("DB_NAME"),
    }

    db, err := database.NewPostgresDB(dbConfig)
    if err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }
    err = db.AutoMigrate(
        &models.User{},
        &models.Quiz{},
        &models.Question{},
        &models.Option{},
        &models.UserQuizResponse{},
        &models.UserQuizProgress{}, // <-- Add this line
    )
    
    if err != nil {
        log.Fatalf("Failed to migrate database: %v", err)
    }
    // Initialize Redis cache
    redisCache := cache.NewRedisCache(os.Getenv("REDIS_ADDR"))

    // Initialize WebSocket hub
    wsHub := websocket.NewHub()
    // In main.go where you initialize the hub
    go wsHub.Run()

    // Initialize repositories
    authRepo := auth.NewRepository(db)
    quizRepo := quiz.NewRepository(db)

    // Initialize services
    jwtSecret := os.Getenv("JWT_SECRET")
    authService := auth.NewService(authRepo, jwtSecret)
    quizService := quiz.NewService(quizRepo, redisCache, wsHub)
    wsHub.SetQuizService(quizService)

    go wsHub.Run()


    // Initialize handlers
    authHandler := auth.NewHandler(authService)
    quizHandler := quiz.NewHandler(quizService)

    // Setup router
    router := mux.NewRouter()

    // CORS middleware configuration
    corsMiddleware := cors.New(cors.Options{
        AllowedOrigins:   []string{"http://localhost:3000"},    // Frontend URL
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Requested-With"},
        ExposedHeaders:   []string{"Content-Length"},
        AllowCredentials: true,
        MaxAge:           300, // Maximum value not ignored by any of major browsers
    })

    // Apply CORS middleware to router
    handler := corsMiddleware.Handler(router)

    // Auth routes - no JWT required
    router.HandleFunc("/api/auth/register", authHandler.Register).Methods("POST", "OPTIONS")
    router.HandleFunc("/api/auth/login", authHandler.Login).Methods("POST", "OPTIONS")

    // Quiz routes - JWT required
    apiRouter := router.PathPrefix("/api").Subrouter()
    apiRouter.Use(auth.JWTMiddleware(jwtSecret))

    apiRouter.HandleFunc("/quiz/my-quizzes", quizHandler.GetMyQuizzes).Methods("GET")
    apiRouter.HandleFunc("/quiz", quizHandler.CreateQuiz).Methods("POST", "OPTIONS")
    apiRouter.HandleFunc("/quiz/{quizCode}/leaderboard", quizHandler.GetLeaderboard).Methods("GET")
    apiRouter.HandleFunc("/quiz/{quizCode}/start", quizHandler.StartQuiz).Methods("POST")  // Add this
    apiRouter.HandleFunc("/quiz/{quizCode}", quizHandler.GetQuiz).Methods("GET", "OPTIONS")
    apiRouter.HandleFunc("/quiz/{quizCode}/join", quizHandler.JoinQuiz).Methods("POST", "OPTIONS")
    apiRouter.HandleFunc("/quiz/answer", quizHandler.SubmitAnswer).Methods("POST", "OPTIONS")
    // WebSocket endpoint
    router.HandleFunc("/ws/{quizCode}", wsHub.HandleWebSocket)
    // In main.go where routes are defined
 
    

    // Initialize random seed
    rand.Seed(time.Now().UnixNano())

    // Setup server with CORS handler
    srv := &http.Server{
        Addr:         ":8080",
        Handler:      handler,  // Use the CORS handler
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
    }

    // Start server in a goroutine
    go func() {
        log.Printf("Server starting on port 8080")
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Failed to start server: %v", err)
        }
    }()

    // Graceful shutdown setup
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt)
    <-c

    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        log.Printf("Server forced to shutdown: %v", err)
    }

    log.Println("Server shutdown gracefully")
}