package matchconfig

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ProfileResponse embeds Profile with all phase CFGs included.
type ProfileResponse struct {
	*Profile
	CFGs map[string]string `json:"cfgs"`
}

func buildResponse(p *Profile) ProfileResponse {
	cfgs := make(map[string]string, len(Phases))
	for _, phase := range Phases {
		content, _ := GetCFG(p.ID, phase)
		cfgs[phase] = content
	}
	return ProfileResponse{Profile: p, CFGs: cfgs}
}

func HandleList() gin.HandlerFunc {
	return func(c *gin.Context) {
		list := List()
		resp := make([]ProfileResponse, len(list))
		for i, p := range list {
			resp[i] = buildResponse(p)
		}
		c.JSON(http.StatusOK, resp)
	}
}

func HandleCreate() gin.HandlerFunc {
	return func(c *gin.Context) {
		var p Profile
		if err := c.ShouldBindJSON(&p); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := Create(&p); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, buildResponse(&p))
	}
}

func HandleGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := Get(c.Param("id"))
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
			return
		}
		c.JSON(http.StatusOK, buildResponse(p))
	}
}

func HandleUpdate() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var update Profile
		if err := c.ShouldBindJSON(&update); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := Update(id, &update); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
			return
		}
		p, _ := Get(id)
		c.JSON(http.StatusOK, buildResponse(p))
	}
}

func HandleDelete() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := Delete(c.Param("id")); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func HandleGetCFG() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, phase := c.Param("id"), c.Param("phase")
		if _, ok := Get(id); !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
			return
		}
		content, err := GetCFG(id, phase)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"content": content})
	}
}

func HandleSetCFG() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, phase := c.Param("id"), c.Param("phase")
		if _, ok := Get(id); !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
			return
		}
		var body struct {
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := SetCFG(id, phase, body.Content); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func HandleGetServerInitCFG() gin.HandlerFunc {
	return func(c *gin.Context) {
		content, err := GetServerInitCFG()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"content": content})
	}
}

func HandleSetServerInitCFG() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := SetServerInitCFG(body.Content); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}
