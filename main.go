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
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	MilestoneID int    `json:"milestone"`
	Labels      []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// URL base para la API de GitHub - ajusta el owner/repo según tu necesidad
const (
	apiBaseURL = "https://api.github.com/repos/OWNER/REPO"
	authToken  = "" // Añade tu token de GitHub aquí
)

func main() {
	// Obtener todos los milestones
	milestones, err := getMilestones()
	if err != nil {
		fmt.Printf("Error al obtener milestones: %v\n", err)
		return
	}

	// Mostrar milestones para selección
	fmt.Println("Milestones disponibles:")
	for i, milestone := range milestones {
		fmt.Printf("%d: %s\n", i+1, milestone.Title)
	}

	// Permitir al usuario seleccionar un milestone
	reader := bufio.NewReader(os.Stdin)
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
	prs, err := getPRsWithReleaseNotes(selectedMilestone.Number)
	if err != nil {
		fmt.Printf("Error al obtener PRs: %v\n", err)
		return
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

// Obtiene todos los milestones abiertos del repositorio
func getMilestones() ([]Milestone, error) {
	url := fmt.Sprintf("%s/milestones?state=open", apiBaseURL)
	
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
func getPRsWithReleaseNotes(milestoneID int) ([]PullRequest, error) {
	url := fmt.Sprintf("%s/issues?milestone=%d&state=all&labels=release-note", apiBaseURL, milestoneID)
	
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
		// Verificar si es un PR y no un issue
		if strings.Contains(fmt.Sprintf("%s/pull/%d", apiBaseURL, pr.Number), "pull") {
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
