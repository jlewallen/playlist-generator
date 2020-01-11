package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"time"

	"net/http"

	"encoding/json"

	"github.com/zmb3/spotify"
)

type options struct {
	RootPath string
}

type LoadedPlaylist struct {
	Path     string
	Playlist *PlaylistSummary
	Tracks   []spotify.PlaylistTrack
}

type Playlists struct {
	RootPath  string
	LoadedAt  time.Time
	Summaries *PlaylistSummaries
	Playlists []*LoadedPlaylist
}

func NewPlaylists(rootPath string) (pl *Playlists) {
	return &Playlists{
		RootPath:  rootPath,
		Playlists: nil,
	}
}

func (pl *Playlists) Load() error {
	summaries, err := LoadSummaries(filepath.Join(pl.RootPath, "playlists.json"))
	if err != nil {
		return err
	}

	for _, playlist := range summaries.Playlists {
		path := filepath.Join(pl.RootPath, fmt.Sprintf("playlist-%s.json", playlist.ID))

		bytes, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		allTracks := make([]spotify.PlaylistTrack, 0)
		err = json.Unmarshal(bytes, &allTracks)
		if err != nil {
			return err
		}

		log.Printf("path %v %d", path, len(allTracks))

		pl.Playlists = append(pl.Playlists, &LoadedPlaylist{
			Path:     path,
			Playlist: playlist,
			Tracks:   allTracks,
		})
	}

	return nil
}

func (pl *Playlists) LoadIfNecessary() error {
	if pl.Playlists != nil {
		if pl.LoadedAt.Add(6 * time.Hour).After(time.Now()) {
			log.Printf("using cached playlists")
			return nil
		}
	}

	log.Printf("loading")

	err := pl.Load()
	if err != nil {
		return err
	}

	pl.LoadedAt = time.Now()

	return nil
}

type MatchedPlaylist struct {
	ID   spotify.ID `json:"id"`
	Name string     `json:"name"`
}

type SmallTrack struct {
	ID      spotify.ID `json:"id"`
	Name    string     `json:"name"`
	Artists []string   `json:"artists"`
	Album   string     `json:"album"`
}

type MatchedTrack struct {
	Playlist *MatchedPlaylist `json:"playlist"`
	Track    *SmallTrack      `json:"track"`
}

type SearchMatches struct {
	Matches []*MatchedTrack `json:"matches"`
}

func isMatch(track spotify.PlaylistTrack, query string) bool {
	if strings.Contains(strings.ToLower(track.Track.Name), query) {
		return true
	}
	if strings.Contains(strings.ToLower(track.Track.Album.Name), query) {
		return true
	}
	for _, artist := range track.Track.Artists {
		if strings.Contains(strings.ToLower(artist.Name), query) {
			return true
		}
	}
	return false
}

func getArtists(track *spotify.FullTrack) []string {
	names := make([]string, 0)
	for _, a := range track.Artists {
		names = append(names, a.Name)
	}
	return names
}

func (pl *Playlists) Search(query string) (matches *SearchMatches, err error) {
	err = pl.LoadIfNecessary()
	if err != nil {
		return nil, err
	}

	matches = &SearchMatches{
		Matches: make([]*MatchedTrack, 0),
	}

	for _, playlist := range pl.Playlists {
		for _, track := range playlist.Tracks {
			if isMatch(track, query) {
				matches.Matches = append(matches.Matches, &MatchedTrack{
					Playlist: &MatchedPlaylist{
						ID:   playlist.Playlist.ID,
						Name: playlist.Playlist.Name,
					},
					Track: &SmallTrack{
						ID:      track.Track.ID,
						Name:    track.Track.Name,
						Album:   track.Track.Album.Name,
						Artists: getArtists(&track.Track),
					},
				})
			}
		}
	}

	return
}

func main() {
	o := &options{}

	flag.StringVar(&o.RootPath, "path", "", "path")

	flag.Parse()

	pl := NewPlaylists(o.RootPath)

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		log.Printf("[http] %v", req.URL)

		q, ok := req.URL.Query()["q"]
		if !ok || len(q) != 1 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		m, err := pl.Search(strings.ToLower(q[0]))
		if err != nil {
			panic(err)
		}

		bytes, err := json.Marshal(m)
		if err != nil {
			panic(err)
		}

		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	})

	log.Printf("starting...")

	err := http.ListenAndServe(":8090", nil)
	if err != nil {
		panic(err)
	}
}
