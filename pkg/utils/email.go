package utils

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"os"
)

type EmailVerificationData struct {
	Name string
	Code string
}

func SendVerificationEmail(toEmail string, name string, code string) {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)

	// O HTML do E-mail (Bem limpo e direto)
	htmlTpl := `
	<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; background: #f4f4f5;">
		<div style="background: #ffffff; padding: 30px; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.1);">
			<h2 style="color: #4f46e5; text-align: center;">Bem-vindo ao StudFy!</h2>
			<p>Olá {{.Name}},</p>
			<p>Para concluir seu cadastro e validar sua conta, use o código de verificação abaixo. <b>Ele expira em 10 minutos.</b></p>
			<div style="font-size: 36px; font-weight: bold; text-align: center; letter-spacing: 8px; color: #111827; padding: 20px; background: #f3f4f6; border-radius: 8px; margin: 20px 0;">
				{{.Code}}
			</div>
			<p style="text-align: center; color: #6b7280; font-size: 12px;">Se você não se cadastrou no StudFy, apenas ignore este e-mail.</p>
		</div>
	</div>`

	t, err := template.New("email").Parse(htmlTpl)
	if err != nil {
		log.Println("Erro ao compilar template de email:", err)
		return
	}

	var body bytes.Buffer
	mimeHeaders := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	body.Write([]byte(fmt.Sprintf("Subject: Seu código de verificação StudFy \n%s\n\n", mimeHeaders)))
	t.Execute(&body, EmailVerificationData{Name: name, Code: code})

	err = smtp.SendMail(smtpHost+":"+smtpPort, auth, smtpUser, []string{toEmail}, body.Bytes())
	if err != nil {
		log.Println("Erro crítico SMTP (Não conseguiu enviar):", err)
	} else {
		log.Println("✅ E-mail de verificação enviado com sucesso para:", toEmail)
	}
}

// ==========================================================
// 2. E-MAIL DE RESET DE SENHA (ESQUECI A SENHA)
// ==========================================================
type EmailPasswordResetData struct {
	Name string
	Link string
}

func SendPasswordResetEmail(toEmail string, name string, resetLink string) {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)

	htmlTpl := `
	<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; background: #f4f4f5;">
		<div style="background: #ffffff; padding: 30px; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.1);">
			<h2 style="color: #4f46e5; text-align: center;">Recuperação de Senha</h2>
			<p>Olá {{.Name}},</p>
			<p>Recebemos um pedido para redefinir a sua senha no StudFy. Clique no botão abaixo para criar uma nova senha. <b>Este link é válido por 1 hora.</b></p>
			<div style="text-align: center; margin-top: 30px; margin-bottom: 30px;">
				<a href="{{.Link}}" style="background-color: #4f46e5; color: white; padding: 15px 25px; text-decoration: none; border-radius: 5px; font-weight: bold; display: inline-block;">Redefinir Minha Senha</a>
			</div>
			<p style="text-align: center; color: #6b7280; font-size: 12px;">Se você não solicitou esta alteração, apenas ignore este e-mail.</p>
		</div>
	</div>`

	t, err := template.New("reset_email").Parse(htmlTpl)
	if err != nil {
		log.Println("Erro ao compilar template de reset:", err)
		return
	}

	var body bytes.Buffer
	mimeHeaders := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	body.Write([]byte(fmt.Sprintf("Subject: Redefinição de Senha - StudFy \n%s\n\n", mimeHeaders)))
	t.Execute(&body, EmailPasswordResetData{Name: name, Link: resetLink})

	err = smtp.SendMail(smtpHost+":"+smtpPort, auth, smtpUser, []string{toEmail}, body.Bytes())
	if err != nil {
		log.Println("Erro crítico SMTP (Não conseguiu enviar reset):", err)
	} else {
		log.Println("✅ E-mail de redefinição enviado com sucesso para:", toEmail)
	}
}
