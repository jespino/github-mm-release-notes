package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// GitHub API structures
type Milestone struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description"`
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
	authToken         = "" // Add your GitHub token here
)

func main() {
	// Select repository
	fmt.Println("Select a repository:")
	fmt.Println("1: mattermost/mattermost")
	fmt.Println("2: mattermost/enterprise")
	fmt.Println("3: Both")
	
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nSelect an option (1-3): ")
	repoInput, _ := reader.ReadString('\n')
	repoInput = strings.TrimSpace(repoInput)
	
	repoChoice, err := strconv.Atoi(repoInput)
	if err != nil || repoChoice < 1 || repoChoice > 3 {
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
		// Get milestones from both repositories and combine them
		mmMilestones, err1 := getMilestones(mattermostRepoURL)
		if err1 != nil {
			fmt.Printf("Error getting milestones from mattermost/mattermost: %v\n", err1)
			return
		}
		
		entMilestones, err2 := getMilestones(enterpriseRepoURL)
		if err2 != nil {
			fmt.Printf("Error getting milestones from mattermost/enterprise: %v\n", err2)
			return
		}
		
		repoName = "both repositories"
		milestones = append(mmMilestones, entMilestones...)
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
	
	if repoChoice == 3 {
		// For "both repositories", search in both
		mmPRs, err1 := getPRsWithReleaseNotes(mattermostRepoURL, selectedMilestone.Number)
		if err1 != nil {
			fmt.Printf("Error getting PRs from mattermost/mattermost: %v\n", err1)
		} else {
			prs = append(prs, mmPRs...)
		}
		
		entPRs, err2 := getPRsWithReleaseNotes(enterpriseRepoURL, selectedMilestone.Number)
		if err2 != nil {
			fmt.Printf("Error getting PRs from mattermost/enterprise: %v\n", err2)
		} else {
			prs = append(prs, entPRs...)
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
		req.Header.Set("Authorization", "token "+authToken)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API responded with code: %d", resp.StatusCode)
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
		req.Header.Set("Authorization", "token "+authToken)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API responded with code: %d", resp.StatusCode)
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
