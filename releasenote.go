package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"time"
)

type ReleaseNote struct {
	PublishDate time.Time `json:"publishedAt"`
	TagName     string    `json:"tagName"`
	Url         string    `json:"url"`
}

const (
	gh            = "gh"
	releaseFields = "publishedAt,tagName,url"
)

var (
	githubRepoUrl = regexp.MustCompile(`^https://github.com/([^/\s]+)/([^/\s]+)/?$`)
	githubPageUrl = regexp.MustCompile(`^https://([^.\s]+).github.io/([^/\s]+)/?$`)
)

func (pkg *Package) GetReleaseNote() *ReleaseNote {
	if !isGhInstalled() {
		return nil
	}

	if matches := githubRepoUrl.FindStringSubmatch(pkg.Homepage); len(matches) > 0 {
		return fetchLatestRelease(matches[1], matches[2])
	} else if matches := githubPageUrl.FindStringSubmatch(pkg.Homepage); len(matches) > 0 {
		return fetchLatestRelease(matches[1], matches[2])
	} else {
		// TODO: add repo look up on github
		// TODO: scrap release note from non-github
		return nil
	}
}

func isGhInstalled() bool {
	if _, err := exec.LookPath(gh); err == nil {
		return true
	} else {
		return false
	}
}

func fetchLatestRelease(ghOwner, ghRepo string) *ReleaseNote {
	var note ReleaseNote
	cmd := exec.Command(gh, "release", "view", "--repo", fmt.Sprintf("%s/%s", ghOwner, ghRepo), "--json", releaseFields)

	body, err := cmd.Output()
	if err != nil {
		log.Printf("Failed to get release info for %s/%s: %v", ghOwner, ghRepo, err)
		return nil
	}

	if err := json.Unmarshal(body, &note); err != nil {
		log.Printf("Failed to decode json from 'gh release view' response %s: %v", body, err)
		return nil
	} else {
		log.Printf("Successfully fetched release info from gh: %s/%s", ghOwner, ghRepo)
		return &note
	}
}
