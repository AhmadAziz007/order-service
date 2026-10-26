package cmd

import (
	"ecommerce/order-service/internal/adapter/message"
	"fmt"

	"github.com/spf13/cobra"
)

var workerUpdatePaymentOrderCmd = &cobra.Command{
	Use:   "worker-update-payment-order",
	Short: "Menjalankan worker untuk consume RabbitMQ dan index ke Elasticsearch",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Worker untuk Order Indexing sedang berjalan...")
		message.ConsumePaymentSuccess()
	},
}

// func init() {
// 	rootCmd.AddCommand(workerUpdatePaymentOrderCmd)
// }
