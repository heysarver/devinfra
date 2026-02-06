services:
  valkey:
    image: valkey/valkey:9-alpine
    restart: unless-stopped
    command: valkey-server --appendonly yes
    volumes:
      - valkey_data:/data
    ports:
      - "6379"
    networks:
      - default

  valkey-sentinel:
    image: valkey/valkey:9-alpine
    restart: unless-stopped
    depends_on:
      - valkey
    entrypoint: >
      sh -c '
        cat > /tmp/sentinel.conf <<CONF
        port 26379
        sentinel resolve-hostnames yes
        sentinel announce-hostnames yes
        sentinel monitor {{.ProjectName}} valkey 6379 1
        sentinel down-after-milliseconds {{.ProjectName}} 5000
        sentinel failover-timeout {{.ProjectName}} 10000
        sentinel parallel-syncs {{.ProjectName}} 1
      CONF
        valkey-sentinel /tmp/sentinel.conf
      '
    ports:
      - "26379"
    networks:
      - default

volumes:
  valkey_data:
