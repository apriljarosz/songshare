package services

import (
	"net/url"
	"strings"
)

// resolveArtworkURL normalizes TIDAL artwork template URLs by substituting a size
// and ensuring they are usable as absolute HTTPS URLs.
func resolveArtworkURL(raw string) string {
	if raw == "" {
		return ""
	}
	u := raw
	// Replace common size templates
	replacements := []string{"{width}x{height}", "{w}x{h}", "{size}"}
	for _, ph := range replacements {
		if strings.Contains(u, ph) {
			u = strings.ReplaceAll(u, ph, "640x640")
		}
	}
	// Ensure scheme
	if strings.HasPrefix(u, "//") {
		u = "https:" + u
	} else if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		// If it looks like a bare host/path, try to make it absolute
		if _, err := url.Parse(u); err == nil {
			u = "https://" + u
		}
	}
	return u
}

// coverIDToURL constructs a standard artwork URL from a TIDAL cover ID/hash
// Example: resources.tidal.com/images/<cover-id>/640x640.jpg
func coverIDToURL(cover string) string {
	if cover == "" {
		return ""
	}
	// Some cover IDs may contain slashes already; just join onto base
	return "https://resources.tidal.com/images/" + strings.TrimLeft(cover, "/") + "/640x640.jpg"
}
