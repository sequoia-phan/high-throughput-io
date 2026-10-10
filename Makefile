.PHONY: build-agent clean

# Configuration
BINARY_AGENT=vm-agent
AGENT_PATH=cmd/agent/main.go

build-agent:
	@echo "🔨 Building VM Agent (Static Linux Binary)..."
	CGO_ENABLED=0 GOOS=linux go build -o $(BINARY_AGENT) $(AGENT_PATH)
	@echo "✅ Agent built successfully: $(BINARY_AGENT)"

clean:
	@echo "🧹 Cleaning agent binary..."
	rm -f $(BINARY_AGENT)
