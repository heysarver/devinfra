services:
  ghost:
    image: ghost:5-alpine
    restart: unless-stopped
    environment:
      url: https://{{.ProjectName}}.{{.TLD}}
      database__client: mysql
      database__connection__host: mysql
      database__connection__user: ghost
      database__connection__password: {{.MysqlPassword}}
      database__connection__database: ghost
    volumes:
      - ./content:/var/lib/ghost/content
    depends_on:
      mysql:
        condition: service_healthy
    networks:
      - default
      - traefik
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.{{.ProjectName}}-ghost.rule=Host(`{{.ProjectName}}.{{.TLD}}`) || Host(`www.{{.ProjectName}}.{{.TLD}}`)"
      - "traefik.http.routers.{{.ProjectName}}-ghost.entrypoints=websecure"
      - "traefik.http.routers.{{.ProjectName}}-ghost.tls=true"
      - "traefik.http.services.{{.ProjectName}}-ghost.loadbalancer.server.port=2368"
      - "traefik.docker.network=traefik"

  mysql:
    image: mysql:8.0
    restart: unless-stopped
    environment:
      MYSQL_ROOT_PASSWORD: {{.MysqlPassword}}
      MYSQL_DATABASE: ghost
      MYSQL_USER: ghost
      MYSQL_PASSWORD: {{.MysqlPassword}}
    volumes:
      - mysql_data:/var/lib/mysql
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
      interval: 5s
      timeout: 5s
      retries: 10
    networks:
      - default

networks:
  traefik:
    external: true

volumes:
  mysql_data:
