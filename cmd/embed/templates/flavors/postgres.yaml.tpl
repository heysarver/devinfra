services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: {{.ProjectName}}
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: {{.PostgresPassword}}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432"
    networks:
      - default

volumes:
  postgres_data:
