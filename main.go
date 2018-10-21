package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strings"

	mapset "github.com/deckarep/golang-set"

	"github.com/zmb3/spotify"
)

type Playlist struct {
	ID   spotify.ID
	User string
	Name string
}

type Track struct {
	ID    spotify.ID
	Title string
}

type PlaylistSet struct {
	Playlists []Playlist
}

func (ps *PlaylistSet) Monthly() (nps *PlaylistSet) {
	monthly := regexp.MustCompile("^(\\d\\d\\d\\d) (january|february|march|april|may|june|july|august|september|october|november|december)$")

	playlists := make([]Playlist, 0)
	for _, pl := range ps.Playlists {
		if monthly.MatchString(pl.Name) {
			playlists = append(playlists, pl)
		}
	}

	return &PlaylistSet{
		Playlists: playlists,
	}
}

func (ps *PlaylistSet) GetAllTracks() (nps *PlaylistSet) {
	return &PlaylistSet{}
}

type SpotifyCacher struct {
	spotifyClient *spotify.Client
}

func GetPlaylistByTitle(spotifyClient *spotify.Client, user, name string) (*spotify.SimplePlaylist, error) {
	limit := 20
	offset := 0
	options := spotify.Options{Limit: &limit, Offset: &offset}
	for {
		playlists, err := spotifyClient.GetPlaylistsForUserOpt(user, &options)
		if err != nil {
			return nil, err
		}

		for _, iter := range playlists.Playlists {
			if strings.EqualFold(iter.Name, name) {
				return &iter, nil
			}
		}

		if len(playlists.Playlists) < *options.Limit {
			break
		}

		offset := *options.Limit + *options.Offset
		options.Offset = &offset
	}

	return nil, nil
}

func (sc *SpotifyCacher) GetPlaylists(user string) (playlists *PlaylistSet, err error) {
	if _, err := os.Stat("playlists.json"); !os.IsNotExist(err) {
		file, err := ioutil.ReadFile("playlists.json")
		if err != nil {
			return nil, err
		}

		playlists = &PlaylistSet{}
		err = json.Unmarshal(file, playlists)
		if err != nil {
			return nil, err
		}

		log.Printf("Returning cached playlists.json")

		return playlists, nil
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
			playlists.Playlists = append(playlists.Playlists, Playlist{
				ID:   iter.ID,
				Name: iter.Name,
				User: user,
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
		return nil, fmt.Errorf("Error saving Playlists.json: %v", err)
	}

	err = ioutil.WriteFile("playlists.json", json, 0644)
	if err != nil {
		return nil, fmt.Errorf("Error saving Playlists.json: %v", err)
	}

	return
}

func (sc *SpotifyCacher) Invalidate(id spotify.ID) {
	cachedFile := fmt.Sprintf("playlist-%s.json", id)
	os.Remove(cachedFile)

	log.Printf("Invalidating playlist %v", id)
}

func (sc *SpotifyCacher) GetPlaylistTracks(userId string, id spotify.ID) (allTracks []spotify.PlaylistTrack, err error) {
	cachedFile := fmt.Sprintf("playlist-%s.json", id)
	if _, err := os.Stat(cachedFile); !os.IsNotExist(err) {
		file, err := ioutil.ReadFile(cachedFile)
		if err != nil {
			return nil, fmt.Errorf("Error opening %v", err)
		}

		allTracks = make([]spotify.PlaylistTrack, 0)
		err = json.Unmarshal(file, &allTracks)
		if err != nil {
			return nil, fmt.Errorf("Error unmarshalling %v", err)
		}

		log.Printf("Returning cached %s", cachedFile)

		return allTracks, nil
	}

	limit := 100
	offset := 0
	options := spotify.Options{Limit: &limit, Offset: &offset}
	for {
		tracks, spotifyErr := sc.spotifyClient.GetPlaylistTracksOpt(userId, id, &options, "")
		if spotifyErr != nil {
			err = spotifyErr
			return
		}

		allTracks = append(allTracks, tracks.Tracks...)

		if len(tracks.Tracks) < *options.Limit {
			break
		}

		offset := *options.Limit + *options.Offset
		options.Offset = &offset
	}

	json, err := json.Marshal(allTracks)
	if err != nil {
		return nil, fmt.Errorf("Error saving playlist tracks: %v", err)
	}

	err = ioutil.WriteFile(cachedFile, json, 0644)
	if err != nil {
		return nil, fmt.Errorf("Error saving playlist tracks: %v", err)
	}

	return
}

func GetTrackIds(tracks []spotify.FullTrack) (ids []spotify.ID) {
	for _, track := range tracks {
		ids = append(ids, track.ID)
	}

	return
}

func ToSpotifyIds(ids []interface{}) (ifaces []spotify.ID) {
	for _, id := range ids {
		ifaces = append(ifaces, id.(spotify.ID))
	}
	return
}

func MapIds(ids []spotify.ID) (ifaces []interface{}) {
	for _, id := range ids {
		ifaces = append(ifaces, id)
	}
	return
}

type PlaylistUpdate struct {
	idsBefore mapset.Set
	idsAfter  []spotify.ID
}

func NewPlaylistUpdate(idsBefore []spotify.ID) *PlaylistUpdate {
	return &PlaylistUpdate{
		idsBefore: mapset.NewSetFromSlice(MapIds(idsBefore)),
		idsAfter:  make([]spotify.ID, 0),
	}
}

func (pu *PlaylistUpdate) AddTrack(id spotify.ID) {
	pu.idsAfter = append(pu.idsAfter, id)
}

func (pu *PlaylistUpdate) GetIdsToRemove() []spotify.ID {
	afterSet := mapset.NewSetFromSlice(MapIds(pu.idsAfter))
	idsToRemove := pu.idsBefore.Difference(afterSet)
	return ToSpotifyIds(idsToRemove.ToSlice())
}

func (pu *PlaylistUpdate) GetIdsToAdd() []spotify.ID {
	ids := make([]spotify.ID, 0)
	for _, id := range pu.idsAfter {
		if !pu.idsBefore.Contains(id) {
			ids = append(ids, id)
		}
	}
	return ids
}

func (pu *PlaylistUpdate) MergeBeforeAndToAdd() {
	for _, id := range pu.idsAfter {
		pu.idsBefore.Add(id)
	}
}

func getPlaylist(spotifyClient *spotify.Client, user string, name string) (pl *spotify.SimplePlaylist, err error) {
	pl, err = GetPlaylistByTitle(spotifyClient, user, name)
	if err != nil {
		return nil, fmt.Errorf("Error getting %s: %v", name, err)
	}
	if pl == nil {
		created, err := spotifyClient.CreatePlaylistForUser(user, name, true)
		if err != nil {
			return nil, fmt.Errorf("Unable to create playlist: %v", err)
		}

		log.Printf("Created destination: %v", created)

		pl, err = GetPlaylistByTitle(spotifyClient, user, name)
		if err != nil {
			return nil, fmt.Errorf("Error getting %s: %v", name, err)
		}
	}

	return pl, nil
}

type Options struct {
	User string
	Name string
	Size int
}

func main() {
	var options Options

	flag.StringVar(&options.User, "user", "jlewalle", "user")
	flag.StringVar(&options.Name, "name", "discovery monthly", "name")
	flag.IntVar(&options.Size, "size", 30, "size")

	flag.Parse()

	logFile, err := os.OpenFile("generator.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	defer logFile.Close()
	buffer := new(bytes.Buffer)
	multi := io.MultiWriter(logFile, buffer, os.Stdout)
	log.SetOutput(multi)

	spotifyClient, _ := AuthenticateSpotify()
	cacher := SpotifyCacher{
		spotifyClient: spotifyClient,
	}

	pl, err := getPlaylist(spotifyClient, options.User, options.Name)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}

	cacher.Invalidate(pl.ID)

	existingTracks, err := cacher.GetPlaylistTracks(options.User, pl.ID)
	if err != nil {
		log.Fatalf("%v", err)
	}

	log.Printf("Have %v (%v tracks)", pl, len(existingTracks))

	playlists, err := cacher.GetPlaylists(options.User)
	if err != nil {
		log.Fatalf("%v", err)
	}

	allTracks := NewEmptyTracksSet()

	for _, pl := range playlists.Monthly().Playlists {
		tracks, err := cacher.GetPlaylistTracks(options.User, pl.ID)
		if err != nil {
			log.Fatalf("%v", err)
		}

		log.Printf("Monthly: %v (%d tracks)", pl, len(tracks))

		allTracks = allTracks.MergeInPlace(tracks)
	}

	log.Printf("Total tracks: %v", len(allTracks.Ids))

	existing := NewTracksSetFromPlaylist(existingTracks)
	sampling := allTracks.Remove(existing)

	log.Printf("Sampling tracks: %v", len(sampling.Ids))

	selected := sampling.Sample(options.Size)

	log.Printf("Removing old tracks: %v", len(existing.Ids))

	err = removeTracksSetFromPlaylist(spotifyClient, options.User, pl.ID, existing)
	if err != nil {
		log.Fatalf("%v", err)
	}

	log.Printf("Adding new tracks: %v", len(selected.Ids))

	err = addTracksSetToPlaylist(spotifyClient, options.User, pl.ID, selected)
	if err != nil {
		log.Fatalf("%v", err)
	}
}

type TracksSet struct {
	Ids map[spotify.ID]bool
}

func NewEmptyTracksSet() (ts *TracksSet) {
	ids := make(map[spotify.ID]bool)

	return &TracksSet{
		Ids: ids,
	}
}

func NewTracksSetFromPlaylist(tracks []spotify.PlaylistTrack) (ts *TracksSet) {
	ids := make(map[spotify.ID]bool)

	for _, t := range tracks {
		ids[t.Track.ID] = true
	}

	return &TracksSet{
		Ids: ids,
	}
}

func (ts *TracksSet) MergeInPlace(tracks []spotify.PlaylistTrack) (ns *TracksSet) {
	for _, t := range tracks {
		ts.Ids[t.Track.ID] = true
	}

	return ts
}

func (ts *TracksSet) Remove(removing *TracksSet) (ns *TracksSet) {
	ids := make(map[spotify.ID]bool)

	for k, _ := range ts.Ids {
		if _, ok := removing.Ids[k]; !ok {
			ids[k] = true
		}
	}

	return &TracksSet{
		Ids: ids,
	}
}

func (ts *TracksSet) ToArray() []spotify.ID {
	array := make([]spotify.ID, 0)
	for id, _ := range ts.Ids {
		array = append(array, id)
	}
	return array
}

func (ts *TracksSet) Sample(number int) (ns *TracksSet) {
	ids := make(map[spotify.ID]bool)

	if len(ts.Ids) < number {
		panic("Not enough tracks to sample from")
	}

	array := ts.ToArray()

	for len(ids) < number {
		i := rand.Uint32() % uint32(len(ts.Ids))
		id := array[i]

		if _, ok := ids[id]; !ok {
			ids[id] = true
		}
	}

	return &TracksSet{
		Ids: ids,
	}
}

func removeTracksSetFromPlaylist(spotifyClient *spotify.Client, user string, id spotify.ID, ts *TracksSet) (err error) {
	removals := ts.ToArray()

	for i := 0; i < len(removals); i += 50 {
		batch := removals[i:min(i+50, len(removals))]
		_, err := spotifyClient.RemoveTracksFromPlaylist(user, id, batch...)
		if err != nil {
			return fmt.Errorf("Error removing tracks: %v", err)
		}
	}

	return nil
}

func addTracksSetToPlaylist(spotifyClient *spotify.Client, user string, id spotify.ID, ts *TracksSet) (err error) {
	additions := ts.ToArray()

	for i := 0; i < len(additions); i += 50 {
		batch := additions[i:min(i+50, len(additions))]
		_, err := spotifyClient.AddTracksToPlaylist(user, id, batch...)
		if err != nil {
			return fmt.Errorf("Error adding tracks: %v", err)
		}
	}

	return nil
}

func min(a, b int) int {
	if a <= b {
		return a
	}
	return b
}
