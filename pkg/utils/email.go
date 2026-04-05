package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// Estruturas exigidas pela API do Brevo
type BrevoSender struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type BrevoRecipient struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type BrevoEmailRequest struct {
	Sender      BrevoSender      `json:"sender"`
	To          []BrevoRecipient `json:"to"`
	Subject     string           `json:"subject"`
	HtmlContent string           `json:"htmlContent"`
}

// O Motor de Envio via API do Brevo
func SendEmailViaBrevo(toEmail, subject, htmlBody string) {
	apiKey := os.Getenv("BREVO_API_KEY")
	senderEmail := os.Getenv("SENDER_EMAIL")

	if apiKey == "" || senderEmail == "" {
		log.Println("⚠️ ERRO: Chave do Brevo (BREVO_API_KEY) ou E-mail Remetente (SENDER_EMAIL) não estão no Render!")
		return
	}

	reqBody := BrevoEmailRequest{
		Sender: BrevoSender{
			Name:  "StudFy",
			Email: senderEmail,
		},
		To: []BrevoRecipient{
			{Email: toEmail},
		},
		Subject:     subject,
		HtmlContent: htmlBody,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Println("Erro ao montar JSON do e-mail:", err)
		return
	}

	// Bate na API do Brevo (Passa direto pelos bloqueios de rede)
	req, err := http.NewRequest("POST", "https://api.brevo.com/v3/smtp/email", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Println("Erro ao criar requisição para o Brevo:", err)
		return
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("api-key", apiKey)
	req.Header.Set("content-type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Erro ao conectar na API do Brevo:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Println("✅ E-mail enviado com sucesso via Brevo para:", toEmail)
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("❌ Falha no envio do Brevo (Status %d): %s\n", resp.StatusCode, string(bodyBytes))
	}
}

// ==========================================================
// 1. E-MAIL DE VERIFICAÇÃO DE CONTA (CÓDIGO 6 DÍGITOS)
// ==========================================================
func SendVerificationEmail(toEmail string, name string, code string) {
	subject := "Seu código de verificação StudFy"

	htmlTpl := fmt.Sprintf(`
	<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; background: #f4f4f5;">
		<div style="background: #ffffff; padding: 30px; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.1);">
			<h2 style="color: #4f46e5; text-align: center;">Bem-vindo ao StudFy!</h2>
			<p>Olá %s,</p>
			<p>Para concluir seu cadastro e validar sua conta, use o código de verificação abaixo. <b>Ele expira em 10 minutos.</b></p>
			<div style="font-size: 36px; font-weight: bold; text-align: center; letter-spacing: 8px; color: #111827; padding: 20px; background: #f3f4f6; border-radius: 8px; margin: 20px 0;">
				%s
			</div>
			<p style="text-align: center; color: #6b7280; font-size: 12px;">Se você não se cadastrou no StudFy, apenas ignore este e-mail.</p>
		</div>
	</div>`, name, code)

	go SendEmailViaBrevo(toEmail, subject, htmlTpl)
}

// ==========================================================
// 2. E-MAIL DE RESET DE SENHA (ESQUECI A SENHA)
// ==========================================================
func SendPasswordResetEmail(toEmail string, name string, resetLink string) {
	subject := "Redefinição de Senha - StudFy"

	htmlTpl := fmt.Sprintf(`
	<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; background: #f4f4f5;">
		<div style="background: #ffffff; padding: 30px; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.1);">
			<h2 style="color: #4f46e5; text-align: center;">Recuperação de Senha</h2>
			<p>Olá %s,</p>
			<p>Recebemos um pedido para redefinir a sua senha no StudFy. Clique no botão abaixo para criar uma nova senha. <b>Este link é válido por 1 hora.</b></p>
			<div style="text-align: center; margin-top: 30px; margin-bottom: 30px;">
				<a href="%s" style="background-color: #4f46e5; color: white; padding: 15px 25px; text-decoration: none; border-radius: 5px; font-weight: bold; display: inline-block;">Redefinir Minha Senha</a>
			</div>
			<p style="text-align: center; color: #6b7280; font-size: 12px;">Se você não solicitou esta alteração, apenas ignore este e-mail.</p>
		</div>
	</div>`, name, resetLink)

	go SendEmailViaBrevo(toEmail, subject, htmlTpl)
}
