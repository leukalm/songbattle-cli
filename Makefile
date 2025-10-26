# Variables
BINARY_NAME=song-battle
BUILD_DIR=bin
MAIN_PATH=./cmd/song-battle
GO_FILES=$(shell find . -name "*.go" -type f)

# Couleurs pour les messages
GREEN=\033[0;32m
YELLOW=\033[0;33m
RED=\033[0;31m
NC=\033[0m # No Color

.PHONY: all build run test clean help install deps lint fmt vet

all: deps fmt vet test build

# Construction de l'application
build: $(BUILD_DIR)/$(BINARY_NAME)

$(BUILD_DIR)/$(BINARY_NAME): $(GO_FILES)
	@echo "$(GREEN)🔨 Construction de l'application...$(NC)"
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "$(GREEN)✅ Application construite: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

# Lancer l'application en mode développement
run: build
	@echo "$(GREEN)🚀 Lancement de Song Battle...$(NC)"
	./$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

# Lancer avec import de données
run-import: build
	@echo "$(GREEN)📥 Lancement en mode import...$(NC)"
	./$(BUILD_DIR)/$(BINARY_NAME) -import $(ARGS)

# Tests
test:
	@echo "$(GREEN)🧪 Exécution des tests...$(NC)"
	go test -v ./...

# Tests avec couverture
test-coverage:
	@echo "$(GREEN)📊 Tests avec couverture...$(NC)"
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✅ Rapport de couverture généré: coverage.html$(NC)"

# Installation des dépendances
deps:
	@echo "$(GREEN)📦 Installation des dépendances...$(NC)"
	go mod tidy
	go mod download

# Nettoyage
clean:
	@echo "$(YELLOW)🧹 Nettoyage...$(NC)"
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	go clean
	@echo "$(GREEN)✅ Nettoyage terminé$(NC)"

# Installation globale
install: build
	@echo "$(GREEN)📥 Installation globale...$(NC)"
	go install $(MAIN_PATH)
	@echo "$(GREEN)✅ Song Battle installé globalement$(NC)"

# Formatage du code
fmt:
	@echo "$(GREEN)🎨 Formatage du code...$(NC)"
	go fmt ./...

# Vérification du code
vet:
	@echo "$(GREEN)🔍 Vérification du code...$(NC)"
	go vet ./...

# Lint avec golangci-lint (si installé)
lint:
	@echo "$(GREEN)🔎 Analyse du code...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "$(YELLOW)⚠️  golangci-lint non installé, utilisation de go vet...$(NC)"; \
		go vet ./...; \
	fi

# Génération de mocks (si nécessaire)
generate:
	@echo "$(GREEN)⚙️  Génération du code...$(NC)"
	go generate ./...

# Construction pour différentes plateformes
build-all: clean
	@echo "$(GREEN)🌍 Construction multi-plateformes...$(NC)"
	@mkdir -p $(BUILD_DIR)
	
	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-macos-amd64 $(MAIN_PATH)
	
	# macOS ARM64 (Apple Silicon)
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-macos-arm64 $(MAIN_PATH)
	
	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	
	@echo "$(GREEN)✅ Construction multi-plateformes terminée$(NC)"

# Création d'un release
release: test build-all
	@echo "$(GREEN)📦 Création du release...$(NC)"
	@mkdir -p release
	
	# Créer les archives
	cd $(BUILD_DIR) && tar -czf ../release/$(BINARY_NAME)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	cd $(BUILD_DIR) && tar -czf ../release/$(BINARY_NAME)-macos-amd64.tar.gz $(BINARY_NAME)-macos-amd64
	cd $(BUILD_DIR) && tar -czf ../release/$(BINARY_NAME)-macos-arm64.tar.gz $(BINARY_NAME)-macos-arm64
	cd $(BUILD_DIR) && zip -q ../release/$(BINARY_NAME)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe
	
	@echo "$(GREEN)✅ Release créé dans le dossier release/$(NC)"

# Commandes de développement rapide
dev: deps fmt vet run

# Setup initial pour le développement
setup:
	@echo "$(GREEN)🎵 Configuration initiale de Song Battle...$(NC)"
	@echo "$(YELLOW)1. Assurez-vous d'avoir créé une app Spotify sur https://developer.spotify.com/dashboard$(NC)"
	@echo "$(YELLOW)2. Configurez l'URI de redirection: http://localhost:8080/callback$(NC)"
	@echo "$(YELLOW)3. Notez votre Client ID$(NC)"
	@echo ""
	@echo "$(GREEN)Ensuite, lancez:$(NC)"
	@echo "  make run-import ARGS='-client-id=VOTRE_CLIENT_ID'"
	@echo "  make run ARGS='-client-id=VOTRE_CLIENT_ID'"

# Aide
help:
	@echo "$(GREEN)🎵 Song Battle CLI - Commandes Make$(NC)"
	@echo ""
	@echo "$(YELLOW)Construction:$(NC)"
	@echo "  build          Construire l'application"
	@echo "  build-all      Construire pour toutes les plateformes"
	@echo "  install        Installer globalement"
	@echo ""
	@echo "$(YELLOW)Développement:$(NC)"
	@echo "  run            Lancer l'application"
	@echo "  run-import     Lancer en mode import"
	@echo "  dev            Setup rapide développement"
	@echo "  setup          Guide de configuration initiale"
	@echo ""
	@echo "$(YELLOW)Qualité du code:$(NC)"
	@echo "  test           Exécuter les tests"
	@echo "  test-coverage  Tests avec couverture"
	@echo "  fmt            Formater le code"
	@echo "  vet            Vérifier le code"
	@echo "  lint           Analyser le code"
	@echo ""
	@echo "$(YELLOW)Maintenance:$(NC)"
	@echo "  deps           Installer les dépendances"
	@echo "  clean          Nettoyer les fichiers générés"
	@echo "  release        Créer un release complet"
	@echo ""
	@echo "$(YELLOW)Exemples:$(NC)"
	@echo "  make run ARGS='-client-id=abc123'"
	@echo "  make run-import ARGS='-client-id=abc123 -db-path=./test.db'"

# Vérification des variables d'environnement pour les tests
check-env:
	@if [ -z "$(SPOTIFY_CLIENT_ID)" ]; then \
		echo "$(RED)❌ Variable SPOTIFY_CLIENT_ID non définie$(NC)"; \
		echo "$(YELLOW)Définissez-la avec: export SPOTIFY_CLIENT_ID=votre_client_id$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)✅ Variables d'environnement OK$(NC)"

# Test d'intégration avec authentification
test-integration: check-env build
	@echo "$(GREEN)🔗 Test d'intégration...$(NC)"
	./$(BUILD_DIR)/$(BINARY_NAME) -import -db-path=./test.db
	@echo "$(GREEN)✅ Test d'intégration terminé$(NC)"
	@rm -f ./test.db