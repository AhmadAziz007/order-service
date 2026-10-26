# order-service
Service order untuk micro-ecommerce.

## Menjalankan
1. cp .env.example .env
2. go run main.go

## Menjalankan worker dengan rabbitmq
1. go run main.go worker-order
2. go run main.go worker-update-status
3. go run main.go worker-delete-order
4. go run main.go worker-delete-order
