package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
)

type Handler struct {
	Mgr *Manager
}

func (h *Handler) HandleRunSubmit(c *gin.Context) {
	var p engine.Pipeline

	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
	}

	c.JSON(http.StatusOK, gin.H{})
}
