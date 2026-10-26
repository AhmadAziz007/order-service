package cmd

import (
	"ecommerce/order-service/internal/adapter/message"
	"fmt"

	"github.com/spf13/cobra"
)

var workerCmd = &cobra.Command{
	Use:   "worker-order",
	Short: "Menjalankan worker untuk consume RabbitMQ dan index ke Elasticsearch",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Worker untuk Order Indexing sedang berjalan...")
		message.StartOrderConsumer()
	},
}

// func init() {
// 	rootCmd.AddCommand(workerCmd)
// }
