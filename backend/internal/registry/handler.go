package registry

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func HandleList() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, List())
	}
}
