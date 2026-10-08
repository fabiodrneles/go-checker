package cmd

import (
	"fmt"
	"os"
	"time"

	"go-checker/checker"

	"github.com/spf13/cobra"
)

var configFile string
var timeoutSeconds int

// checkCmd representa o comando check
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Verifica a saúde dos serviços listados no arquivo de configuração",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validação do timeout
		if timeoutSeconds <= 0 {
			return fmt.Errorf("o valor do timeout deve ser um número inteiro positivo")
		}

		// Verifica se o arquivo de configuração existe
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			return fmt.Errorf("arquivo de configuração '%s' não encontrado. Por favor, crie um ou especifique o caminho com a flag -f", configFile)
		}

		// Lê e interpreta o arquivo de configuração
		sites, err := checker.ParseConfig(configFile)
		if err != nil {
			return fmt.Errorf("erro ao ler o arquivo de configuração: %w", err)
		}

		// Executa as verificações com timeout
		checker.RunChecks(sites, time.Duration(timeoutSeconds)*time.Second)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
	// Define a flag para o arquivo de configuração
	checkCmd.Flags().StringVarP(&configFile, "file", "f", "config.yaml", "Caminho para o arquivo de configuração (ex: config.yaml)")
	// Define a flag para timeout com valor padrão de 120 segundos
	checkCmd.Flags().IntVar(&timeoutSeconds, "timeout", 120, "Tempo máximo de execução em segundos (padrão: 120)")
}
