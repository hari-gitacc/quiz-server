// backend/pkg/cache/redis.go
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"quiz-system/internal/models"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisCache struct {
    client *redis.Client
    ctx    context.Context
}

func NewRedisCache(addr string) *RedisCache {
    client := redis.NewClient(&redis.Options{
        Addr: addr,
    })
    return &RedisCache{
        client: client,
        ctx:    context.Background(),
    }
}

func (c *RedisCache) SetQuiz(quiz *models.Quiz) error {
    data, err := json.Marshal(quiz)
    if err != nil {
        return err
    }

    key := "quiz:" + quiz.QuizCode
    return c.client.Set(c.ctx, key, data, 24*time.Hour).Err()
}

func (c *RedisCache) GetQuiz(code string) (*models.Quiz, error) {
    key := "quiz:" + code
    data, err := c.client.Get(c.ctx, key).Bytes()
    if err != nil {
        return nil, err
    }

    var quiz models.Quiz
    err = json.Unmarshal(data, &quiz)
    return &quiz, err
}

func (c *RedisCache) UpdateLeaderboard(quizCode string, entries []models.LeaderboardEntry) error {
    key := "leaderboard:" + quizCode
    
    // Clear existing leaderboard
    pipe := c.client.Pipeline()
    pipe.Del(c.ctx, key)
    
    // Add new entries
    for _, entry := range entries {
        pipe.ZAdd(c.ctx, key, &redis.Z{
            Score:  float64(entry.TotalScore),
            Member: entry.Username,
        })
    }
    
    // Set expiration
    pipe.Expire(c.ctx, key, 24*time.Hour)
    
    _, err := pipe.Exec(c.ctx)
    return err
}

func (c *RedisCache) SetLeaderboard(quizCode string, scores map[string]int) error {
    key := "leaderboard:" + quizCode
    
    // Clear existing leaderboard
    if err := c.client.Del(c.ctx, key).Err(); err != nil {
        return err
    }
    
    // Add all scores in a pipeline
    pipe := c.client.Pipeline()
    for username, score := range scores {
        pipe.ZAdd(c.ctx, key, &redis.Z{
            Score:  float64(score),
            Member: username,
        })
    }
    pipe.Expire(c.ctx, key, 24*time.Hour)
    
    _, err := pipe.Exec(c.ctx)
    return err
}

func (c *RedisCache) RemoveUserQuizData(quizCode string, userID uint) error {
    key := fmt.Sprintf("quiz:%s:user:%d", quizCode, userID)
    return c.client.Del(context.Background(), key).Err()
}

func (c *RedisCache) GetLeaderboard(quizCode string) ([]models.LeaderboardEntry, error) {
    key := "leaderboard:" + quizCode
    
    // Get all entries sorted by score (descending)
    results, err := c.client.ZRevRangeWithScores(c.ctx, key, 0, -1).Result()
    if err != nil {
        return nil, err
    }
    
    entries := make([]models.LeaderboardEntry, len(results))
    for i, z := range results {
        entries[i] = models.LeaderboardEntry{
            Username:   z.Member.(string),
            TotalScore: int(z.Score),
        }
    }
    
    return entries, nil
}