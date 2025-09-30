package services

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type ParsedURL struct {
	Platform string
	ID       string
}

var (
	spotifyTrackRegex = regexp.MustCompile(`spotify\.com/track/([a-zA-Z0-9]+)`)
	spotifyOpenRegex  = regexp.MustCompile(`open\.spotify\.com/track/([a-zA-Z0-9]+)`)
)

func ParsePlatformURL(urlStr string) (*ParsedURL, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	host := strings.ToLower(parsedURL.Host)

	// Spotify
	if strings.Contains(host, "spotify.com") {
		// Try standard spotify.com
		if match := spotifyTrackRegex.FindStringSubmatch(urlStr); len(match) > 1 {
			return &ParsedURL{
				Platform: "spotify",
				ID:       match[1],
			}, nil
		}
		// Try open.spotify.com
		if match := spotifyOpenRegex.FindStringSubmatch(urlStr); len(match) > 1 {
			return &ParsedURL{
				Platform: "spotify",
				ID:       match[1],
			}, nil
		}
		return nil, fmt.Errorf("could not extract Spotify track ID from URL")
	}

	return nil, fmt.Errorf("unsupported platform: %s", host)
}
