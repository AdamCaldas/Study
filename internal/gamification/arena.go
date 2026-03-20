package gamification

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ==========================================================
// 🎮 FASE 6: MODO ARENA (WebSockets em Tempo Real)
// ==========================================================

// Configura o Upgrader (Transforma a requisição HTTP normal em um túnel WebSocket)
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Permite que o Next.js e o App Mobile se conectem livremente
	},
}

// Memória RAM do Servidor: Guarda quem está conectado em qual Space
// Map: SpaceID -> Lista de Conexões Ativas
var arenaRooms = make(map[string][]*websocket.Conn)
var arenaMutex = sync.Mutex{} // Protege a memória para não dar erro se entrarem 100 alunos de uma vez

// Função principal que segura a conexão aberta
func JoinArenaMode(c *gin.Context) {
	spaceID := c.Param("space_id")

	// 1. "Upgrade" da conexão para WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return // Se der erro, simplesmente ignora e fecha
	}
	defer conn.Close()

	// 2. Coloca o aluno/professor na "Sala" correta da memória
	arenaMutex.Lock()
	arenaRooms[spaceID] = append(arenaRooms[spaceID], conn)
	arenaMutex.Unlock()

	// 3. Loop infinito escutando as mensagens dessa conexão
	for {
		var msg map[string]interface{}

		// Espera alguém mandar uma mensagem JSON
		err := conn.ReadJSON(&msg)
		if err != nil {
			// Se der erro (ex: aluno fechou o app), quebra o loop e desconecta
			break
		}

		// 4. EFEITO KAHOOT: O que chega aqui, nós repassamos para a sala inteira!
		// Ex: Se o professor mandar {"action": "start_question", "question_id": "123"}
		// O Go envia isso pra todos os alunos renderizarem os botões de resposta!
		arenaMutex.Lock()
		for _, client := range arenaRooms[spaceID] {
			// Escreve a mesma mensagem de volta para cada cliente da sala
			err := client.WriteJSON(msg)
			if err != nil {
				client.Close()
			}
		}
		arenaMutex.Unlock()
	}

	// 5. Limpeza: Se o loop quebrou, remove a conexão morta da memória
	arenaMutex.Lock()
	var activeConnections []*websocket.Conn
	for _, c := range arenaRooms[spaceID] {
		if c != conn {
			activeConnections = append(activeConnections, c)
		}
	}
	arenaRooms[spaceID] = activeConnections
	arenaMutex.Unlock()
}
