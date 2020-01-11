package main

import (
	_ "log"
	"time"

	"github.com/zmb3/spotify"
)

type PlaylistSummary struct {
	ID             spotify.ID     `json:"id"`
	Name           string         `json:"name"`
	User           PlaylistUser   `json:"user"`
	Owner          PlaylistUser   `json:"owner"`
	Images         []SpotifyImage `json:"images"`
	Description    string         `json:"description"`
	NumberOfTracks uint32         `json:"numberOfTracks"`
	LastModified   time.Time      `json:"lastModified"`
	Subscribed     bool           `json:"subscribed"`
	SnapshotID     string         `json:"snapshot"`
}

type PlaylistUser struct {
	ID string `json:"id"`
}

type PlaylistSummaries struct {
	Playlists []*PlaylistSummary `json:"playlists"`
}

func Summarize(pl Playlist, tracks []spotify.PlaylistTrack) (ps *PlaylistSummary, err error) {
	ps = &PlaylistSummary{
		ID:             pl.ID,
		Name:           pl.Name,
		Subscribed:     pl.Owner != pl.User,
		Images:         pl.Images,
		SnapshotID:     pl.SnapshotID,
		NumberOfTracks: uint32(len(tracks)),
		User: PlaylistUser{
			ID: pl.User,
		},
		Owner: PlaylistUser{
			ID: pl.Owner,
		},
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
