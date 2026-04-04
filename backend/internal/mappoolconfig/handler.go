package mappoolconfig

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func HandleGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, Get())
	}
}

func HandleSet() gin.HandlerFunc {
	return func(c *gin.Context) {
		var p Pool
		if err := c.ShouldBindJSON(&p); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := Set(p); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, Get())
	}
}
