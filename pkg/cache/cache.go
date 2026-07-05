package cache

import (
	"time"

	"github.com/patrickmn/go-cache"
)

// AppCache é a nossa memória global super-rápida.
// Os dados ficam salvos por 5 minutos por padrão.
// A cada 10 minutos, ele faz uma "faxina" e apaga o que já expirou.
var AppCache = cache.New(5*time.Minute, 10*time.Minute)
