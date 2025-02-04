// backend/internal/models/dto.go
package models

// models/dto.go
type QuestionDTO struct {
    ID            uint       `json:"id"`
    Text          string     `json:"text"`
    Options       []OptionDTO `json:"options"`
    TimeLimit     int        `json:"time_limit"`
    CorrectAnswer string     `json:"correct_answer,omitempty"` // Only for host
}

type OptionDTO struct {
    ID   uint   `json:"id"`
    Text string `json:"text"`
}

// In models/dto.go
func (q Question) ToDTO(isHost bool) QuestionDTO {
    optionDTOs := make([]OptionDTO, len(q.Options))
    for i, opt := range q.Options {
        optionDTOs[i] = OptionDTO{
            ID:   opt.ID,
            Text: opt.Text,
        }
    }
    
    timeLimit := q.TimeLimit
    if timeLimit <= 0 {
        timeLimit = 30 // Default 30 seconds if not set
    }
    
    dto := QuestionDTO{
        ID:        q.ID,
        Text:      q.Text,
        Options:   optionDTOs,
        TimeLimit: timeLimit,
    }
    if isHost {
        dto.CorrectAnswer = q.CorrectAnswer
    }
    return dto
}
