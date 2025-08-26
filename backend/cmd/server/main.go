package main

import (
	"net/http"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/analysis"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.POST("/analyze", func(c *gin.Context) {
		var req struct {
			Input string `json:"input"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		res := analysis.AnalyzeInput(req.Input)
		c.JSON(http.StatusOK, res)
	})

	r.Run()
}
