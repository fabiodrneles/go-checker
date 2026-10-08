package cmd

import (
	"fmt"
	"os"

	"go-checker/checker"

	"github.com/spf13/cobra"
)

var configFile string

// checkCmd representa o comando check
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Verifica a saúde dos serviços listados no arquivo de configuração",
	Run: func(cmd *cobra.Command, args []string) {
		// Verifica se o arquivo de configuração existe
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			fmt.Printf("Erro: Arquivo de configuração '%s' não encontrado.\n", configFile)
			fmt.Println("Por favor, crie um ou especifique o caminho com a flag -f.")
			os.Exit(1)
		}

		// Lê e interpreta o arquivo de configuração
		sites, err := checker.ParseConfig(configFile)
		if err != nil {
			fmt.Printf("Erro ao ler o arquivo de configuração: %v\n", err)
			os.Exit(1)
		}

		// Executa as verificações
		checker.RunChecks(sites)
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
	// Define a flag para o arquivo de configuração
	checkCmd.Flags().StringVarP(&configFile, "file", "f", "config.yaml", "Caminho para o arquivo de configuração (ex: config.yaml)")
}
