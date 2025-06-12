.PHONY: run check_dockerfile check_k8s_manifest

run:
	@echo "Starting the application..."
	@go run cmd/server/main.go

check_dockerfile:
	@echo "Checking Dockerfile..."
	@curl -X POST http://localhost:8080/scan \
	  -H "Content-Type: application/json" \
	  -d "{\"target_type\": \"file\", \"target\": \"/home/one2n/Desktop/NACK/weekly-security-ai/vulnerable-manifests/Dockerfile\", \"summarize\": true}"

	  
check_k8s_manifest:
	@echo "Checking Kubernetes manifest..."
	@curl -X POST http://localhost:8080/scan \
	  -H "Content-Type: application/json" \
	  -d "{\"target_type\": \"file\", \"target\": \"/home/one2n/Desktop/NACK/weekly-security-ai/vulnerable-manifests/k8s-manifets.yml\", \"summarize\": true}"

 