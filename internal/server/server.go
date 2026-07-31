package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Status int

const (
	StatusRunning = iota
	StatusFailed
	StatusSucceeded
)

func NewServer() *gin.Engine {
	r := gin.Default()

	api := r.Group("/api/v1")
	api.GET("/health", HandleHealth)

	api.POST("/runs")
	api.GET("/runs/:id")

	return r
}

func HandleHealth(c *gin.Context) {
	c.String(http.StatusOK, "health ok")
}
