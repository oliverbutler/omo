# Run Docker Compose for development environment
local:
	docker-compose -f docker-compose.yml up --build


# Stop Docker Compose services
down:
	docker-compose down

# View logs for Docker Compose services
logs:
	docker-compose logs -f

# Clean up Docker Compose resources
clean:
	docker-compose down -v --remove-orphans
