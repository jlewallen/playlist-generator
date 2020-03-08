package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/zmb3/spotify"

	"github.com/gorilla/mux"
)

func sendFile(w http.ResponseWriter, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = io.Copy(w, f)

	return err
}

type Services struct {
	spotify *SpotifyCacher
	user    string
}

func getPlaylists(ctx context.Context, s *Services, w http.ResponseWriter, r *http.Request) error {
	return sendFile(w, ".cache/playlists.json")
}

func getPlaylist(ctx context.Context, s *Services, w http.ResponseWriter, r *http.Request) error {
	playlistId := mux.Vars(r)["id"]
	return sendFile(w, fmt.Sprintf(".cache/playlist-%s.json", playlistId))
}

type PlaylistIDAndName struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Album struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SearchTrack struct {
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	Album     Album                `json:"album"`
	Artists   []*Artist            `json:"artists"`
	Playlists []*PlaylistIDAndName `json:"playlists"`
}

type Search struct {
	Tracks []*SearchTrack `json:"tracks"`
}

func toArtists(t spotify.FullTrack) []*Artist {
	artists := make([]*Artist, 0)

	for _, a := range t.Artists {
		artists = append(artists, &Artist{
			ID:   a.ID.String(),
			Name: a.Name,
		})
	}

	return artists
}

func searchPlaylists(ctx context.Context, s *Services, w http.ResponseWriter, r *http.Request) error {
	started := time.Now()

	allQ := r.URL.Query()["q"]
	if len(allQ) == 0 {
		return fmt.Errorf("query missing")
	}

	q := strings.ToLower(allQ[0])
	if len(q) == 0 {
		return fmt.Errorf("query empty")
	}

	tracksByID := make(map[string]*SearchTrack)
	tracks := make([]*SearchTrack, 0)

	playlists, err := s.spotify.GetPlaylists(s.user)
	if err != nil {
		return err
	}

	for _, pl := range playlists.Playlists {
		playlistTracks, err := s.spotify.GetPlaylistTracks(s.user, pl.ID)
		if err != nil {
			return err
		}
		for _, track := range playlistTracks {
			if strings.Contains(strings.ToLower(track.Track.Name), q) {
				id := track.Track.ID.String()

				if tracksByID[id] == nil {
					tracksByID[id] = &SearchTrack{
						ID:   id,
						Name: track.Track.Name,
						Album: Album{
							ID:   track.Track.Album.ID.String(),
							Name: track.Track.Album.Name,
						},
						Artists:   toArtists(track.Track),
						Playlists: make([]*PlaylistIDAndName, 0),
					}
					tracks = append(tracks, tracksByID[id])
				}

				st := tracksByID[id]

				st.Playlists = append(st.Playlists, &PlaylistIDAndName{
					ID:   pl.ID.String(),
					Name: pl.Name,
				})
			}
		}
	}

	elapsed := time.Now().Sub(started)

	search := &Search{
		Tracks: tracks,
	}

	data, err := json.Marshal(search)
	if err != nil {
		return err
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)

	log.Printf("done %vms q = '%s'", elapsed, q)

	return err
}

func middleware(services *Services, h func(context.Context, *Services, http.ResponseWriter, *http.Request) error) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		err := h(ctx, services, w, r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, fmt.Sprintf("error: %v", err))
		}
	}
}

func Serve(options *Options) error {
	spotifyClient, _ := AuthenticateSpotify()
	cacher := &SpotifyCacher{
		spotifyClient: spotifyClient,
	}

	services := &Services{
		spotify: cacher,
		user:    options.User,
	}

	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/playlists", middleware(services, getPlaylists)).Methods("GET")
	router.HandleFunc("/playlists/{id}", middleware(services, getPlaylist)).Methods("GET")
	router.HandleFunc("/search", middleware(services, searchPlaylists)).Methods("GET")

	log.Printf("listening on :8080")

	if err := http.ListenAndServe(":8080", router); err != nil {
		return err
	}

	return nil
}
