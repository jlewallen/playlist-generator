package main

import (
	_ "log"
	"time"

	"github.com/zmb3/spotify"
)

type PlaylistSummary struct {
	ID             spotify.ID     `json:"id"`
	User           string         `json:"user"`
	Name           string         `json:"name"`
	Owner          string         `json:"owner"`
	Images         []SpotifyImage `json:"images"`
	Description    string         `json:"description"`
	NumberOfTracks uint32         `json:"numberOfTracks"`
	LastModified   time.Time      `json:"lastModified"`
}

type PlaylistSummaries struct {
	Playlists []*PlaylistSummary `json:"playlists"`
}

func Summarize(pl Playlist, tracks []spotify.PlaylistTrack) (ps *PlaylistSummary, err error) {
	ps = &PlaylistSummary{
		ID:             pl.ID,
		Name:           pl.Name,
		User:           pl.User,
		Owner:          pl.Owner,
		Images:         pl.Images,
		NumberOfTracks: uint32(len(tracks)),
	}

	for _, track := range tracks {
		addedAt, err := time.Parse("2006-01-02T15:04:05Z", track.AddedAt)
		if err != nil {
			return nil, err
		}

		if ps.LastModified.Before(addedAt) {
			ps.LastModified = addedAt
		}
	}

	return
}
