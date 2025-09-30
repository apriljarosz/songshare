package handlers

import (
	"net/http"

	"github.com/apriljarosz/songshare/internal/models"
	"github.com/gin-gonic/gin"
)

func GetPlatforms() gin.HandlerFunc {
	return func(c *gin.Context) {
		platforms := []models.Platform{
			{ID: "spotify", Name: "Spotify"},
			{ID: "apple_music", Name: "Apple Music"},
		}

		c.JSON(http.StatusOK, gin.H{
			"platforms": platforms,
		})
	}
}
