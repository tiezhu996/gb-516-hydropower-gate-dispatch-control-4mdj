package dto

import "time"

type PageQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Search   string `form:"search"`
	Status   string `form:"status"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=80"`
	Password string `json:"password" binding:"required,min=6,max=100"`
}

type LoginResponse struct {
	Token       string `json:"token"`
	ExpiresIn   int64  `json:"expiresIn"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

type TransitionRequest struct {
	Status          string `json:"status" binding:"required,max=40"`
	ExpectedVersion uint   `json:"expectedVersion" binding:"required"`
	Reason          string `json:"reason" binding:"required,min=3,max=500"`
}

type AuditSummaryQuery struct {
	WindowHours int `form:"windowHours"`
}

func (q AuditSummaryQuery) Window() time.Duration {
	hours := q.WindowHours
	if hours < 1 {
		hours = 24
	}
	if hours > 24*90 {
		hours = 24 * 90
	}
	return time.Duration(hours) * time.Hour
}

type EntityHistoryQuery struct {
	Limit int `form:"limit"`
}

func (q EntityHistoryQuery) NormalizedLimit() int {
	if q.Limit < 1 {
		return 20
	}
	if q.Limit > 100 {
		return 100
	}
	return q.Limit
}

type SessionResponse struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	RequestID   string `json:"requestId"`
}
