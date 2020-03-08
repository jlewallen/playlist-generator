package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	_ "time"

	"encoding/json"

	"github.com/zmb3/spotify"
)

const VerboseLogging = false

type SpotifyCacher struct {
	cache         map[string]interface{}
	spotifyClient *spotify.Client
	refresh       bool
}

func NewSpotifyCacher(spotifyClient *spotify.Client, refresh bool) *SpotifyCacher {
	return &SpotifyCacher{
		cache:         make(map[string]interface{}),
		spotifyClient: spotifyClient,
		refresh:       refresh,
	}
}

func (sc *SpotifyCacher) lookup(path string, value interface{}) (interface{}, error) {
	if sc.cache[path] != nil {
		value = sc.cache[path]
		return value, nil
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		file, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("error reading: %v", err)
		}

		err = json.Unmarshal(file, value)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling: %v", err)
		}

		if VerboseLogging {
			log.Printf("returning cached %v %v", path)
		}

		sc.cache[path] = value

		return value, nil
	}

	return nil, nil
}

func (sc *SpotifyCacher) GetPlaylists(user string) (playlists *PlaylistSet, err error) {
	cachedFile := fmt.Sprintf(".cache/playlists-%s.json", user)
	if !sc.refresh {
		playlists = &PlaylistSet{}
		cached, err := sc.lookup(cachedFile, playlists)
		if err != nil {
			return nil, err
		}
		if cached != nil {
			return cached.(*PlaylistSet), nil
		}
	}

	limit := 50
	offset := 0
	options := spotify.Options{Limit: &limit, Offset: &offset}
	playlists = &PlaylistSet{
		Playlists: make([]Playlist, 0),
	}
	for {
		page, err := sc.spotifyClient.GetPlaylistsForUserOpt(user, &options)
		if err != nil {
			return nil, err
		}

		for _, iter := range page.Playlists {
			images := make([]SpotifyImage, 0)
			for _, image := range iter.Images {
				images = append(images, SpotifyImage{
					URL: image.URL,
					Dx:  int32(image.Width),
					Dy:  int32(image.Height),
				})
			}
			playlists.Playlists = append(playlists.Playlists, Playlist{
				ID:         iter.ID,
				Name:       iter.Name,
				User:       user,
				Owner:      iter.Owner.ID,
				Images:     images,
				SnapshotID: iter.SnapshotID,
			})
		}

		if len(page.Playlists) < *options.Limit {
			break
		}

		offset := *options.Limit + *options.Offset
		options.Offset = &offset
	}

	json, err := json.Marshal(playlists)
	if err != nil {
		return nil, fmt.Errorf("error saving Playlists: %v", err)
	}

	err = ioutil.WriteFile(cachedFile, json, 0644)
	if err != nil {
		return nil, fmt.Errorf("error saving Playlists: %v", err)
	}

	return
}

func (sc *SpotifyCacher) InvalidateUser(user string) {
	cachedFile := fmt.Sprintf(".cache/playlists-%s.json", user)
	os.Remove(cachedFile)

	log.Printf("invalidating playlists %v", user)
}

func (sc *SpotifyCacher) Invalidate(id spotify.ID) {
	cachedFile := fmt.Sprintf(".cache/playlist-%s.json", id)
	os.Remove(cachedFile)

	log.Printf("invalidating playlist %v", id)
}

func (sc *SpotifyCacher) GetPlaylistTracks(userId string, id spotify.ID) (allTracks []spotify.PlaylistTrack, err error) {
	cachedFile := fmt.Sprintf(".cache/playlist-%s.json", id)
	if !sc.refresh {
		allTracks = make([]spotify.PlaylistTrack, 0)
		cached, err := sc.lookup(cachedFile, &allTracks)
		if err != nil {
			return nil, err
		}
		if cached != nil {
			return *(cached.(*[]spotify.PlaylistTrack)), nil
		}
	}

	allTracks, spotifyErr := GetPlaylistTracks(sc.spotifyClient, id)
	if spotifyErr != nil {
		err = spotifyErr
		return
	}

	json, err := json.Marshal(allTracks)
	if err != nil {
		return nil, fmt.Errorf("error saving playlist tracks: %v", err)
	}

	err = ioutil.WriteFile(cachedFile, json, 0644)
	if err != nil {
		return nil, fmt.Errorf("error saving playlist tracks: %v", err)
	}

	return
}

func (sc *SpotifyCacher) GetAlbum(id spotify.ID) (album *spotify.FullAlbum, err error) {
	cachedFile := fmt.Sprintf(".cache/album-%s.json", id)
	if _, err := os.Stat(cachedFile); !os.IsNotExist(err) {
		var album *spotify.FullAlbum
		cached, err := sc.lookup(cachedFile, album)
		if err != nil {
			return nil, err
		}
		if cached != nil {
			return cached.(*spotify.FullAlbum), nil
		}
	}

	album, spotifyErr := sc.spotifyClient.GetAlbum(id)
	if spotifyErr != nil {
		err = spotifyErr
		return
	}

	json, err := json.Marshal(album)
	if err != nil {
		return nil, fmt.Errorf("error saving album tracks: %v", err)
	}

	err = ioutil.WriteFile(cachedFile, json, 0644)
	if err != nil {
		return nil, fmt.Errorf("error saving album tracks: %v", err)
	}

	return
}

func (sc *SpotifyCacher) GetAlbumTracks(id spotify.ID) (allTracks []spotify.SimpleTrack, err error) {
	cachedFile := fmt.Sprintf(".cache/album-tracks-%s.json", id)
	if _, err := os.Stat(cachedFile); !os.IsNotExist(err) {
		allTracks := make([]spotify.SimpleTrack, 0)
		cached, err := sc.lookup(cachedFile, &allTracks)
		if err != nil {
			return nil, err
		}
		if cached != nil {
			return *(cached.(*[]spotify.SimpleTrack)), nil
		}
	}

	allTracks, spotifyErr := GetAlbumTracks(sc.spotifyClient, id)
	if spotifyErr != nil {
		err = spotifyErr
		return
	}

	json, err := json.Marshal(allTracks)
	if err != nil {
		return nil, fmt.Errorf("error saving album tracks: %v", err)
	}

	err = ioutil.WriteFile(cachedFile, json, 0644)
	if err != nil {
		return nil, fmt.Errorf("error saving album tracks: %v", err)
	}

	return
}

func (sc *SpotifyCacher) GetArtistAlbums(id spotify.ID) (allAlbums []spotify.SimpleAlbum, err error) {
	cachedFile := fmt.Sprintf(".cache/artist-albums-%s.json", id)
	if _, err := os.Stat(cachedFile); !os.IsNotExist(err) {
		allAlbums := make([]spotify.SimpleAlbum, 0)
		cached, err := sc.lookup(cachedFile, &allAlbums)
		if err != nil {
			return nil, err
		}
		if cached != nil {
			return *(cached.(*[]spotify.SimpleAlbum)), nil
		}
	}

	allAlbums, spotifyErr := GetArtistAlbums(sc.spotifyClient, id)
	if spotifyErr != nil {
		err = spotifyErr
		return
	}

	json, err := json.Marshal(allAlbums)
	if err != nil {
		return nil, fmt.Errorf("error saving artist albums: %v", err)
	}

	err = ioutil.WriteFile(cachedFile, json, 0644)
	if err != nil {
		return nil, fmt.Errorf("error saving artist albums: %v", err)
	}

	return
}

func (sc *SpotifyCacher) GetTracks(ids []spotify.ID) (tracks []spotify.FullTrack, err error) {
	requesting := make([]spotify.ID, 0)
	for _, id := range ids {
		cachedFile := fmt.Sprintf(".cache/track-%s.json", id)
		if _, err := os.Stat(cachedFile); os.IsNotExist(err) {
			requesting = append(requesting, id)
		}
	}

	if len(requesting) > 0 {
		requested, spotifyErr := sc.spotifyClient.GetTracks(requesting...)
		if spotifyErr != nil {
			err = spotifyErr
			return
		}

		for _, track := range requested {
			cachedFile := fmt.Sprintf(".cache/track-%s.json", track.ID)

			json, err := json.Marshal(track)
			if err != nil {
				return nil, fmt.Errorf("error saving track: %v", err)
			}

			err = ioutil.WriteFile(cachedFile, json, 0644)
			if err != nil {
				return nil, fmt.Errorf("error saving track: %v", err)
			}
		}
	}

	tracks = make([]spotify.FullTrack, 0)

	for _, id := range ids {
		cachedFile := fmt.Sprintf(".cache/track-%s.json", id)
		if _, err := os.Stat(cachedFile); !os.IsNotExist(err) {
			file, err := ioutil.ReadFile(cachedFile)
			if err != nil {
				return nil, fmt.Errorf("error opening %v", err)
			}

			var track spotify.FullTrack
			err = json.Unmarshal(file, &track)
			if err != nil {
				return nil, fmt.Errorf("error unmarshalling %v", err)
			}

			tracks = append(tracks, track)

			if VerboseLogging {
				log.Printf("returning cached %s", cachedFile)
			}
		}
	}

	return
}
