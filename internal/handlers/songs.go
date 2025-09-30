package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/apriljarosz/songshare/internal/repository"
	"github.com/gin-gonic/gin"
)

type SongHandler struct {
	songRepo *repository.SongRepository
}

func NewSongHandler(songRepo *repository.SongRepository) *SongHandler {
	return &SongHandler{
		songRepo: songRepo,
	}
}

func (h *SongHandler) GetSong(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	song, err := h.songRepo.FindByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get song", "details": err.Error()})
		return
	}

	if song == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "song not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"song": song})
}

// UniversalLink redirects or displays song from universal link
func (h *SongHandler) UniversalLink(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	song, err := h.songRepo.FindByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get song", "details": err.Error()})
		return
	}

	if song == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "song not found"})
		return
	}

	// For now, return JSON. Later we can add HTML page or redirect logic
	c.JSON(http.StatusOK, gin.H{"song": song})
}
