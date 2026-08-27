package utils

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// ==========================================================
// 📄 PAGINAÇÃO PADRÃO DE TODAS AS LISTAGENS
// ==========================================================
// Sem isso, uma turma com muitos flashcards/questões/provas devolvia TUDO num
// JSON só — o que derruba o app no celular do aluno e trava o banco no pico.
//
// Uso no handler:
//
//	p := utils.GetPage(c)                       // lê ?page= e ?limit=
//	query.Count(&total)                          // total antes do recorte
//	query.Offset(p.Offset()).Limit(p.Limit)...   // aplica o recorte
//	c.JSON(200, gin.H{"itens": itens, "pagination": p.Meta(total)})

const (
	defaultPageSize = 30
	maxPageSize     = 100
)

type Page struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

// Offset devolve quantos registros pular no banco.
func (p Page) Offset() int { return (p.Page - 1) * p.Limit }

// Meta monta o bloco de paginação da resposta, para o front saber se há mais.
func (p Page) Meta(total int64) gin.H {
	totalPages := int64(0)
	if p.Limit > 0 {
		totalPages = (total + int64(p.Limit) - 1) / int64(p.Limit)
	}
	return gin.H{
		"page":        p.Page,
		"limit":       p.Limit,
		"total":       total,
		"total_pages": totalPages,
		"has_more":    int64(p.Page) < totalPages,
	}
}

// GetPage lê ?page= e ?limit= da URL com limites seguros.
// page começa em 1; limit é travado em maxPageSize para ninguém pedir a base inteira.
func GetPage(c *gin.Context) Page {
	page := 1
	if v, err := strconv.Atoi(c.Query("page")); err == nil && v > 0 {
		page = v
	}

	limit := defaultPageSize
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 {
		limit = v
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	return Page{Page: page, Limit: limit}
}
