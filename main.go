package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Mattermost Release Notes Extractor
//
// This tool helps extract release notes from GitHub pull requests in Mattermost repositories.
// It retrieves PRs with the "release-note" label from selected milestones and displays their release notes.
//
// Usage:
//   ./release-notes-extractor [--token=YOUR_GITHUB_TOKEN]
//
// Token can be provided in three ways (in order of precedence):
//   1. Command line flag: --token=YOUR_TOKEN
//   2. Environment variable: export GITHUB_TOKEN=YOUR_TOKEN
//   3. Default token defined in the code (not recommended)

// GitHub API structures
type Milestone struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description"`
	RepoURL     string `json:"-"` // Internal field, not from API
}

// unifyMilestonesByName combines milestones with the same title/name across repositories
func unifyMilestonesByName(milestoneSets ...[]Milestone) []UnifiedMilestone {
	// Map to hold milestones by title
	milestoneMap := make(map[string]*UnifiedMilestone)
	
	// Process all milestone sets
	for _, milestoneSet := range milestoneSets {
		for _, milestone := range milestoneSet {
			if existing, ok := milestoneMap[milestone.Title]; ok {
				// Add to existing unified milestone
				existing.Milestones = append(existing.Milestones, milestone)
			} else {
				// Create new unified milestone
				milestoneMap[milestone.Title] = &UnifiedMilestone{
					Title:       milestone.Title,
					Description: milestone.Description,
					Milestones:  []Milestone{milestone},
				}
			}
		}
	}
	
	// Convert map to slice
	result := make([]UnifiedMilestone, 0, len(milestoneMap))
	for _, unified := range milestoneMap {
		result = append(result, *unified)
	}
	
	return result
}

// UnifiedMilestone represents a milestone that may exist in multiple repositories
type UnifiedMilestone struct {
	Title       string      // Common name/title
	Description string      // Description (from the first found milestone)
	Milestones  []Milestone // Actual milestones from different repos
}

type PullRequest struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Milestone *struct {
		Number int `json:"number"`
	} `json:"milestone"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// URLs for Mattermost repositories
const (
	mattermostRepoURL = "https://api.github.com/repos/mattermost/mattermost"
	enterpriseRepoURL = "https://api.github.com/repos/mattermost/enterprise"
	mobileRepoURL     = "https://api.github.com/repos/mattermost/mattermost-mobile"
	desktopRepoURL    = "https://api.github.com/repos/mattermost/mattermost-desktop"
	defaultAuthToken  = "" // Default token, lowest priority
)

var authToken string

// max returns the larger of x or y
func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// getGitHubToken returns the GitHub API token from available sources in order of precedence:
// 1. Command-line flag
// 2. Environment variable
// 3. Default token defined in the code
func getGitHubToken() string {
	var flagToken string
	flag.StringVar(&flagToken, "token", "", "GitHub API token")
	flag.Parse()

	// Check sources in order of precedence
	if flagToken != "" {
		return flagToken
	}

	if envToken := os.Getenv("GITHUB_TOKEN"); envToken != "" {
		return envToken
	}

	return defaultAuthToken
}

func main() {
	// Get GitHub token from available sources
	authToken = getGitHubToken()
	
	if authToken == "" {
		fmt.Println("Warning: No GitHub token found. Access to private repositories will fail.")
	} else {
		tokenLength := len(authToken)
		fmt.Printf("Using GitHub token (last 4 chars: %s)\n", 
			authToken[max(0, tokenLength-4):tokenLength])
	}
	// Select repository
	fmt.Println("Select a repository:")
	fmt.Println("1: mattermost/mattermost")
	fmt.Println("2: mattermost/enterprise")
	fmt.Println("3: mattermost/mattermost-mobile")
	fmt.Println("4: mattermost/mattermost-desktop")
	fmt.Println("5: All repositories")

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nSelect an option (1-5): ")
	repoInput, _ := reader.ReadString('\n')
	repoInput = strings.TrimSpace(repoInput)

	repoChoice, err := strconv.Atoi(repoInput)
	if err != nil || repoChoice < 1 || repoChoice > 5 {
		fmt.Println("Invalid selection")
		return
	}

	var repoURL string
	var repoName string
	var milestones []Milestone

	switch repoChoice {
	case 1:
		repoURL = mattermostRepoURL
		repoName = "mattermost/mattermost"
		milestones, err = getMilestones(repoURL)
	case 2:
		repoURL = enterpriseRepoURL
		repoName = "mattermost/enterprise"
		milestones, err = getMilestones(repoURL)
	case 3:
		repoURL = mobileRepoURL
		repoName = "mattermost/mattermost-mobile"
		milestones, err = getMilestones(repoURL)
	case 4:
		repoURL = desktopRepoURL
		repoName = "mattermost/mattermost-desktop"
		milestones, err = getMilestones(repoURL)
	case 5:
		// Get milestones from all repositories and combine them
		mmMilestones, err1 := getMilestones(mattermostRepoURL)
		if err1 != nil {
			fmt.Printf("Error getting milestones from mattermost/mattermost: %v\n", err1)
			return
		}
		// Add repo URL to each milestone
		for i := range mmMilestones {
			mmMilestones[i].RepoURL = mattermostRepoURL
		}

		entMilestones, err2 := getMilestones(enterpriseRepoURL)
		if err2 != nil {
			fmt.Printf("Error getting milestones from mattermost/enterprise: %v\n", err2)
			return
		}
		// Add repo URL to each milestone
		for i := range entMilestones {
			entMilestones[i].RepoURL = enterpriseRepoURL
		}
		
		mobileMilestones, err3 := getMilestones(mobileRepoURL)
		if err3 != nil {
			fmt.Printf("Error getting milestones from mattermost/mattermost-mobile: %v\n", err3)
			return
		}
		// Add repo URL to each milestone
		for i := range mobileMilestones {
			mobileMilestones[i].RepoURL = mobileRepoURL
		}
		
		desktopMilestones, err4 := getMilestones(desktopRepoURL)
		if err4 != nil {
			fmt.Printf("Error getting milestones from mattermost/mattermost-desktop: %v\n", err4)
			return
		}
		// Add repo URL to each milestone
		for i := range desktopMilestones {
			desktopMilestones[i].RepoURL = desktopRepoURL
		}

		repoName = "all repositories"
		
		// Create unified milestones by name
		unifiedMilestones := unifyMilestonesByName(mmMilestones, entMilestones, mobileMilestones, desktopMilestones)
		
		// Convert back to simple milestones for display and selection
		for _, um := range unifiedMilestones {
			// Use the first milestone as the representative for this name
			representative := um.Milestones[0]
			milestones = append(milestones, representative)
		}
		
		err = nil
	}

	if err != nil {
		fmt.Printf("Error getting milestones: %v\n", err)
		return
	}

	fmt.Printf("\nWorking with %s\n", repoName)

	// Display milestones for selection
	fmt.Println("Available milestones:")
	for i, milestone := range milestones {
		fmt.Printf("%d: %s\n", i+1, milestone.Title)
	}

	// Allow user to select a milestone
	reader = bufio.NewReader(os.Stdin)
	fmt.Print("\nSelect a milestone (number): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	index, err := strconv.Atoi(input)
	if err != nil || index < 1 || index > len(milestones) {
		fmt.Println("Invalid selection")
		return
	}

	selectedMilestone := milestones[index-1]
	fmt.Printf("\nSelected milestone: %s\n\n", selectedMilestone.Title)

	// Get PRs with "release-note" label for the selected milestone
	var prs []PullRequest

	if repoChoice == 5 {
		// For "all repositories", we need to find all instances of this milestone name in all repos
		// Get unified milestones again
		mmMilestones, _ := getMilestones(mattermostRepoURL)
		for i := range mmMilestones {
			mmMilestones[i].RepoURL = mattermostRepoURL
		}
		
		entMilestones, _ := getMilestones(enterpriseRepoURL)
		for i := range entMilestones {
			entMilestones[i].RepoURL = enterpriseRepoURL
		}
		
		mobileMilestones, _ := getMilestones(mobileRepoURL)
		for i := range mobileMilestones {
			mobileMilestones[i].RepoURL = mobileRepoURL
		}
		
		desktopMilestones, _ := getMilestones(desktopRepoURL)
		for i := range desktopMilestones {
			desktopMilestones[i].RepoURL = desktopRepoURL
		}
		
		unifiedMilestones := unifyMilestonesByName(mmMilestones, entMilestones, mobileMilestones, desktopMilestones)
		
		// Find the unified milestone that matches our selection
		var targetMilestones []Milestone
		for _, um := range unifiedMilestones {
			if um.Title == selectedMilestone.Title {
				targetMilestones = um.Milestones
				break
			}
		}
		
		// Fetch PRs for each matching milestone
		for _, milestone := range targetMilestones {
			milePRs, err := getPRsWithReleaseNotes(milestone.RepoURL, milestone.Number)
			if err != nil {
				repoName := "mattermost/mattermost"
				if milestone.RepoURL == enterpriseRepoURL {
					repoName = "mattermost/enterprise"
				} else if milestone.RepoURL == mobileRepoURL {
					repoName = "mattermost/mattermost-mobile"
				} else if milestone.RepoURL == desktopRepoURL {
					repoName = "mattermost/mattermost-desktop"
				}
				fmt.Printf("Error getting PRs from %s: %v\n", repoName, err)
			} else {
				prs = append(prs, milePRs...)
			}
		}
	} else {
		// For a single repository
		prs, err = getPRsWithReleaseNotes(repoURL, selectedMilestone.Number)
		if err != nil {
			fmt.Printf("Error getting PRs: %v\n", err)
			return
		}
	}

	// Print information for each PR and its release notes
	if len(prs) == 0 {
		fmt.Println("No PRs with 'release-note' label found in this milestone.")
		return
	}

	fmt.Printf("PRs with release notes in milestone %s:\n\n", selectedMilestone.Title)
	for _, pr := range prs {
		releaseNote := extractReleaseNote(pr.Body)
		fmt.Printf("PR #%d: %s\n", pr.Number, pr.Title)
		fmt.Printf("Release Note: %s\n\n", releaseNote)
	}
}

// Gets all open milestones from the specified repository
func getMilestones(repoURL string) ([]Milestone, error) {
	url := fmt.Sprintf("%s/milestones?state=open", repoURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read error response body for more details
		errorBody := make([]byte, 1024)
		n, _ := resp.Body.Read(errorBody)
		return nil, fmt.Errorf("API responded with code: %d for URL %s - Response: %s", 
			resp.StatusCode, url, string(errorBody[:n]))
	}

	var milestones []Milestone
	if err := json.NewDecoder(resp.Body).Decode(&milestones); err != nil {
		return nil, err
	}

	return milestones, nil
}

// Gets PRs with "release-note" label for a specific milestone
func getPRsWithReleaseNotes(repoURL string, milestoneID int) ([]PullRequest, error) {
	url := fmt.Sprintf("%s/issues?milestone=%d&state=all&labels=release-note", repoURL, milestoneID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read error response body for more details
		errorBody := make([]byte, 1024)
		n, _ := resp.Body.Read(errorBody)
		return nil, fmt.Errorf("API responded with code: %d for URL %s - Response: %s", 
			resp.StatusCode, url, string(errorBody[:n]))
	}

	var prs []PullRequest
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return nil, err
	}

	var pullRequests []PullRequest
	for _, pr := range prs {
		// Verify if it's a PR (not an issue) and has a milestone
		if strings.Contains(fmt.Sprintf("%s/pull/%d", repoURL, pr.Number), "pull") && pr.Milestone != nil {
			pullRequests = append(pullRequests, pr)
		}
	}

	return pullRequests, nil
}

// Extracts the release note section from the PR description
func extractReleaseNote(body string) string {
	if body == "" {
		return "No release note found"
	}

	// Try different release note formats

	// Format 1: ```release-note ... ```
	re1 := regexp.MustCompile("(?s)```release-note\n(.*?)\n```")
	matches1 := re1.FindStringSubmatch(body)
	if len(matches1) >= 2 {
		return strings.TrimSpace(matches1[1])
	}

	// Format 2: ```release-note ... ``` (with spaces)
	re2 := regexp.MustCompile("(?s)```\\s*release-note\\s*\n(.*?)\n\\s*```")
	matches2 := re2.FindStringSubmatch(body)
	if len(matches2) >= 2 {
		return strings.TrimSpace(matches2[1])
	}

	// Format 3: ### Release Note ... ###
	re3 := regexp.MustCompile("(?s)###\\s*Release Note\\s*\n(.*?)(\n###|\n$)")
	matches3 := re3.FindStringSubmatch(body)
	if len(matches3) >= 2 {
		return strings.TrimSpace(matches3[1])
	}

	// Format 4: release-note: ...
	re4 := regexp.MustCompile("(?s)release-note:\\s*(.*?)(\n\n|\n$)")
	matches4 := re4.FindStringSubmatch(body)
	if len(matches4) >= 2 {
		return strings.TrimSpace(matches4[1])
	}

	// Try to extract any paragraph with "release note" mentioned
	re5 := regexp.MustCompile("(?i)(?s)(?:release notes?|release changes?)[:\\s]+(.*?)(\n\n|\n$)")
	matches5 := re5.FindStringSubmatch(body)
	if len(matches5) >= 2 {
		return strings.TrimSpace(matches5[1])
	}

	return "No release note found in expected format"
}
