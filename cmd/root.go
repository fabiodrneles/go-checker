package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd representa o comando base quando chamado sem subcomandos
var rootCmd = &cobra.Command{
	Use:   "go-checker",
	Short: "Go-Checker é uma CLI para monitorar a saúde de serviços web.",
	Long: `Uma ferramenta de linha de comando rápida e concorrente escrita em Go
para verificar o status de múltiplos websites e APIs definidos em um arquivo de configuração.`,
}

// Execute adiciona todos os comandos filhos ao comando raiz e define as flags apropriadamente.
// Isso é chamado por main.main(). Só precisa acontecer uma vez para o rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Ocorreu um erro: '%s'", err)
		os.Exit(1)
	}
}