package utils

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetUserID extrai o ID do usuário do contexto do Gin de forma 100% segura.
// Ele tenta converter direto e, se falhar, usa o fallback anti-panic.
func GetUserID(c *gin.Context) (uuid.UUID, error) {
	userIDInterface, exists := c.Get("userID")
	if !exists {
		return uuid.Nil, fmt.Errorf("usuário não autenticado no contexto")
	}

	switch v := userIDInterface.(type) {
	case uuid.UUID:
		return v, nil
	case string:
		return uuid.Parse(v)
	default:
		// Fallback super seguro garantindo que não vai dar panic
		return uuid.Parse(fmt.Sprintf("%v", userIDInterface))
	}
}
