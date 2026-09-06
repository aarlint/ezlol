package api

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aarlint/ezlol/internal/settings"
)

var validPlatform = regexp.MustCompile(`^[a-z]{2,4}[0-9]?$`)

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	m := s.settings.Get().Masked()
	m["ocrAvailable"] = s.ocr.Available()
	m["version"] = s.version
	writeJSON(w, 200, m)
}

// putSettings applies a partial update. The Riot key is accepted as
// "riotApiKey" (empty string clears it) and is never echoed back or logged.
func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AutoAccept    *bool   `json:"autoAccept"`
		RiotAPIKey    *string `json:"riotApiKey"`
		Platform      *string `json:"platform"`
		MatchesPerRun *int    `json:"matchesPerRun"`
		AutoCompile   *bool   `json:"autoCompile"`
		OCR           *bool   `json:"ocr"`
		UpdateCheck   *bool   `json:"updateCheck"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	if body.Platform != nil && !validPlatform.MatchString(*body.Platform) {
		writeErr(w, 400, "platform must look like na1, euw1, kr")
		return
	}
	if body.MatchesPerRun != nil && (*body.MatchesPerRun < 10 || *body.MatchesPerRun > 5000) {
		writeErr(w, 400, "matchesPerRun must be 10-5000")
		return
	}
	if body.RiotAPIKey != nil {
		k := strings.TrimSpace(*body.RiotAPIKey)
		if k != "" && (len(k) < 20 || strings.ContainsAny(k, " \n\t")) {
			writeErr(w, 400, "that does not look like a Riot API key")
			return
		}
		body.RiotAPIKey = &k
	}
	err := s.settings.Update(func(st *settings.Settings) {
		if body.AutoAccept != nil {
			st.AutoAccept = *body.AutoAccept
		}
		if body.RiotAPIKey != nil {
			st.RiotAPIKey = *body.RiotAPIKey
		}
		if body.Platform != nil {
			st.Platform = *body.Platform
		}
		if body.MatchesPerRun != nil {
			st.MatchesPerRun = *body.MatchesPerRun
		}
		if body.AutoCompile != nil {
			st.AutoCompile = *body.AutoCompile
		}
		if body.OCR != nil {
			st.OCR = *body.OCR
		}
		if body.UpdateCheck != nil {
			st.UpdateCheck = *body.UpdateCheck
		}
	})
	if err != nil {
		writeErr(w, 500, "could not save settings: "+err.Error())
		return
	}
	cur := s.settings.Get()
	s.compiler.Configure(cur.RiotAPIKey, cur.Platform, cur.MatchesPerRun)
	if body.AutoAccept != nil && s.watcher.Status().AutoAccept != *body.AutoAccept {
		s.watcher.SetAutoAccept(*body.AutoAccept)
	}
	s.log.Info("settings updated", "keySet", cur.RiotAPIKey != "", "platform", cur.Platform, "ocr", cur.OCR)
	s.getSettings(w, r)
}

func (s *Server) getUpdate(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.update.Info())
}

func (s *Server) checkUpdate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	writeJSON(w, 200, s.update.Check(ctx))
}
