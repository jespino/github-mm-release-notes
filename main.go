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

// Estructuras para la API de GitHub
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

// URLs para los repositorios de Mattermost
const (
	mattermostRepoURL = "https://api.github.com/repos/mattermost/mattermost"
	enterpriseRepoURL = "https://api.github.com/repos/mattermost/enterprise"
	authToken         = "" // Añade tu token de GitHub aquí
)

func main() {
	// Seleccionar repositorio
	fmt.Println("Selecciona un repositorio:")
	fmt.Println("1: mattermost/mattermost")
	fmt.Println("2: mattermost/enterprise")
	fmt.Println("3: Ambos")
	
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nSelecciona una opción (1-3): ")
	repoInput, _ := reader.ReadString('\n')
	repoInput = strings.TrimSpace(repoInput)
	
	repoChoice, err := strconv.Atoi(repoInput)
	if err != nil || repoChoice < 1 || repoChoice > 3 {
		fmt.Println("Selección inválida")
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
		// Obtener milestones de ambos repositorios y combinarlos
		mmMilestones, err1 := getMilestones(mattermostRepoURL)
		if err1 != nil {
			fmt.Printf("Error al obtener milestones de mattermost/mattermost: %v\n", err1)
			return
		}
		
		entMilestones, err2 := getMilestones(enterpriseRepoURL)
		if err2 != nil {
			fmt.Printf("Error al obtener milestones de mattermost/enterprise: %v\n", err2)
			return
		}
		
		repoName = "ambos repositorios"
		milestones = append(mmMilestones, entMilestones...)
		err = nil
	}
	
	if err != nil {
		fmt.Printf("Error al obtener milestones: %v\n", err)
		return
	}
	
	fmt.Printf("\nTrabajando con %s\n", repoName)

	// Mostrar milestones para selección
	fmt.Println("Milestones disponibles:")
	for i, milestone := range milestones {
		fmt.Printf("%d: %s\n", i+1, milestone.Title)
	}

	// Permitir al usuario seleccionar un milestone
	reader = bufio.NewReader(os.Stdin)
	fmt.Print("\nSelecciona un milestone (número): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	
	index, err := strconv.Atoi(input)
	if err != nil || index < 1 || index > len(milestones) {
		fmt.Println("Selección inválida")
		return
	}

	selectedMilestone := milestones[index-1]
	fmt.Printf("\nMilestone seleccionado: %s\n\n", selectedMilestone.Title)

	// Obtener PRs con etiqueta "release-note" para el milestone seleccionado
	var prs []PullRequest
	
	if repoChoice == 3 {
		// Para "ambos repositorios", buscar en los dos
		mmPRs, err1 := getPRsWithReleaseNotes(mattermostRepoURL, selectedMilestone.Number)
		if err1 != nil {
			fmt.Printf("Error al obtener PRs de mattermost/mattermost: %v\n", err1)
		} else {
			prs = append(prs, mmPRs...)
		}
		
		entPRs, err2 := getPRsWithReleaseNotes(enterpriseRepoURL, selectedMilestone.Number)
		if err2 != nil {
			fmt.Printf("Error al obtener PRs de mattermost/enterprise: %v\n", err2)
		} else {
			prs = append(prs, entPRs...)
		}
	} else {
		// Para un solo repositorio
		prs, err = getPRsWithReleaseNotes(repoURL, selectedMilestone.Number)
		if err != nil {
			fmt.Printf("Error al obtener PRs: %v\n", err)
			return
		}
	}

	// Imprimir información de cada PR y sus notas de release
	if len(prs) == 0 {
		fmt.Println("No se encontraron PRs con etiqueta 'release-note' en este milestone.")
		return
	}

	fmt.Printf("PRs con notas de release en milestone %s:\n\n", selectedMilestone.Title)
	for _, pr := range prs {
		releaseNote := extractReleaseNote(pr.Body)
		fmt.Printf("PR #%d: %s\n", pr.Number, pr.Title)
		fmt.Printf("Release Note: %s\n\n", releaseNote)
	}
}

// Obtiene todos los milestones abiertos del repositorio especificado
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
		return nil, fmt.Errorf("API respondió con código: %d", resp.StatusCode)
	}
	
	var milestones []Milestone
	if err := json.NewDecoder(resp.Body).Decode(&milestones); err != nil {
		return nil, err
	}
	
	return milestones, nil
}

// Obtiene PRs con etiqueta "release-note" para un milestone específico
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
		return nil, fmt.Errorf("API respondió con código: %d", resp.StatusCode)
	}
	
	var prs []PullRequest
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return nil, err
	}
	
	var pullRequests []PullRequest
	for _, pr := range prs {
		// Verificar si es un PR y no un issue y tiene un milestone
		if strings.Contains(fmt.Sprintf("%s/pull/%d", repoURL, pr.Number), "pull") && pr.Milestone != nil {
			pullRequests = append(pullRequests, pr)
		}
	}
	
	return pullRequests, nil
}

// Extrae la sección de release note de la descripción del PR
func extractReleaseNote(body string) string {
	if body == "" {
		return "No se encontró nota de release"
	}
	
	// Buscar el bloque ```release-note ... ```
	re := regexp.MustCompile("(?s)```release-note\n(.*?)\n```")
	matches := re.FindStringSubmatch(body)
	
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}
	
	return "No se encontró nota de release en formato esperado"
}
