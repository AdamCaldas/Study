package utils

// Tipos de Conta (Roles)
const (
	RoleUser    = "USER"
	RoleTeacher = "TEACHER"
	RoleAdmin   = "ADMIN"
	RoleDev     = "DEV"
)

// Níveis de Acesso no Space
const (
	AccessViewer  = "VIEWER"
	AccessEditor  = "EDITOR"
	AccessMonitor = "MONITOR"
	AccessOwner   = "OWNER"
)

// Tipos de Assinatura
const (
	PlanFreeTrial = "FREE_TRIAL"
	PlanFree      = "FREE"
	PlanPro       = "PRO"
)

// Status Gerais do Sistema
const (
	StatusPending   = "pending"
	StatusActive    = "active"
	StatusCompleted = "completed"
	StatusRejected  = "rejected"
	StatusApproved  = "approved"
)
