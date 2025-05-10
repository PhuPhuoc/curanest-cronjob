.DEFAULT_GOAL := help

# Variables
APP_NAME := curanest-cronjob
APP_DEBUG := $(APP_NAME)-debug
MAIN_FILE := main.go
GCFLAGS := all=-N -l
SERVICE_NAME := cron_job 
DOCKER_OWNER := pardes29
IMAGE_VER := v1

.PHONY: help build run build-debug debug up down tag push clean

# Show all available commands
help: ## Show all available commands
	@echo Available commands:
	@findstr /R /C:"^[a-zA-Z_-]*:.*##" $(MAKEFILE_LIST) | findstr /V findstr | sort

build: ## Build the application
	go build -o $(APP_NAME).exe $(MAIN_FILE)

run: build ## Run the application
	./$(APP_NAME).exe

build-debug: ## Build the application in debug mode
	go build -gcflags="$(GCFLAGS)" -o $(APP_DEBUG).exe $(MAIN_FILE)

debug: build-debug ## Run the application in debug mode
	@if not exist dlv.exe go install github.com/go-delve/delve/cmd/dlv@latest
	dlv exec ./$(APP_DEBUG).exe

up: ## Start Docker containers
	docker compose up -d

down: ## Stop Docker containers
	docker compose down

tag: ## Tag the Docker image
	docker tag $(SERVICE_NAME):$(IMAGE_VER) $(DOCKER_OWNER)/$(SERVICE_NAME):$(IMAGE_VER)

push: ## Push the Docker image to a registry
	docker push $(DOCKER_OWNER)/$(SERVICE_NAME):$(IMAGE_VER)

clean: ## Clean build files
	@if exist $(APP_NAME).exe del $(APP_NAME).exe
	@if exist $(APP_DEBUG).exe del $(APP_DEBUG).exe
