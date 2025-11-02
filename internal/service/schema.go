package service

type InitialSessionRequest struct {
	GameName string `json:"game_name"`
	TagLine  string `json:"tag_line"`
}

type InitialSessionResponse struct {
	SessionId string `json:"session_id"`
}

type ChatRequest struct {
	SessionId string `json:"session_id"`
	Message   string `json:"message"`
}
