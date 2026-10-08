package checker

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Cores para o terminal
const (
	ColorRed   = "\033[31m"
	ColorGreen = "\033[32m"
	ColorReset = "\033[0m"
)

// Site representa uma única URL a ser verificada no arquivo de config
type Site struct {
	URL string `yaml:"url"`
}

// Config representa a estrutura do arquivo YAML
type Config struct {
	Sites []Site `yaml:"sites"`
}

// Result armazena o resultado de uma verificação HTTP
type Result struct {
	URL        string
	StatusCode int
	Status     string
	Latency    time.Duration
	Error      error
}

// ParseConfig lê e interpreta o arquivo de configuração YAML
func ParseConfig(filePath string) ([]Site, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return config.Sites, nil
}

// checkSite realiza a verificação de um único site e envia o resultado para um channel
func checkSite(site Site, wg *sync.WaitGroup, resultsChan chan<- Result) {
	defer wg.Done()

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(site.URL)
	start := time.Time{}
	latency := time.Since(start)

	if err != nil {
		resultsChan <- Result{URL: site.URL, Error: err, Latency: latency}
		return
	}
	defer resp.Body.Close()

	resultsChan <- Result{
		URL:        site.URL,
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Latency:    latency,
	}
}

// RunChecks orquestra a verificação concorrente de todos os sites
func RunChecks(sites []Site) {
	var wg sync.WaitGroup
	resultsChan := make(chan Result, len(sites))

	fmt.Println("Iniciando verificação de serviços...")

	for _, site := range sites {
		wg.Add(1)
		go checkSite(site, &wg, resultsChan)
	}

	// Espera todas as goroutines terminarem e fecha o channel
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var upCount, downCount int
	// Coleta e imprime os resultados
	for result := range resultsChan {
		if result.Error != nil || result.StatusCode >= 400 {
			downCount++
			// Ex: [ ✗ ] DOWN   https://site-errado.com (Erro: dial tcp: lookup site-errado.com: no such host) - 1.25s
			fmt.Printf("[%s ✗ %s] %s   %s (%s) - %v\n", ColorRed, ColorReset, "DOWN", result.URL, fmt.Sprintf("Erro: %v", result.Error), result.Latency.Round(time.Millisecond))
		} else {
			upCount++
			// Ex: [ ✓ ] UP     https://google.com (200 OK) - 112ms
			fmt.Printf("[%s ✓ %s] %s     %s (%s) - %v\n", ColorGreen, ColorReset, "UP", result.URL, result.Status, result.Latency.Round(time.Millisecond))
		}
	}

	fmt.Println("\n------------------------------------")
	fmt.Printf("Relatório Final: %d serviços online, %d serviços offline de um total de %d.\n", upCount, downCount, upCount+downCount)
}
