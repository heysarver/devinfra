services:
  minio:
    image: minio/minio:latest
    restart: unless-stopped
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: {{.ProjectName}}
      MINIO_ROOT_PASSWORD: {{.MinioPassword}}
    volumes:
      - minio_data:/data
    ports:
      - "9000"
      - "9001"
    networks:
      - default

volumes:
  minio_data:
