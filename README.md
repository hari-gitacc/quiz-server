
## Quiz System Backend

This is a Go-based backend server that provides REST API endpoints and WebSocket functionality for the quiz system.

### Prerequisites

- Go (version 1.21 or higher)
- PostgreSQL (version 13 or higher)
- Redis (version 6 or higher)

### Installation

1. Navigate to the backend directory:
```bash
cd backend
```

2. Install Go dependencies:
```bash
go mod download
```

3. Create a `.env` file in the backend root directory with the following content:
```
DB_HOST=localhost
DB_PORT=5432
DB_USER=your_postgres_user
DB_PASSWORD=your_postgres_password
DB_NAME=quiz_system
REDIS_ADDR=localhost:6379
JWT_SECRET=your_jwt_secret_key
```

### Database Setup

1. Create a PostgreSQL database:
```sql
CREATE DATABASE quiz_system;
```

2. The application will automatically create the required tables on startup through GORM auto-migration.

### Running the Server

To start the development server with hot reload:
```bash
air
```
or
```bash
go run main.go
```

The server will start on `http://localhost:8080`.

### Project Structure

- `internal/auth`: Authentication-related components
- `internal/quiz`: Quiz management components
- `internal/models`: Database models
- `pkg/database`: Database configuration
- `pkg/cache`: Redis cache implementation
- `pkg/websocket`: WebSocket hub and client handling

### API Endpoints

Authentication:
- POST `/api/auth/register`: User registration
- POST `/api/auth/login`: User login

Quiz Management:
- GET `/api/quiz/my-quizzes`: Get user's quizzes
- POST `/api/quiz`: Create new quiz
- GET `/api/quiz/{quizCode}`: Get quiz details
- POST `/api/quiz/{quizCode}/join`: Join a quiz
- POST `/api/quiz/{quizCode}/start`: Start a quiz
- POST `/api/quiz/answer`: Submit answer
- GET `/api/quiz/{quizCode}/leaderboard`: Get quiz leaderboard

WebSocket:
- WS `/ws/{quizCode}`: WebSocket connection for real-time quiz participation

### Development Notes

1. The backend uses CORS middleware configured for `http://localhost:3000`. Adjust the allowed origins in `main.go` if needed.

2. The WebSocket implementation includes automatic reconnection and message handling for various quiz events.

3. Redis is used for caching quiz data and managing leaderboards. Ensure Redis is running before starting the server.

4. JWT tokens are used for authentication. The tokens expire after 24 hours.

### Error Handling

The system includes comprehensive error handling and logging. Check the server logs for troubleshooting.

This documentation provides all the necessary information to set up and run both the frontend and backend components of the quiz system. Let me know if you need any clarification or additional information.