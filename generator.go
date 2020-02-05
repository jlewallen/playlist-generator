package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"encoding/json"
)

type Options struct {
	Dry     bool
	Refresh bool
	Self    string
	User    string
	Name    string
	Size    int
}

func readPlaylistSummaries(file string) (summaries *PlaylistSummaries, err error) {
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		summaries, err = LoadSummaries(file)
		if err != nil {
			return nil, err
		}

		return summaries, nil
	}

	return nil, nil
}

func generateSummary(cacher *SpotifyCacher, user string, playlists *PlaylistSet) error {
	old, err := readPlaylistSummaries("playlists.json")
	if err != nil {
		return err
	}

	summaries := &PlaylistSummaries{
		Playlists: make([]*PlaylistSummary, 0),
	}

	for _, pl := range playlists.Playlists {
		if old != nil {
			for _, oldSummary := range old.Playlists {
				if oldSummary.ID == pl.ID {
					if oldSummary.ID != pl.ID {
						log.Printf("invalidating! (%v != %v)", oldSummary.SnapshotID, pl.SnapshotID)
						cacher.Invalidate(pl.ID)
					}
				}
			}
		}

		tracks, err := cacher.GetPlaylistTracks(user, pl.ID)
		if err != nil {
			return err
		}

		summary, err := Summarize(pl, tracks)
		if err != nil {
			return err
		}

		log.Printf("playlist: %v %v (%d tracks) %v", pl.ID, pl.Name, len(tracks), summary.LastModified)

		summaries.Playlists = append(summaries.Playlists, summary)
	}

	json, err := json.Marshal(summaries)
	if err != nil {
		return fmt.Errorf("error saving playlists: %v", err)
	}

	err = ioutil.WriteFile("playlists.json", json, 0644)
	if err != nil {
		return fmt.Errorf("error saving playlists: %v", err)
	}

	return nil
}

func main() {
	var options Options

	flag.BoolVar(&options.Dry, "dry", false, "dry")
	flag.BoolVar(&options.Refresh, "refresh", false, "refresh")
	flag.StringVar(&options.Self, "self", "jlewalle", "self")
	flag.StringVar(&options.User, "user", "jlewalle", "user")
	flag.StringVar(&options.Name, "name", "discovery monthly", "name")
	flag.IntVar(&options.Size, "size", 30, "size")

	flag.Parse()

	log.Printf("getting playlists for %v, creating playlist for %v", options.User, options.Self)

	logFile, err := os.OpenFile("generator.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer logFile.Close()
	buffer := new(bytes.Buffer)
	multi := io.MultiWriter(logFile, buffer, os.Stdout)
	log.SetOutput(multi)

	spotifyClient, _ := AuthenticateSpotify()
	cacher := &SpotifyCacher{
		spotifyClient: spotifyClient,
		refresh:       options.Refresh,
	}

	pl, err := GetPlaylist(spotifyClient, options.Self, options.Name)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	cacher.Invalidate(pl.ID)

	existingTracks, err := cacher.GetPlaylistTracks(options.User, pl.ID)
	if err != nil {
		log.Fatalf("%v", err)
	}

	log.Printf("have %v (%v tracks)", pl, len(existingTracks))

	cacher.InvalidateUser(options.User)

	playlists, err := cacher.GetPlaylists(options.User)
	if err != nil {
		log.Fatalf("%v", err)
	}

	allTracks := NewEmptyTracksSet()

	err = generateSummary(cacher, options.User, playlists)
	if err != nil {
		log.Fatalf("%v", err)
	}

	for _, pl := range playlists.Monthly().Playlists {
		tracks, err := cacher.GetPlaylistTracks(options.User, pl.ID)
		if err != nil {
			log.Fatalf("%v", err)
		}

		log.Printf("monthly: %v (%d tracks)", pl.Name, len(tracks))

		allTracks = allTracks.MergeInPlace(tracks)
	}

	log.Printf("total tracks: %v", len(allTracks.Ids))

	existing := NewTracksSetFromPlaylist(existingTracks)
	sampling := allTracks.Remove(existing)

	log.Printf("sampling tracks: %v", len(sampling.Ids))

	selected := sampling.Sample(options.Size)

	if !options.Dry {
		log.Printf("removing old tracks: %v", len(existing.Ids))

		err = RemoveTracksSetFromPlaylist(spotifyClient, pl.ID, existing)
		if err != nil {
			log.Fatalf("%v", err)
		}

		log.Printf("adding new tracks: %v", len(selected.Ids))

		err = AddTracksSetToPlaylist(spotifyClient, pl.ID, selected)
		if err != nil {
			log.Fatalf("%v", err)
		}
	} else {
		log.Printf("dry run!")
	}
}
