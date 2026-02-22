services:
  wordpress:
    image: wordpress:6-apache
    restart: unless-stopped
    environment:
      WORDPRESS_DB_HOST: mysql
      WORDPRESS_DB_USER: wordpress
      WORDPRESS_DB_PASSWORD: {{.MysqlPassword}}
      WORDPRESS_DB_NAME: wordpress
      WORDPRESS_CONFIG_EXTRA: |
        define('WP_HOME', 'https://{{.ProjectName}}.test');
        define('WP_SITEURL', 'https://{{.ProjectName}}.test');
    volumes:
      - ./wp-content:/var/www/html/wp-content
    depends_on:
      mysql:
        condition: service_healthy
    networks:
      - default
      - traefik
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.{{.ProjectName}}-wordpress.rule=Host(`{{.ProjectName}}.test`) || Host(`www.{{.ProjectName}}.test`)"
      - "traefik.http.routers.{{.ProjectName}}-wordpress.entrypoints=websecure"
      - "traefik.http.routers.{{.ProjectName}}-wordpress.tls=true"
      - "traefik.http.services.{{.ProjectName}}-wordpress.loadbalancer.server.port=80"
      - "traefik.docker.network=traefik"

  mysql:
    image: mysql:8.0
    restart: unless-stopped
    environment:
      MYSQL_ROOT_PASSWORD: {{.MysqlPassword}}
      MYSQL_DATABASE: wordpress
      MYSQL_USER: wordpress
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
  default:
    name: {{.ProjectName}}

volumes:
  mysql_data:
