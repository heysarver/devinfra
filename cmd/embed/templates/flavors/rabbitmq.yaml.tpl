services:
  rabbitmq:
    image: rabbitmq:4-management-alpine
    restart: unless-stopped
    environment:
      RABBITMQ_DEFAULT_USER: {{.ProjectName}}
      RABBITMQ_DEFAULT_PASS: {{.RabbitmqPassword}}
    volumes:
      - rabbitmq_data:/var/lib/rabbitmq
    ports:
      - "5672"
      - "15672"
    networks:
      - default

volumes:
  rabbitmq_data:
