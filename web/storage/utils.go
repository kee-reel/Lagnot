package storage

import (
	"encoding/json"
	"fmt"
	"late/security"
	"late/utils"
	"time"
)

func makeTokenDurationMap() map[TokenType]time.Duration {
	default_duration, err := time.ParseDuration(utils.Env("WEB_TOKEN_DEFAULT_DURATION"))
	utils.Err(err)
	access_duration, err := time.ParseDuration(utils.Env("WEB_TOKEN_ACCESS_DURATION"))
	utils.Err(err)
	return map[TokenType]time.Duration{
		RegisterToken: default_duration,
		VerifyToken:   default_duration,
		AccessToken:   access_duration,
		RestoreToken:  default_duration,
		SuspendToken:  default_duration,
	}
}

func makeKey(token_type TokenType, email string, ip string) string {
	return fmt.Sprintf("%d:%s:%s", token_type, email, ip)
}

func (s *Storage) addToken(token_type TokenType, email string, ip string, extra_data *map[string]string) string {
	token_data := TokenData{
		IP:    ip,
		Email: email,
		Extra: extra_data,
	}
	json_data, err := json.Marshal(token_data)
	utils.Err(err)
	key := makeKey(token_type, email, ip)
	token := security.GenerateToken()
	s.kv.Send("MULTI")
	s.kv.Send("SET", fmt.Sprintf("%d:%s", token_type, token),
		json_data, "EX", s.token_expiration[token_type].Seconds())
	s.kv.Send("SET", key, token, "EX", s.token_expiration[token_type].Seconds())
	_, err = s.kv.Do("EXEC")
	utils.Err(err)
	return token
}
